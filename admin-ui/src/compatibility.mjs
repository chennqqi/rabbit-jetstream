import {latestRead} from "./api.mjs";
const text=value=>typeof value==="string"&&value.length>0&&value.length<=512;
export function buildMetadata(value){
  if(value?.schemaVersion!=="rjs.build-info.v1"||![value.version,value.goVersion,value.os,value.arch].every(text)||
    value.revision!==undefined&&!text(value.revision)||value.modified!==undefined&&typeof value.modified!=="boolean"||
    value.revisionSource!==undefined&&!['release-build','injected','go-build-info'].includes(value.revisionSource)||value.revisionSource!==undefined&&value.revision===undefined||
    value.uiAssets!==undefined&&(value.uiAssets?.algorithm!=="sha256-framed-files-v1"||!/^[0-9a-f]{64}$/.test(value.uiAssets.digest)||!Number.isSafeInteger(value.uiAssets.fileCount)||value.uiAssets.fileCount<1))throw Error("invalid");
  return {schemaVersion:value.schemaVersion,version:value.version,goVersion:value.goVersion,os:value.os,arch:value.arch,revision:value.revision,revisionSource:value.revisionSource,modified:value.modified,uiAssets:value.uiAssets&&{algorithm:value.uiAssets.algorithm,digest:value.uiAssets.digest,fileCount:value.uiAssets.fileCount}};
}
export function sdkMetadata(value){
  if(value?.schema!=="rabbit-jetstream.io/native-sdk-contract/v1alpha1"||!text(value.availability)||
    !value.delivery||![value.delivery.publisher_confirm,value.delivery.delivery_guarantee,value.delivery.ack_policy].every(text)||
    !Array.isArray(value.message_headers)||value.message_headers.length>64||value.message_headers.some(h=>!h||!text(h.name)||typeof h.required!=="boolean"))throw Error("invalid");
  return {schema:value.schema,availability:value.availability,delivery:{publisher_confirm:value.delivery.publisher_confirm,delivery_guarantee:value.delivery.delivery_guarantee,ack_policy:value.delivery.ack_policy},headers:value.message_headers.map(h=>({name:h.name,required:h.required}))};
}
export function createCompatibility(api){
  const sources={build:["/api/v1/console/build",buildMetadata],sdk:["/api/v1/native-sdk-contract.json",sdkMetadata]};
  const readers={build:latestRead(api),sdk:latestRead(api)},listeners=new Set(),empty=()=>({phase:"idle",value:null,readAt:null});
  let state={build:empty(),sdk:empty()};const emit=(key,value)=>{state={...state,[key]:value};for(const fn of listeners)fn();};
  async function read(key){
    const reader=readers[key];reader.cancel();emit(key,{...empty(),phase:"loading"});
    try{const result=await reader.run(sources[key][0]);if(result.stale)return;const value=sources[key][1](result.result.body);emit(key,{phase:"ready",value,readAt:new Date().toISOString()});}
    catch{emit(key,{...empty(),phase:"error"});}
  }
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},load:()=>Promise.all(Object.keys(sources).map(read)),clear(){for(const key of Object.keys(sources)){readers[key].cancel();emit(key,empty());}}};
}
