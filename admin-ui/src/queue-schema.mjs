import {parseJSON,stringifyJSON} from "./api.mjs";

export const queueSchemaPath="/api/v1/console/queue-schema";
export function validSchemaDescriptor(value){
  return value?.id==="urn:rabbit-jetstream:queue:v1alpha1:schema:1"&&value.version==="rjs.queue-schema.v1"&&
    value.url===queueSchemaPath&&/^"rjs-queue-schema-v1:[0-9a-f]{64}"$/.test(value.etag??"");
}
const sameSet=(a,b)=>Array.isArray(a)&&Array.isArray(b)&&a.length===b.length&&new Set(a).size===a.length&&a.every(value=>b.includes(value));
function objectFields(node,fields,required){
  return node?.type==="object"&&node.additionalProperties===false&&
    sameSet(Object.keys(node.properties??{}),fields)&&sameSet(node.required??[],required);
}
export function compatibleQueueSchema(schema,capabilities){
  const q=capabilities?.queue,p=schema?.properties,spec=p?.spec?.properties;
  if(!validSchemaDescriptor(q?.schema)||schema?.$id!==q.schema.id||schema?.["x-rjs-version"]!==q.schema.version||
    schema?.$schema!=="https://json-schema.org/draft/2020-12/schema")return false;
  if(!objectFields(schema,["apiVersion","kind","metadata","spec"],["apiVersion","kind","metadata","spec"])||
    !objectFields(p.metadata,["name","labels"],["name"])||
    !objectFields(p.spec,["subjects","bindings","replicas","storage","maxPriority","retention","delivery","deadLetter"],["replicas"])||
    !objectFields(spec.retention,["maxAge","maxBytes","maxMessages"],[])||
    !objectFields(spec.delivery,["ackWait","maxDeliver"],[])||
    !objectFields(spec.deadLetter,["queue"],["queue"])||
    !objectFields(spec.bindings?.items,["exchange","type","keys"],["exchange","type"]))return false;
  return p.apiVersion.const===q.apiVersion&&p.kind.const===q.kind&&
    spec.replicas.type==="integer"&&sameSet(spec.replicas.enum,q.supportedReplicas)&&
    spec.storage.type==="string"&&sameSet(spec.storage.enum,q.supportedStorage)&&spec.storage.default===q.defaults.storage&&
    spec.maxPriority.minimum===q.minimumPriority&&spec.maxPriority.maximum===q.maximumPriority&&
    spec.delivery.properties.ackWait.default===q.defaults.delivery.ackWait&&
    spec.delivery.properties.maxDeliver.default===q.defaults.delivery.maxDeliver;
}

// Fetch only the pinned same-origin route, never an arbitrary URL from metadata.
// This checks the contract we consume, not general JSON Schema satisfiability.
export async function readQueueSchema(api,capabilities,capabilityRevision){
  if(!capabilities.features.includes("queue-schema"))return undefined;
  if(!validSchemaDescriptor(capabilities.queue.schema)||!/^"rjs-capabilities-v1:[0-9a-f]{64}"$/.test(capabilityRevision??""))
    throw new Error("Missing or unsupported Queue schema descriptor");
  const response=await api.request(queueSchemaPath,{headers:{"X-RJS-If-Capabilities-Match":capabilityRevision}});
  if(response.headers?.get("ETag")!==capabilities.queue.schema.etag||!compatibleQueueSchema(response.body,capabilities))
    throw new Error("Queue schema changed or is incompatible");
  return parseJSON(stringifyJSON(response.body));
}
