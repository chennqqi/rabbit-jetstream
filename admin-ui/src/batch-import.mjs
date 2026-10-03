import {parseJSON,stringifyJSON} from "./api.mjs";
import {capabilityHeaders} from "./mutation-capabilities.mjs";

const limit=4*1024*1024,validName=name=>typeof name==="string"&&/^[A-Za-z0-9_-]{1,256}$/.test(name);
const codes=["invalid_declaration","duplicate_queue","external_dependency_unverified","dependency_cycle","blocked_dependency"];
export function validateBatchResult(body,files){
  const plan=body?.plan;
  if(body?.scope!=="declaration-only-not-apply-authorization"||!Array.isArray(plan?.items)||plan.items.length!==files.length||!Array.isArray(plan.order)||typeof plan.ready!=="boolean"||!Array.isArray(body.external_declarations))throw Error("invalid");
  const expectedExternal=new Set();
  const items=plan.items.map((item,index)=>{
    const name=files[index].document?.metadata?.name,expected=validName(name)?name:"";
    if(!item||item.index!==index||item.queue!==expected||!Array.isArray(item.dependencies)||!item.dependencies.every(validName)||!Array.isArray(item.problems)||item.revision!==undefined&&typeof item.revision!=="string")throw Error("invalid");
    const problems=item.problems.map(problem=>{
      if(!problem||!codes.includes(problem.code)||problem.dependency!==undefined&&!validName(problem.dependency))throw Error("invalid");
      if(["external_dependency_unverified","blocked_dependency"].includes(problem.code)&&(!validName(problem.dependency)||!item.dependencies.includes(problem.dependency)))throw Error("invalid");
      if(problem.code==="external_dependency_unverified")expectedExternal.add(problem.dependency);
      return {code:problem.code,...(problem.dependency===undefined?{}:{dependency:problem.dependency})};
    });
    if(!problems.length&&(!validName(expected)||files.filter(file=>file.document?.metadata?.name===expected).length!==1))throw Error("invalid");
    return {index,queue:expected,revision:item.revision,dependencies:[...item.dependencies],problems};
  });
  const seen=new Set(),orderedNames=new Set();
  for(const index of plan.order){
    if(!Number.isSafeInteger(index)||index<0||index>=items.length||seen.has(index)||items[index].problems.length||!items[index].revision||items[index].dependencies.some(name=>!orderedNames.has(name)))throw Error("invalid");
    seen.add(index);orderedNames.add(items[index].queue);
  }
  if(items.some(item=>!item.problems.length&&!seen.has(item.index))||plan.ready!==(seen.size===items.length))throw Error("invalid");
  const external=body.external_declarations.map(item=>{
    if(!item||!expectedExternal.delete(item.queue)||!["present","missing","unavailable","unrepresentable"].includes(item.status)||item.status==="present"&&!/^"[0-9]+"$/.test(item.etag??""))throw Error("invalid");
    return {queue:item.queue,status:item.status,...(item.status==="present"?{etag:item.etag}:{})};
  });
  if(expectedExternal.size)throw Error("invalid");
  // Older planning servers retain the conservative internal-only path.
  const review=plan.review_order===undefined?plan.order:plan.review_order;
  if(!Array.isArray(review))throw Error("invalid");
  const reviewSeen=new Set(),reviewNames=new Set(),inputNames=new Set(items.map(item=>item.queue));
  const reviewable=item=>item.problems.every(problem=>["external_dependency_unverified","blocked_dependency"].includes(problem.code));
  for(const index of review){
    const item=items[index];
    if(!Number.isSafeInteger(index)||!item||reviewSeen.has(index)||!reviewable(item)||!item.revision||!validName(item.queue)||items.filter(value=>value.queue===item.queue).length!==1)throw Error("invalid");
    for(const name of item.dependencies){
      if(inputNames.has(name)?!reviewNames.has(name):!item.problems.some(problem=>problem.code==="external_dependency_unverified"&&problem.dependency===name))throw Error("invalid");
    }
    reviewSeen.add(index);reviewNames.add(item.queue);
  }
  if(plan.order.some(index=>!reviewSeen.has(index)))throw Error("invalid");
  if(plan.review_order!==undefined&&items.some(item=>reviewable(item)&&!reviewSeen.has(item.index)&&item.dependencies.every(name=>!inputNames.has(name)||reviewNames.has(name))))throw Error("invalid");
  return {items,order:[...plan.order],...(plan.review_order===undefined?{}:{review_order:[...review]}),ready:plan.ready,external};
}

export function createBatchImport(api){
  let readGeneration=0,requestGeneration=0,controller,state={phase:"idle",files:[],result:null,failure:null};
  const listeners=new Set(),emit=next=>{state=next;for(const fn of listeners)fn();};
  const cancelRequest=()=>{requestGeneration++;controller?.abort();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){readGeneration++;cancelRequest();emit({phase:"idle",files:[],result:null,failure:null});},
    invalidatePlan(){cancelRequest();emit({...state,phase:state.phase==="planning"||state.phase==="ready"?"files":state.phase,result:null,failure:null});},
    async read(input){
      const generation=++readGeneration;cancelRequest();const files=Array.from(input??[]);
      emit({phase:"reading",files:files.map(file=>({name:file.name,status:"pending"})),result:null,failure:null});
      if(!files.length||files.length>100||files.some(file=>!Number.isSafeInteger(file.size)||file.size<1||file.size>1024*1024)||files.reduce((sum,file)=>sum+file.size,0)>limit){emit({...state,phase:"error",failure:"bounds"});return;}
      const rows=[];
      for(const file of files){
        try{
          const bytes=await file.arrayBuffer();if(generation!==readGeneration)return;
          if(!(bytes instanceof ArrayBuffer)||bytes.byteLength!==file.size)throw Error("invalid");
          rows.push({name:file.name,status:"loaded",document:parseJSON(new TextDecoder("utf-8",{fatal:true}).decode(bytes))});
        }catch{if(generation!==readGeneration)return;rows.push({name:file.name,status:"invalid"});}
      }
      emit({phase:"files",files:rows,result:null,failure:rows.some(row=>row.status!=="loaded")?"files":null});
    },
    async plan(binding){
      if(!state.files.length||state.files.some(file=>file.status!=="loaded")||!binding)return;
      cancelRequest();const generation=requestGeneration,files=state.files;controller=new AbortController();emit({...state,phase:"planning",result:null,failure:null});
      try{
        const body={documents:files.map(file=>file.document)};
        if(new TextEncoder().encode(stringifyJSON(body)).length>limit)throw Error("bounds");
        const response=await api.request("/api/v1/queues/import-plan",{method:"POST",body,headers:capabilityHeaders(binding),signal:controller.signal});
        if(generation!==requestGeneration)return;
        const result=validateBatchResult(response.body,files);emit({...state,phase:"ready",result,binding,failure:null});
      }catch(error){if(generation!==requestGeneration)return;emit({...state,phase:"files",result:null,failure:error.status===401||error.status===403?"denied":error.status===412?"changed":error.message==="bounds"?"bounds":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable"});}
    }
  };
}
