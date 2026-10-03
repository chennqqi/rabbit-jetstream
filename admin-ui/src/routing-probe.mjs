import {latestRead} from "./api.mjs";

export function routingProbePath(name, input) {
  if(!input||typeof input!=="object"||Array.isArray(input))throw Error("query");
  if(typeof name!=="string"||!/^[A-Za-z0-9_-]{1,256}$/.test(name))throw Error("query");
  const literal=Object.hasOwn(input,"subject"),allowed=literal?["subject"]:["exchange","type","routingKey"];
  if(Object.keys(input).some(key=>!allowed.includes(key))||Object.values(input).some(value=>typeof value!=="string"||new TextEncoder().encode(value).length>1024))throw Error("query");
  if(literal?!input.subject:!input.exchange||!["direct","topic","fanout"].includes(input.type))throw Error("query");
  const query=new URLSearchParams(input).toString();if(query.length>4096)throw Error("query");
  return `/api/v1/queues/${encodeURIComponent(name)}/routing-probe?${query}`;
}

const strings=value=>Array.isArray(value)&&value.every(item=>typeof item==="string"&&item.length>0);
function routingFailure(error){
  // Authorization takes precedence even if an intermediary replaces the body.
  // Other resource conclusions require a recognized, decoded error contract.
  if(error.status===401||error.status===403)return "denied";
  if(error.kind==="invalid-response"||error.message==="invalid")return "invalid";
  if(error.status===404)return error.code==="read_api_disabled"?"disabled":error.code==="not_found"?"missing":"invalid";
  if(error.status===409)return error.code==="routing_declaration_unavailable"?"changed":"invalid";
  if(error.message==="changed")return "changed";
  if(error.status===400||error.message==="query")return "query";
  if(error.status===503&&error.code==="routing_probe_limit")return "limit";
  return "unavailable";
}
function evidence(body,name,revision,input){
  if(!body||body.queue!==name||body.revision!==revision||typeof body.stream!=="string"||!body.stream||typeof body.subject!=="string"||!body.subject||!strings(body.streamSubjects)||!body.streamSubjects.length||!strings(body.matchedStreamSubjects)||!Array.isArray(body.bindings))throw Error("invalid");
  if(Object.hasOwn(input,"subject")&&body.subject!==input.subject)throw Error("invalid");
  if(body.matchedStreamSubjects.some(s=>!body.streamSubjects.includes(s)))throw Error("invalid");
  const bindings=body.bindings.map(row=>{
    if(!row||typeof row.exchange!=="string"||!row.exchange||!["direct","topic","fanout"].includes(row.type)||!(strings(row.keys)||(row.type==="fanout"&&row.keys===null))||!strings(row.subjects)||!row.subjects.length||!strings(row.matchedSubjects)||row.matchedSubjects.some(s=>!row.subjects.includes(s)))throw Error("invalid");
    return {exchange:row.exchange,type:row.type,keys:row.keys??[],subjects:row.subjects,matchedSubjects:row.matchedSubjects};
  });
  return {queue:name,revision,stream:body.stream,subject:body.subject,streamSubjects:body.streamSubjects,matchedStreamSubjects:body.matchedStreamSubjects,bindings};
}

export function createRoutingProbe(api,name,revision,etag){
  const reader=latestRead(api),listeners=new Set(),empty=()=>({phase:"idle",result:null,failure:null,readAt:null});let state=empty();
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},clear(){reader.cancel();emit(empty());},
    async run(input){
      reader.cancel();emit({...empty(),phase:"loading"});
      try{
        const response=await reader.run(routingProbePath(name,input));if(response.stale)return;
        if(typeof response.result.body?.revision!=="string"||!response.result.body.revision||typeof response.result.headers?.get("ETag")!=="string")throw Error("invalid");
        if(response.result.body?.revision!==revision||response.result.headers?.get("ETag")!==etag)throw Error("changed");
        const result=evidence(response.result.body,name,revision,input);
        emit({phase:"ready",result,failure:null,readAt:new Date().toISOString()});
      }catch(error){
        const failure=routingFailure(error);
        emit({...empty(),phase:"error",failure});
      }
    }
  };
}
