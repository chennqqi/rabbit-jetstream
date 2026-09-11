import {latestRead} from "./api.mjs";
import {nodeDetailURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";

export function nodeSourceAvailable(node, source) {
  return node?.sources?.[source]?.available === true;
}

export function nodeJSMetric(node, field) {
  return nodeSourceAvailable(node,"jsz") ? consumerCounter({observed:node.jetstream},field) : null;
}

export function createNodes(api,{retainOnRefresh=false}={}) {
  const reader = latestRead(api), listeners = new Set();
  let state = {phase:"idle", snapshot:null, readAt:null, failure:null};
  const emit = next => {state=next; for (const fn of listeners) fn();};
  return {
    snapshot:()=>state,
    subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){reader.cancel();emit({phase:"idle",snapshot:null,readAt:null,failure:null});},
    async load(){
      reader.cancel();const previous=state;emit(retainOnRefresh?{...state,phase:"loading"}:{phase:"loading",snapshot:null,readAt:null,failure:null});
      try {
        const result=await reader.run("/api/v1/nodes");if(result.stale)return;
        const snapshot=result.result.body;
        if(!snapshot || !Array.isArray(snapshot.nodes) || snapshot.total!==snapshot.nodes.length ||
          snapshot.nodes.some(node=>!node || typeof node.endpoint!=="string" || !["available","degraded","unavailable"].includes(node.status) ||
            node.id!==undefined && typeof node.id!=="string" || node.errors!==undefined && (!Array.isArray(node.errors)||node.errors.some(v=>typeof v!=="string"))))throw new Error("invalid");
        for (const node of snapshot.nodes) {
          if(node.id){try{nodeDetailURL(node.id);}catch{throw new Error("invalid");}}
          if(node.sources!==undefined && (!node.sources || typeof node.sources!=="object" || Array.isArray(node.sources)))throw new Error("invalid");
          for(const source of Object.values(node.sources??{})) {
            if(!source || typeof source.available!=="boolean" || typeof source.read_at!=="string" || !Number.isFinite(Date.parse(source.read_at)))throw new Error("invalid");
          }
        }
        emit({phase:"ready",snapshot,readAt:new Date().toISOString(),failure:null});
      }catch(error){
        const denied=error.status===401||error.status===403,disabled=error.status===404&&error.code==="read_api_disabled";
        emit({...(!denied&&!disabled&&retainOnRefresh?previous:{snapshot:null,readAt:null}),phase:"error",failure:denied?"denied":disabled?"disabled":error.message==="invalid"?"invalid":"unavailable"});
      }
    },
  };
}
