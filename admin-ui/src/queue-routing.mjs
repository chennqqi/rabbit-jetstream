import {parseJSON,stringifyJSON} from "./api.mjs";
import {supportsQueueFields} from "./queue-fields.mjs";

export function routingMode(document) {
  const spec=document.spec;
  if(spec.subjects?.length&&spec.bindings?.length)return "mixed";
  return spec.bindings?.length||(!spec.subjects?.length&&spec.bindings!==undefined)?"bindings":"subjects";
}
export function routingNeedsConfirmation(document,action) {
  const spec=document.spec;
  if(action.kind==="mode")return action.value!==routingMode(document)&&!!(spec.subjects?.length||spec.bindings?.length);
  if(action.kind==="type")return action.value==="fanout"&&!!spec.bindings?.[action.index]?.keys?.length;
  if(action.kind==="remove-subject")return !!spec.subjects?.[action.index];
  if(action.kind==="remove-binding"){
    const binding=spec.bindings?.[action.index];return !!(binding?.exchange||binding?.type||binding?.keys?.length);
  }
  if(action.kind==="remove-key")return !!spec.bindings?.[action.index]?.keys?.[action.keyIndex];
  return false;
}
export function editQueueRouting(document,action,{confirmed=false}={}) {
  if(!supportsQueueFields(document))throw new Error("Unsupported document; retain expert JSON");
  if(routingNeedsConfirmation(document,action)&&!confirmed)throw new Error("Explicit removal confirmation required");
  const next=parseJSON(stringifyJSON(document)),spec=next.spec,mode=routingMode(document);
  const index=(list,value)=>{if(!Array.isArray(list)||!Number.isInteger(value)||value<0||value>=list.length)throw new Error("Unknown routing row");return value;};
  const text=value=>{if(typeof value!=="string")throw new Error("Expected text");return value;};
  if(action.kind==="mode"){
    if(!["subjects","bindings"].includes(action.value))throw new Error("Unknown routing mode");
    if(action.value===mode)return stringifyJSON(next);
    if(action.value==="subjects"){delete spec.bindings;spec.subjects=mode==="mixed"?spec.subjects:[];}
    else{delete spec.subjects;spec.bindings=mode==="mixed"?spec.bindings:[];}
  }else if(["add-subject","subject","remove-subject"].includes(action.kind)){
    if(mode!=="subjects")throw new Error("Subjects mode required");
    if(action.kind==="add-subject")(spec.subjects??(spec.subjects=[])).push("");
    else if(action.kind==="subject")spec.subjects[index(spec.subjects,action.index)]=text(action.value);
    else spec.subjects.splice(index(spec.subjects,action.index),1);
  }else{
    if(mode!=="bindings")throw new Error("Bindings mode required");
    if(action.kind==="add-binding")(spec.bindings??(spec.bindings=[])).push({exchange:"",type:"",keys:[]});
    else if(action.kind==="remove-binding")spec.bindings.splice(index(spec.bindings,action.index),1);
    else{
      const binding=spec.bindings[index(spec.bindings,action.index)];
      if(action.kind==="exchange")binding.exchange=text(action.value);
      else if(action.kind==="type"){
        if(!["","direct","topic","fanout"].includes(action.value))throw new Error("Unknown binding type");
        binding.type=action.value;if(action.value==="fanout")delete binding.keys;
      }else if(action.kind==="remove-key"){binding.keys.splice(index(binding.keys,action.keyIndex),1);}
      else if(action.kind==="add-key"||action.kind==="key"){
        if(binding.type==="fanout")throw new Error("Fanout cannot have routing keys");
        if(action.kind==="add-key")(binding.keys??(binding.keys=[])).push("");
        else binding.keys[index(binding.keys,action.keyIndex)]=text(action.value);
      }else throw new Error("Unknown routing action");
    }
  }
  return stringifyJSON(next);
}
