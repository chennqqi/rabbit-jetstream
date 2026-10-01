import {latestRead} from "./api.mjs";
import {nodeConnectionURL} from "./routes.mjs";
import {validObservationTime} from "./observation-time.mjs";

const maxInt64=9223372036854775807n,encoder=new TextEncoder(),control=/[\u0000-\u001f\u007f-\u009f]/u;
const integer=value=>(typeof value==="bigint"?value>=0n&&value<=maxInt64:Number.isSafeInteger(value)&&value>=0&&BigInt(value)<=maxInt64);
const text=(value,max,empty=false)=>typeof value==="string"&&(empty||value.length>0)&&encoder.encode(value).length<=max&&!control.test(value);

export function connectionSubscriptions(body,id,cid){
  cid=String(BigInt(cid));
  if(!body||body.node_id!==id||String(body.cid)!==cid||!validObservationTime(body.observed_at)||!validObservationTime(body.read_at)||!Array.isArray(body.items)||body.items.length>1000)throw Error("invalid");
  let previous=null;
  const items=body.items.map(row=>{
    if(!row||!text(row.sid,256)||!text(row.subject,1024)||row.queue!==undefined&&!text(row.queue,512,true)||!integer(row.messages)||row.maximum!==undefined&&!integer(row.maximum)||previous!==null&&row.sid<=previous)throw Error("invalid");
    previous=row.sid;return {sid:row.sid,subject:row.subject,...(row.queue!==undefined?{queue:row.queue}:{}),messages:row.messages,...(row.maximum!==undefined?{maximum:row.maximum}:{})};
  });
  return {node_id:id,cid:body.cid,observed_at:body.observed_at,read_at:body.read_at,items};
}

export function subscriptionPage(observation,query="",offset=0,limit=50){
  if(!observation||typeof query!=="string"||encoder.encode(query.trim()).length>256||!Number.isSafeInteger(offset)||offset<0||!Number.isSafeInteger(limit)||limit<1||limit>200)throw Error("invalid");
  const needle=query.trim().toLocaleLowerCase();
  const filtered=needle?observation.items.filter(row=>[row.sid,row.subject,row.queue??""].some(value=>value.toLocaleLowerCase().includes(needle))):observation.items;
  return {total:filtered.length,items:filtered.slice(offset,offset+limit)};
}

export function createConnectionSubscriptions(api,id,cid){
  nodeConnectionURL(id,cid);cid=String(BigInt(cid));
  const reader=latestRead(api),listeners=new Set();let state={phase:"idle",observation:null,failure:null};
  const emit=value=>{state=value;for(const fn of listeners)fn();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    async load(){const previous=state.observation;emit({phase:"loading",observation:previous,failure:null});try{const result=await reader.run(`/api/v1/nodes/${encodeURIComponent(id)}/connections/${cid}/subscriptions`);if(result.stale)return;emit({phase:"ready",observation:connectionSubscriptions(result.result.body,id,cid),failure:null});}catch(error){const failure=error.status===401||error.status===403?"denied":error.status===422&&error.code==="subscription_limit_exceeded"?"limit":error.status===409?"ambiguous":error.status===404?(error.code==="connection_not_found"?"missing":error.code==="read_api_disabled"?"disabled":"node_missing"):error.status===400?"query":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable";emit({phase:"error",observation:failure==="unavailable"?previous:null,failure});}},
    clear(){reader.cancel();emit({phase:"idle",observation:null,failure:null});}};
}
