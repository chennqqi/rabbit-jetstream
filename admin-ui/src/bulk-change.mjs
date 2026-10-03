import {parseJSON,stringifyJSON} from "./api.mjs";
import {capabilityHeaders,readMutationCapabilities} from "./mutation-capabilities.mjs";
import {previewOperations} from "./preview-operations.mjs";
import {declarationReview} from "./declaration-review.mjs";

const totalLimit=4*1024*1024,fileLimit=2*1024*1024,validName=value=>typeof value==="string"&&/^[A-Za-z0-9_-]{1,256}$/.test(value),validETag=value=>typeof value==="string"&&/^"[1-9][0-9]*"$/.test(value);
const statuses=new Set(["ready","blocked","invalid","conflict","missing","unavailable"]),codes=new Set(["invalid_declaration","invalid_etag","duplicate_queue","invalid_preview","conflict","not_found","dlq_dependency_cycle","jetstream_unavailable","deadline_exceeded"]);

export function validateChangePackage(value){
  if(!value||typeof value!=="object"||Array.isArray(value)||Object.keys(value).sort().join(",")!=="document,etag,schema"||value.schema!=="rjs.queue-change.v1"||!validETag(value.etag)||
    value.document?.apiVersion!=="rabbit-jetstream.io/v1alpha1"||value.document?.kind!=="Queue"||!validName(value.document?.metadata?.name)||!value.document.spec||typeof value.document.spec!=="object"||Array.isArray(value.document.spec)||new TextEncoder().encode(stringifyJSON(value.document)).length>1024*1024)throw Error("invalid");
  return parseJSON(stringifyJSON(value));
}

export function validateChangePlan(body,packages){
  if(body?.scope!=="bulk-change-preview-not-apply-authorization"||typeof body.ready!=="boolean"||!Array.isArray(body.items)||body.items.length!==packages.length)throw Error("invalid");
  const items=body.items.map((item,index)=>{
    const expected=packages[index],name=expected.document.metadata.name;
    if(!item||item.index!==index||item.queue!==name||!statuses.has(item.status)||item.code!==undefined&&!codes.has(item.code))throw Error("invalid");
    if(["ready","blocked"].includes(item.status)){
      if(item.etag!==expected.etag||item.code!==undefined||!item.preview||item.preview.create_only!==false||item.preview.base_revision!==expected.etag||item.preview.plan?.queue!==name||item.preview.result?.queue!==name||typeof item.preview.result.blocked!=="boolean"||
        item.preview.result.blocked!==(item.status==="blocked")||(item.status==="blocked"?item.preview.result.status!=="blocked":!["ready","noop"].includes(item.preview.result.status))||previewOperations(item.preview.result)===null||declarationReview({name,create:false,etag:expected.etag,preview:item.preview}).status==="invalid")throw Error("invalid");
    }else if(item.preview!==undefined||item.status!=="invalid"&&item.etag!==expected.etag)throw Error("invalid");
    return {index,filename:expected.filename,document:expected.document,etag:expected.etag,status:item.status,...(item.code?{code:item.code}:{}),...(item.preview?{preview:item.preview}:{})};
  });
  if(body.ready!==items.every(item=>item.status==="ready"))throw Error("invalid");
  return {ready:body.ready,items};
}

export function createBulkChange(api){
  let readGeneration=0,requestGeneration=0,controller,state={phase:"idle",packages:[],result:null,failure:null};const listeners=new Set(),emit=next=>{state=next;for(const fn of listeners)fn();};
  const cancel=()=>{requestGeneration++;controller?.abort();controller=null;};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},clear(){readGeneration++;cancel();emit({phase:"idle",packages:[],result:null,failure:null});},
    async read(input){const generation=++readGeneration;cancel();const files=Array.from(input??[]);emit({phase:"reading",packages:files.map(file=>({filename:file.name,status:"pending"})),result:null,failure:null});
      if(!files.length||files.length>100||files.some(file=>!Number.isSafeInteger(file.size)||file.size<1||file.size>fileLimit)||files.reduce((sum,file)=>sum+file.size,0)>totalLimit){emit({...state,phase:"error",failure:"bounds"});return;}
      const packages=[];for(const file of files){try{const bytes=await file.arrayBuffer();if(generation!==readGeneration)return;if(!(bytes instanceof ArrayBuffer)||bytes.byteLength!==file.size)throw Error();packages.push({...validateChangePackage(parseJSON(new TextDecoder("utf-8",{fatal:true}).decode(bytes))),filename:file.name,status:"loaded"});}catch{if(generation!==readGeneration)return;packages.push({filename:file.name,status:"invalid"});}}
      emit({phase:"files",packages,result:null,failure:packages.some(item=>item.status!=="loaded")?"files":null});
    },
    async plan(providedBinding){if(!state.packages.length||state.packages.some(item=>item.status!=="loaded"))return;cancel();const generation=requestGeneration,packages=state.packages;controller=new AbortController();emit({...state,phase:"planning",result:null,failure:null});
      try{const binding=providedBinding??await readMutationCapabilities(api,["queue-preview","conditional-queue-writes"]);if(generation!==requestGeneration)return;const body={items:packages.map(item=>({document:item.document,etag:item.etag}))};if(new TextEncoder().encode(stringifyJSON(body)).length>totalLimit)throw Error("bounds");const response=await api.request("/api/v1/queues/change-plan",{method:"POST",body,headers:capabilityHeaders(binding),signal:controller.signal});if(generation!==requestGeneration)return;emit({...state,phase:"ready",result:validateChangePlan(response.body,packages),binding,failure:null});}
      catch(error){if(generation!==requestGeneration)return;emit({...state,phase:"files",result:null,failure:error.status===401||error.status===403?"denied":error.status===412?"changed":error.message==="bounds"?"bounds":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable"});}
    }
  };
}
