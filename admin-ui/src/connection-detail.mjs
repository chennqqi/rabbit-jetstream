import {latestRead} from "./api.mjs";
import {nodeConnectionURL} from "./routes.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";
import {validObservationTime} from "./observation-time.mjs";
import {connectionSample} from "./connections.mjs";

export function createConnectionDetail(api,id,cid,options={}){
  nodeConnectionURL(id,cid);cid=String(BigInt(cid));
  const reader=latestRead(api),listeners=new Set(),empty=()=>({phase:"idle",detail:null,readAt:null,failure:null});let state=empty();
  const emit=value=>{state=value;for(const fn of listeners)fn();};
  async function read(){
    const previous={detail:state.detail,readAt:state.readAt};emit({...previous,phase:"loading",failure:null});
    try{
      const response=await reader.run(`/api/v1/nodes/${encodeURIComponent(id)}/connections/${cid}`);if(response.stale)return false;
      const body=response.result.body;
      if(!body||body.node_id!==id||!validObservationTime(body.observed_at)||!validObservationTime(body.read_at))throw Error("invalid");
      const item=connectionSample(body.item);if(String(item.cid)!==cid)throw Error("invalid");
      emit({phase:"ready",detail:{node_id:id,observed_at:body.observed_at,read_at:body.read_at,item},readAt:new Date().toISOString(),failure:null});return true;
    }catch(error){
      const failure=error.status===401||error.status===403?"denied":error.status===409?"ambiguous":error.status===404?(error.code==="read_api_disabled"?"disabled":error.code==="connection_not_found"?"missing":error.code==="not_found"?"node_missing":"invalid"):error.status===400?"query":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable";
      emit({phase:"error",...(failure==="unavailable"?previous:{detail:null,readAt:null}),failure});return false;
    }
  }
  const refresh=createRefreshLoop(read,options);
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},refresh,clear(){refresh.stop();reader.cancel();emit(empty());}};
}
