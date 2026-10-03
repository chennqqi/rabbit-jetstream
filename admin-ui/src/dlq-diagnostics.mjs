import {latestRead} from "./api.mjs";
import {validObservationTime} from "./observation-time.mjs";
const empty=()=>({phase:"unobserved",readAt:null});
const name=value=>typeof value==="string"&&/^[A-Za-z0-9_-]+$/.test(value);
export function dlqTarget(plan){
  if(plan?.deadLetter===undefined||plan.deadLetter===null)return null;
  const value=plan.deadLetter;
  if(!name(plan.queue)||!name(value.queue)||value.queue===plan.queue||!name(value.stream)||value.mechanism!=="advisory-republish")throw Error("invalid DLQ plan");
  return {queue:value.queue,stream:value.stream,mechanism:value.mechanism};
}
export function createDLQDiagnostics(api,plan){
  const target=dlqTarget(plan),listeners=new Set(),readers={target:latestRead(api),stream:latestRead(api),controller:latestRead(api)};
  let generation=0,state={target:empty(),stream:empty(),controller:empty()};
  const emit=(key,value)=>{state={...state,[key]:value};for(const fn of listeners)fn();};
  async function read(key,path,validate,current){
    if(current!==generation)return;
    emit(key,{phase:"loading",readAt:null});
    try{
      const response=await readers[key].run(path);if(response.stale||current!==generation)return;
      const value=validate(response.result.body),result={phase:"available",value,etag:response.result.headers?.get("ETag")??null,readAt:new Date().toISOString()};
      emit(key,result);return result;
    }catch(error){if(current===generation)emit(key,{phase:error.status===401||error.status===403?"denied":error.status===404&&error.code==="not_found"?"missing":error.message==="invalid"||error.kind==="invalid-response"?"invalid":"unavailable",readAt:null});}
  }
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){generation++;for(const reader of Object.values(readers))reader.cancel();for(const key of Object.keys(state))emit(key,empty());},
    async load(){
      if(!target)return;const current=++generation;for(const reader of Object.values(readers))reader.cancel();for(const key of Object.keys(state))emit(key,empty());
      await Promise.all([
        (async()=>{
          const declaration=await read("target",`/api/v1/queues/${encodeURIComponent(target.queue)}`,body=>{
            if(body?.queue!==target.queue||body.plan?.queue!==target.queue||typeof body.revision!=="string"||body.plan.revision!==body.revision||body.plan.stream?.name!==target.stream)throw Error("invalid");
            return {queue:body.queue,revision:body.revision,stream:body.plan.stream.name};
          },current);
          if(!declaration||current!==generation)return;
          await read("stream",`/api/v1/streams/${encodeURIComponent(target.stream)}`,body=>{if(body?.name!==target.stream)throw Error("invalid");return {name:body.name};},current);
        })(),
        read("controller","/api/v1/controller",body=>{
          if(!body||typeof body.enabled!=="boolean"||typeof body.leader!=="boolean"||typeof body.instanceId!=="string")throw Error("invalid");
          const counters={};for(const field of ["dlqProcessed","dlqMoved","dlqFailed","dlqIgnored"]){const value=body[field];if(value!==undefined&&!(typeof value==="bigint"&&value>=0n||Number.isSafeInteger(value)&&value>=0))throw Error("invalid");counters[field]=value;}
          const times={};for(const field of ["lastRun","lastSuccess"]){
            const value=body[field];
            // Only an exact serialized zero is unreported, not every value
            // sharing its prefix (for example one tenth of a second later).
            if(value===undefined||typeof value==="string"&&/^0001-01-01T00:00:00(?:\.0{1,9})?Z$/.test(value)){times[field]=undefined;continue;}
            if(!validObservationTime(value))throw Error("invalid");times[field]=value;
          }
          if(body.lastError!==undefined&&typeof body.lastError!=="string")throw Error("invalid");
          return {instanceId:body.instanceId||undefined,enabled:body.enabled,leader:body.leader,errorReported:!!body.lastError,...times,...counters};
        },current),
      ]);
    },
  };
}
