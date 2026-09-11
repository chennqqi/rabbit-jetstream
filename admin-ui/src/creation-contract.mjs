import {readMutationCapabilities,observedCapabilityChange} from "./mutation-capabilities.mjs";
import {compileQueueForm} from "./queue-form-schema.mjs";
import {templateQueueDocument} from "./queue-templates.mjs";
import {parseQueueImport} from "./queue-import.mjs";
import {stringifyJSON} from "./api.mjs";

export function createCreationContract(api){
  let state={phase:"idle"},generation=0,unsubscribe;
  const listeners=new Set(),emit=next=>{state=next;for(const listener of listeners)listener();};
  return {
    snapshot:()=>state,
    subscribe(listener){listeners.add(listener);return()=>listeners.delete(listener);},
    dispose(){generation++;unsubscribe?.();unsubscribe=undefined;emit({phase:"idle"});},
    async load(){
      if(state.phase==="loading")return;
      unsubscribe??=api.subscribeCapabilityReads?.(observation=>{
        if(state.phase==="ready"&&observedCapabilityChange(state.binding,observation)){
          generation++;emit({phase:"error"});
        }
      });
      const current=++generation;emit({phase:"loading"});
      try{
        const binding=await readMutationCapabilities(api,["queue-schema","queue-preview","conditional-queue-writes"]);
        const controls=compileQueueForm(binding);
        if(current===generation)emit({phase:"ready",binding,controls});
      }catch(error){if(current===generation)emit({phase:"error",status:error.status});}
    },
    prepare(form){
      if(state.phase!=="ready")throw Object.assign(new Error("Creation schema unavailable"),{code:"schema-unavailable"});
      return templateQueueDocument(form,state.controls);
    },
    prepareImport(document){
      if(state.phase!=="ready")throw Object.assign(new Error("Creation schema unavailable"),{code:"schema-unavailable"});
      const draft=parseQueueImport(stringifyJSON(document));
      const options=key=>state.controls.fields.find(field=>field.key===key)?.options??[];
      if(!Number.isSafeInteger(draft.spec.replicas)||!options("replicas").includes(String(draft.spec.replicas))||draft.spec.storage!==undefined&&!options("storage").includes(draft.spec.storage))throw Object.assign(new Error("Unsupported imported deployment settings"),{code:"import-settings"});
      return draft;
    },
  };
}
