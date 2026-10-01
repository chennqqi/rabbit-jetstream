import {validCapabilities} from "./capabilities.mjs";
import {stringifyJSON,parseJSON} from "./api.mjs";
import {readQueueSchema} from "./queue-schema.mjs";

function canonical(value){
  if(Array.isArray(value))return value.map(canonical);
  if(value&&typeof value==="object")return Object.fromEntries(Object.keys(value).sort().map(key=>[key,canonical(value[key])]));
  return value;
}
export function capabilityFingerprint(body){
  if(!validCapabilities(body))throw Object.assign(new Error("Capabilities unavailable or incompatible"),{code:"capabilities_unavailable"});
  const normalized=parseJSON(stringifyJSON(body));
  normalized.features.sort();normalized.queue.supportedReplicas.sort((a,b)=>a-b);normalized.queue.supportedStorage.sort();
  return stringifyJSON(canonical(normalized));
}
export async function readMutationCapabilities(api,features,expected){
  try{
    const response=await api.request("/api/v1/console/capabilities");
    const fingerprint=capabilityFingerprint(response.body);
    const revision=response.headers?.get("ETag");
    if(!/^"rjs-capabilities-v1:[0-9a-f]{64}"$/.test(revision??"")||[...features,"capability-preconditions"].some(feature=>!response.body.features.includes(feature)))throw new Error("Required contract revision not advertised");
    if(expected&&(fingerprint!==expected.fingerprint||revision!==expected.revision))throw Object.assign(new Error("Capabilities changed; preview again"),{code:"capabilities_changed"});
    const schema=await readQueueSchema(api,response.body,revision);
    if(expected&&stringifyJSON(canonical(schema))!==stringifyJSON(canonical(expected.schema)))throw Object.assign(new Error("Queue schema changed; preview again"),{code:"capabilities_changed"});
    return {fingerprint,revision,body:parseJSON(stringifyJSON(response.body)),...(schema?{schema}:{})};
  }catch(error){throw Object.assign(new Error("Capability check failed before mutation"),{code:error.code==="capabilities_changed"?error.code:"capabilities_unavailable",status:error.status});}
}

export const capabilityHeaders=binding=>binding?{"X-RJS-If-Capabilities-Match":binding.revision}:{};
export const capabilityCheckPreventedDispatch=state=>["preview-error","denied","conflict"].includes(state.phase)&&["capabilities_changed","capabilities_unavailable"].includes(state.error?.code);
export function observedCapabilityChange(binding,observation){
  if(!binding)return false;
  if(observation?.schema){
    if(!binding.schema)return false;
    try{return observation.schema.revision!==binding.body.queue.schema.etag||
      stringifyJSON(canonical(observation.schema.body))!==stringifyJSON(canonical(binding.schema));}
    catch{return true;}
  }
  try{return !observation||binding.revision!==observation.revision||binding.fingerprint!==capabilityFingerprint(observation.body);}
  catch{return true;}
}
