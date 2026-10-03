import {latestRead,parseJSON,stringifyJSON} from "./api.mjs";

function canonical(value){
  if(Array.isArray(value))return value.map(canonical);
  if(value&&typeof value==="object")return Object.fromEntries(Object.keys(value).sort().map(key=>[key,canonical(value[key])]));
  return value;
}

export function createQueueExport(api,name,document,etag){
  const reader=latestRead(api),listeners=new Set(),empty=()=>({phase:"idle",file:null,failure:null});let state=empty();
  const emit=next=>{state=next;for(const listener of listeners)listener();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},clear(){reader.cancel();emit(empty());},
    async prepare(includeLabels=false){
      reader.cancel();emit({...empty(),phase:"loading"});
      try{
        if(typeof includeLabels!=="boolean"||!/^[A-Za-z0-9_-]{1,256}$/.test(name)||!/^"[1-9][0-9]*"$/.test(etag??"")||!document||document.apiVersion!=="rabbit-jetstream.io/v1alpha1"||document.kind!=="Queue"||document.metadata?.name!==name||!document.spec)throw Error("invalid");
        const expected=parseJSON(stringifyJSON(document));
        const omitted=includeLabels?0:Object.keys(expected.metadata.labels??{}).length;
        if(!includeLabels)delete expected.metadata.labels;
        const response=await reader.run(`/api/v1/queues/${encodeURIComponent(name)}/export?include_labels=${includeLabels}`);
        if(response.stale)return;
        const {body,headers}=response.result;
        if(headers.get("ETag")!==etag)throw Error("changed");
        if(headers.get("X-RJS-Export-Scope")!=="single-queue-declaration"||headers.get("X-RJS-Export-Omitted-Labels")!==String(omitted)||headers.get("Content-Disposition")!==`attachment; filename="${name}.queue.json"`||stringifyJSON(canonical(body))!==stringifyJSON(canonical(expected)))throw Error("invalid");
        const content=stringifyJSON(body)+"\n";
        if(new TextEncoder().encode(content).length>1024*1024)throw Error("limit");
        const changeContent=includeLabels?stringifyJSON({schema:"rjs.queue-change.v1",etag,document:body})+"\n":null;
        emit({phase:"ready",failure:null,file:{content,changeContent,name:`${name}.queue.json`,changeName:`${name}.queue-change.json`,etag,omitted,includeLabels,readAt:new Date().toISOString()}});
      }catch(error){
        const failure=error.status===401||error.status===403?"denied":error.kind==="invalid-response"||error.message==="invalid"?"invalid":error.message==="changed"?"changed":error.status===404?(error.code==="not_found"?"missing":error.code==="read_api_disabled"?"disabled":"invalid"):error.status===409&&error.code==="export_declaration_unavailable"?"changed":error.message==="limit"||error.status===503&&error.code==="export_limit"?"limit":"unavailable";
        emit({...empty(),phase:"error",failure});
      }
    }
  };
}
