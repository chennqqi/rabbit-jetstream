import {parseJSON,stringifyJSON} from "./api.mjs";

export const queueImportLimit=1024*1024;
// This is a document-envelope check, not a replacement for server Schema and
// topology validation. Unknown spec fields remain intact for preview rejection.
export function parseQueueImport(text){
  if(typeof text!=="string"||new TextEncoder().encode(text).length>queueImportLimit)throw Error("size");
  const value=parseJSON(text);
  const object=v=>v!==null&&typeof v==="object"&&!Array.isArray(v);
  if(!object(value)||value.apiVersion!=="rabbit-jetstream.io/v1alpha1"||value.kind!=="Queue"||Object.keys(value).some(key=>!["apiVersion","kind","metadata","spec"].includes(key))||!object(value.metadata)||typeof value.metadata.name!=="string"||!/^[A-Za-z0-9_-]{1,256}$/.test(value.metadata.name)||!object(value.spec))throw Error("document");
  if(Object.keys(value.metadata).some(key=>!["name","labels"].includes(key))||value.metadata.labels!==undefined&&(!object(value.metadata.labels)||Object.values(value.metadata.labels).some(label=>typeof label!=="string")))throw Error("document");
  return value;
}

export function createQueueImport(){
  let generation=0,state={phase:"idle",document:null};const listeners=new Set();
  const emit=next=>{state=next;for(const listener of listeners)listener();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},clear(){generation++;emit({phase:"idle",document:null});},
    async read(file){
      const current=++generation;emit({phase:"loading",document:null});
      try{
        if(!file||!Number.isSafeInteger(file.size)||file.size<1||file.size>queueImportLimit)throw Error("size");
        const bytes=await file.arrayBuffer();if(current!==generation)return;
        if(!(bytes instanceof ArrayBuffer)||bytes.byteLength!==file.size||bytes.byteLength>queueImportLimit)throw Error("size");
        const document=parseQueueImport(new TextDecoder("utf-8",{fatal:true}).decode(bytes));
        emit({phase:"ready",document,raw:stringifyJSON(document),bytes:bytes.byteLength,filename:typeof file.name==="string"?file.name:""});
      }catch(error){if(current===generation)emit({phase:"error",document:null,failure:error.message==="size"?"size":"invalid"});}
    }
  };
}
