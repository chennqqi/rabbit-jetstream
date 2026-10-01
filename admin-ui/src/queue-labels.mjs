import {parseJSON,stringifyJSON} from "./api.mjs";
import {supportsQueueFields} from "./queue-fields.mjs";

export function editQueueLabel(document,action,{confirmed=false}={}) {
  if(!supportsQueueFields(document))throw new Error("Unsupported document");
  const labels=document.metadata.labels??{};
  const fail=code=>{throw Object.assign(new Error(code),{code});};
  const text=value=>{if(typeof value!=="string")fail("invalid-text");return value;};
  const key=text(action.key),exists=Object.hasOwn(labels,key);
  let entries=Object.entries(labels);
  if(action.kind==="add"){
    if(exists)fail("duplicate-label");
    entries.push([key,""]);
  }else{
    if(!exists)fail("missing-label");
    if(action.kind==="rename"){
      const target=text(action.value);
      if(target!==key&&Object.hasOwn(labels,target))fail("duplicate-label");
      entries=entries.map(([name,value])=>[name===key?target:name,value]);
    }else if(action.kind==="value")entries=entries.map(([name,value])=>[name,name===key?text(action.value):value]);
    else if(action.kind==="remove"){
      if(!confirmed)fail("confirmation-required");
      entries=entries.filter(([name])=>name!==key);
    }else fail("unknown-action");
  }
  const next=parseJSON(stringifyJSON(document));
  // fromEntries creates own data properties even for __proto__/constructor.
  next.metadata.labels=Object.fromEntries(entries);
  return stringifyJSON(next);
}
