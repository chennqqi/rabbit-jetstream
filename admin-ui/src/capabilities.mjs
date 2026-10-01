import {latestRead} from "./api.mjs";
import {readQueueSchema,validSchemaDescriptor} from "./queue-schema.mjs";

const strings=value=>Array.isArray(value)&&value.every(item=>typeof item==="string"&&item.length>0)&&new Set(value).size===value.length;
const digest=value=>typeof value==="string"&&/^[a-f0-9]{64}$/.test(value);
const validQualification=value=>value?.status==="unreported"?(value.statement===undefined&&value.manifestDigest===undefined):
  value?.status==="reported"&&typeof value.statement==="string"&&value.statement.length>0&&value.statement.length<=512&&value.statement.trim()===value.statement&&digest(value.manifestDigest);
export function validCapabilities(body){
  const q=body?.queue,d=body?.deployment;
  return body?.schemaVersion==="rjs.console-capabilities.v1"&&
    ["unknown","standalone","cluster"].includes(d?.profile)&&d.source===(d.profile==="unknown"?"unspecified":"configuration")&&
    q?.apiVersion==="rabbit-jetstream.io/v1alpha1"&&q.kind==="Queue"&&q.requiresExplicitReplicas===true&&
    Array.isArray(q.supportedReplicas)&&q.supportedReplicas.length>0&&q.supportedReplicas.every(value=>Number.isSafeInteger(value)&&value>0)&&new Set(q.supportedReplicas).size===q.supportedReplicas.length&&
    strings(q.supportedStorage)&&q.supportedStorage.length>0&&
    Number.isSafeInteger(q.minimumPriority)&&Number.isSafeInteger(q.maximumPriority)&&q.minimumPriority>=0&&q.maximumPriority>=q.minimumPriority&&
    q.supportedStorage.includes(q.defaults?.storage)&&typeof q.defaults?.delivery?.ackWait==="string"&&q.defaults.delivery.ackWait.length>0&&
    Number.isSafeInteger(q.defaults.delivery.maxDeliver)&&q.defaults.delivery.maxDeliver>0&&strings(body.features)&&
    (!body.features.includes("queue-schema")||validSchemaDescriptor(q.schema))&&validQualification(body.qualification);
}

export function createCapabilities(api){
  const reader=latestRead(api),listeners=new Set();let state={phase:"idle"},generation=0;
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){generation++;reader.cancel();emit({phase:"idle"});},
    async load(){
      const current=++generation;
      emit({phase:"loading"});
      try{
        const result=await reader.run("/api/v1/console/capabilities");if(result.stale)return;
        if(!validCapabilities(result.result.body))throw new Error("invalid-capabilities");
        const schema=await readQueueSchema(api,result.result.body,result.result.headers?.get("ETag"));
        if(current!==generation)return;
        emit({phase:"ready",body:result.result.body,schema,readAt:new Date().toISOString()});
      }catch(error){if(current===generation)emit({phase:"error",status:error.status??null});}
    },
  };
}
