import {latestRead} from "./api.mjs";
import {nodeConnectionsURL} from "./routes.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";
import {validObservationTime} from "./observation-time.mjs";
export const connectionCounters=["pending_bytes","in_msgs","out_msgs","in_bytes","out_bytes","subscriptions"];
const integer=(value,max)=>typeof value==="bigint"?value>=0n&&value<=max:Number.isSafeInteger(value)&&value>=0&&BigInt(value)<=max;
export function connectionSample(row){
  if(!row||!integer(row.cid,18446744073709551615n)||BigInt(row.cid)===0n)throw Error("invalid");
  const safe={cid:row.cid};for(const key of connectionCounters)if(row[key]!==undefined){if(!integer(row[key],key==="subscriptions"?4294967295n:9223372036854775807n))throw Error("invalid");safe[key]=row[key];}return safe;
}
export function createConnections(api,id,query,options={}){
  const requested={...query};nodeConnectionsURL(id,requested);let identity=null;
  const reader=latestRead(api),listeners=new Set();let state={phase:"idle",page:null,readAt:null,failure:null};
  const emit=value=>{state=value;for(const fn of listeners)fn();};
  async function read(){
    const previous={page:state.page,readAt:state.readAt};emit({...previous,phase:"loading",failure:null});
    try{
      const active=identity?{offset:identity.offset,limit:identity.limit}:requested;
      const result=identity
        ?await reader.run(`/api/v1/nodes/${encodeURIComponent(id)}/connections/search`,{method:"POST",body:identity})
        :await reader.run(`/api/v1/nodes/${encodeURIComponent(id)}/connections?${new URLSearchParams(requested)}`);if(result.stale)return false;
      const page=result.result.body;
      if(!page||page.node_id!==id||page.offset!==active.offset||page.limit!==active.limit||!Number.isSafeInteger(page.total)||page.total<0||!Array.isArray(page.items)||page.items.length>Math.min(page.limit,Math.max(0,page.total-page.offset))||!identity&&requested.cid!==undefined&&(page.total>1||page.items.some(row=>String(row.cid)!==requested.cid))||[page.read_at,page.observed_at].some(v=>!validObservationTime(v)))throw Error("invalid");
      let prior=0n;const items=page.items.map(row=>{
        const safe=connectionSample(row);if(BigInt(safe.cid)<=prior)throw Error("invalid");prior=BigInt(safe.cid);return safe;
      });
      emit({phase:"ready",page:{node_id:id,offset:page.offset,limit:page.limit,total:page.total,observed_at:page.observed_at,read_at:page.read_at,items},readAt:new Date().toISOString(),failure:null,identity:identity&&{kind:identity.kind,offset:identity.offset,limit:identity.limit}});return true;
    }catch(error){
      const failure=error.status===401||error.status===403?"denied":error.status===422&&error.code==="connection_search_limit"?"limit":error.status===409?"ambiguous":error.status===404&&error.code==="read_api_disabled"?"disabled":error.status===404?"missing":error.status===400?"query":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable";
      emit({phase:"error",...(failure==="unavailable"?previous:{page:null,readAt:null}),failure});return false;
    }
  }
  const refresh=createRefreshLoop(read,options);
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},refresh,
    async search(kind,value,offset=0,limit=requested.limit){identity={kind,value,offset,limit};return read();},
    async searchPage(offset){if(!identity)return false;identity={...identity,offset};return read();},
    async clearSearch(){identity=null;return read();},
    clear(){identity=null;refresh.stop();reader.cancel();emit({phase:"idle",page:null,readAt:null,failure:null});}};
}
