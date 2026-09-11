import {parseJSON,stringifyJSON} from "./api.mjs";

// Field paths pinned to internal/topology/queue.go v1alpha1. No client defaults.
export const queueFields=[
  {key:"replicas",path:["spec","replicas"],type:"integer",en:"Replicas",zh:"副本数",options:["1","3","5"]},
  {key:"storage",path:["spec","storage"],en:"Storage",zh:"存储类型",options:["file","memory"]},
  {key:"maxAge",path:["spec","retention","maxAge"],en:"Maximum age (duration)",zh:"最大保留时间（时长）"},
  {key:"maxBytes",path:["spec","retention","maxBytes"],en:"Maximum bytes (size)",zh:"最大保留大小（容量）"},
  {key:"maxMessages",path:["spec","retention","maxMessages"],type:"integer",en:"Maximum retained messages",zh:"最大保留消息数"},
  {key:"ackWait",path:["spec","delivery","ackWait"],en:"Acknowledgement wait (duration)",zh:"确认等待时间（时长）"},
  {key:"maxDeliver",path:["spec","delivery","maxDeliver"],type:"integer",en:"Maximum delivery attempts",zh:"最大投递次数"},
  {key:"maxPriority",path:["spec","maxPriority"],type:"integer",en:"Maximum priority (optional)",zh:"最大优先级（可选）"},
  {key:"deadLetter",path:["spec","deadLetter","queue"],en:"Dead-letter Queue (optional)",zh:"死信 Queue（可选）"},
];
const object=value=>value!==null&&typeof value==="object"&&!Array.isArray(value);
const keys=(value,allowed)=>object(value)&&Object.keys(value).every(key=>allowed.includes(key));
export function supportsQueueFields(document) {
  if(!keys(document,["apiVersion","kind","metadata","spec"])||document.apiVersion!=="rabbit-jetstream.io/v1alpha1"||document.kind!=="Queue")return false;
  if(!keys(document.metadata,["name","labels"])||typeof document.metadata.name!=="string")return false;
  if(document.metadata.labels!==undefined&&(!object(document.metadata.labels)||!Object.values(document.metadata.labels).every(v=>typeof v==="string")))return false;
  const spec=document.spec;
  if(!keys(spec,["subjects","bindings","replicas","storage","retention","delivery","maxPriority","deadLetter"]))return false;
  if(spec.subjects!=null&&(!Array.isArray(spec.subjects)||!spec.subjects.every(v=>typeof v==="string")))return false;
  if(spec.bindings!==undefined&&(!Array.isArray(spec.bindings)||!spec.bindings.every(v=>keys(v,["exchange","type","keys"])&&typeof v.exchange==="string"&&typeof v.type==="string"&&(v.keys===undefined||(Array.isArray(v.keys)&&v.keys.every(k=>typeof k==="string"))))))return false;
  for(const [key,allowed] of [["retention",["maxAge","maxBytes","maxMessages"]],["delivery",["ackWait","maxDeliver"]],["deadLetter",["queue"]]])if(spec[key]!==undefined&&!keys(spec[key],allowed))return false;
  return queueFields.every(field=>{
    const value=field.path.reduce((v,key)=>v?.[key],document);
    return value===undefined||typeof value==="string"||(field.type==="integer"&&(typeof value==="bigint"||Number.isSafeInteger(value)));
  });
}
export function queueFieldValue(document,field) {
  const value=field.path.reduce((v,key)=>v?.[key],document);
  return value===undefined?"":String(value);
}
export function editQueueField(document,key,input,fields=queueFields) {
  const field=fields.find(field=>field.key===key);
  if(!field||typeof input!=="string"||!supportsQueueFields(document))throw new Error("Unsupported structured edit; retain expert JSON");
  const next=parseJSON(stringifyJSON(document));
  let parent=next;
  for(const part of field.path.slice(0,-1))parent=parent[part]??(parent[part]={});
  const property=field.path.at(-1);
  if(input===""){
    delete parent[property];
    if(field.key==="deadLetter")delete next.spec.deadLetter;
  }else{
    // Incomplete/invalid numeric input remains a string in the ONE draft.
    // Server preview validates it; never clamp, round or silently drop input.
    parent[property]=field.type==="integer"&&/^-?(?:0|[1-9][0-9]*)$/.test(input)?BigInt(input):input;
  }
  return stringifyJSON(next);
}
