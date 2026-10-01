import test from "node:test";
import assert from "node:assert/strict";
import {createAPI} from "../src/api.mjs";
import {createConsumerRefresh} from "../src/consumer-refresh.mjs";
import {createConsumerCollectionRefresh} from "../src/consumer-collection-refresh.mjs";
import {createSummaryRefresh} from "../src/summary-refresh.mjs";
const settle=()=>new Promise(resolve=>setImmediate(resolve));
const consumer={stream:"S",name:"C",mode:"pull",filter_subjects:[],pending:7};
const page={queue:"Q",stream:"S",declaration_revision:'"1"',stream_status:"present",stream_ownership:"matching",items:[{name:"C",stream:"S",expected:null,observed:consumer,status:"present",ownership:"unknown"}],total:1,offset:0,limit:50};
function fixture(kind){
  let response=null;const requests=[];
  const api=createAPI({origin:"http://localhost",fetch:async(url,options)=>{
    const path=new URL(url).pathname;requests.push({path,method:options.method});
    if(path==="/api/v1/streams/S")return new Response(JSON.stringify({name:"S",subjects:[],messages:9}));
    return response?response():new Response(JSON.stringify(kind==="collection"?page:consumer));
  }});
  const view=kind==="summary"?createSummaryRefresh(api,{stream:{name:"S"},consumer:{stream:"S",name:"C"}}):kind==="collection"?createConsumerCollectionRefresh(api,"queue","Q",{limit:50}):createConsumerRefresh(api,"S","C");
  const model=kind==="summary"?view.consumer:view.model;
  return {view,model,requests,setResponse:value=>response=value,value:()=>kind==="collection"?model.snapshot().page:model.snapshot().resource,refresh:()=>view.refresh.refresh(kind==="summary"?"consumer":undefined)};
}
for(const kind of ["detail","summary","collection"]){
  test(`${kind} clears malformed JSON and time, cannot resurrect old history after subsequent unavailable response`,async()=>{
    for(const raw of ['{"name":','{} trailing','', '[1,]', '9007199254740993e0']){
      const f=fixture(kind);f.view.refresh.setInterval(0);f.view.refresh.start();await settle();assert.equal(f.model.snapshot().phase,"ready");const sibling=kind==="summary"?f.view.stream.snapshot():null;
      f.setResponse(()=>new Response(raw,{status:200}));await f.refresh();assert.equal(f.model.snapshot().failure,"invalid");assert.equal(f.value(),null);assert.equal(f.model.snapshot().readAt,null);
      f.setResponse(()=>new Response('{"error":{"code":"jetstream_unavailable"}}',{status:503}));await f.refresh();assert.equal(f.model.snapshot().failure,"unavailable");assert.equal(f.value(),null);assert.equal(f.model.snapshot().readAt,null);
      if(sibling)assert.equal(f.view.stream.snapshot(),sibling);
      f.setResponse(null);await f.refresh();assert.equal(f.model.snapshot().phase,"ready");assert.ok(f.value());assert.ok(f.requests.every(v=>v.method==="GET"));f.view.clear();
    }
  });
  test(`${kind} preserves HTTP denial classification even with malformed error bodies`,async()=>{
    for(const status of [401,403]){
      const f=fixture(kind);f.view.refresh.setInterval(0);f.view.refresh.start();await settle();f.setResponse(()=>new Response('broken',{status}));await f.refresh();assert.equal(f.model.snapshot().failure,"denied");assert.equal(f.value(),null);assert.equal(f.model.snapshot().readAt,null);f.view.clear();
    }
  });
}
