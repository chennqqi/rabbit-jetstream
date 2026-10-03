import {latestRead} from "./api.mjs";

export function createConsumerDetail(api, stream, name, {retainOnRefresh=false}={}) {
  const reader = latestRead(api), listeners = new Set();
  let state = {phase:"idle", resource:null, failure:null, readAt:null};
  const emit = next => {state=next;for(const fn of listeners)fn();};
  return {
    snapshot:()=>state,
    subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){reader.cancel();emit({phase:"idle",resource:null,failure:null,readAt:null});},
    async load(){
      reader.cancel();const previous=state;emit(retainOnRefresh?{...state,phase:"loading"}:{phase:"loading",resource:null,failure:null,readAt:null});
      try {
        const response=await reader.run(`/api/v1/streams/${encodeURIComponent(stream)}/consumers/${encodeURIComponent(name)}`);
        if(response.stale)return;
        const resource=response.result.body;
        if(!resource || resource.stream!==stream || resource.name!==name || !["pull","push"].includes(resource.mode) ||
          !(resource.filter_subjects===null || Array.isArray(resource.filter_subjects) && resource.filter_subjects.every(v=>typeof v==="string")) ||
          resource.filter_subject!==undefined && typeof resource.filter_subject!=="string" ||
          ["durable","ack_policy"].some(field=>resource[field]!==undefined&&typeof resource[field]!=="string"))throw new Error("invalid-resource");
        emit({phase:"ready",resource,failure:null,readAt:new Date().toISOString()});
      }catch(error){
        const failure=error.status===404 && error.code==="not_found" ? "missing" :
          error.status===401 || error.status===403 ? "denied" : error.status===404&&error.code==="read_api_disabled"?"disabled":error.message==="invalid-resource" || error.kind==="invalid-response" ? "invalid" : "unavailable";
        emit({...(retainOnRefresh&&failure==="unavailable"?previous:{resource:null,readAt:null}),phase:"error",failure});
      }
    },
  };
}
