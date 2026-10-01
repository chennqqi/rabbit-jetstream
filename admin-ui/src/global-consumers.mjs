import {latestRead} from "./api.mjs";

const exact=value=>typeof value==="bigint"&&value>=0n||Number.isSafeInteger(value)&&value>=0;
export function decodeGlobalConsumerPage(page,query){
  if(!page||!["ready","stale"].includes(page.state)||typeof page.generation_id!=="string"||!page.generation_id||!Array.isArray(page.items)||!Number.isSafeInteger(page.total)||page.total<0||!Number.isSafeInteger(page.offset)||page.offset!==Math.min(query.offset,page.total)||page.limit!==query.limit||page.items.length!==Math.min(page.limit,page.total-page.offset))throw new Error("invalid-page");
  const identities=new Set();
  for(const row of page.items){
    if(!row||typeof row.stream!=="string"||!row.stream||typeof row.name!=="string"||!row.name||typeof row.queue!=="string"&&row.queue!==undefined||typeof row.durable!=="string"&&row.durable!==undefined||!["pull","push"].includes(row.mode)||!["present","missing","mismatched"].includes(row.status)||!["matching","different","external","unknown"].includes(row.ownership)||row.pending!==null&&!exact(row.pending)||row.ack_pending!==null&&!exact(row.ack_pending)||row.status==="missing"&&(row.pending!==null||row.ack_pending!==null))throw new Error("invalid-row");
    const identity=`${row.stream}\0${row.name}`;if(identities.has(identity))throw new Error("duplicate-row");identities.add(identity);
  }
  return page;
}

export function createGlobalConsumers(api){
  const reader=latestRead(api),listeners=new Set();let state={phase:"idle",query:null,page:null,failure:null};
  const emit=next=>{state=next;for(const listener of listeners)listener();};
  return {snapshot:()=>state,subscribe(listener){listeners.add(listener);return()=>listeners.delete(listener);},clear(){reader.cancel();emit({phase:"idle",query:null,page:null,failure:null});},async load(query){
    reader.cancel();const previous=state.query&&Object.entries(query).every(([key,value])=>key==="generation"||state.query[key]===value)?state.page:null;emit({phase:"loading",query,page:previous,failure:null});
    try{const params=new URLSearchParams({...query,sort:"identity"});if(!query.generation)params.delete("generation");const response=await reader.run(`/api/v1/consumers?${params}`);if(response.stale)return;const page=decodeGlobalConsumerPage(response.result.body,query);emit({phase:"ready",query:{...query,generation:page.generation_id},page,failure:null});}
    catch(error){if(error.stale)return;const serverState=error.body?.state;const kind=error.status===409&&error.code==="consumer_generation_changed"?"generation-changed":error.status===503&&["unavailable","collecting"].includes(serverState)?serverState:error.status===401?"credentials-rejected":error.status===403?"role-denied":error.status===400?"invalid-query":error.message==="invalid-page"||error.message==="invalid-row"||error.message==="duplicate-row"?"invalid-response":"unavailable";emit({phase:"error",query,page:["unavailable","invalid-response"].includes(kind)?previous:null,failure:{kind}});}
  },async refreshCollection(){
    try{const response=await api.request("/api/v1/consumers/refresh",{method:"POST",timeout:32000});return {ok:true,busy:response.status===202};}catch(error){return {ok:false,kind:error.status===401?"credentials-rejected":error.status===403?"role-denied":"collection-failed"};}
  }};
}
