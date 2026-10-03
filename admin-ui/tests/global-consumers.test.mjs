import {test} from "node:test";
import assert from "node:assert/strict";
import {createGlobalConsumers,decodeGlobalConsumerPage} from "../src/global-consumers.mjs";

const query={q:"",queue:"",stream:"",mode:"",state:"",order:"asc",offset:0,limit:50,generation:""};
const page={state:"ready",generation_id:"g1",started_at:"2026-09-11T00:00:00Z",completed_at:"2026-09-11T00:00:01Z",items:[{queue:"q",stream:"S",name:"C",durable:"C",mode:"pull",status:"present",ownership:"matching",pending:0n,ack_pending:0}],total:1,offset:0,limit:50};

test("global Consumer decoder preserves exact zero and rejects misleading missing counters",()=>{
  const result=decodeGlobalConsumerPage(structuredClone(page),query);assert.equal(result.items[0].pending,0n);assert.equal(result.items[0].ack_pending,0);
  for(const patch of [{pending:-1},{pending:1, status:"missing"},{pending:null,ack_pending:0,status:"missing"},{mode:"other"},{ownership:"unproven"}])assert.throws(()=>decodeGlobalConsumerPage({...page,items:[{...page.items[0],...patch}]},query));
});

test("global Consumer model queries snapshots only and pins returned generation",async()=>{
  const calls=[],api={request:async(path,options={})=>{calls.push({path,options});return {body:structuredClone(page),status:200};}};
  const model=createGlobalConsumers(api);await model.load(query);const state=model.snapshot();assert.equal(state.phase,"ready");assert.equal(state.query.generation,"g1");assert.match(calls[0].path,/^\/api\/v1\/consumers\?/);assert.equal(calls[0].options.method,undefined);
  const refresh=await model.refreshCollection();assert.deepEqual(refresh,{ok:true,busy:false});assert.equal(calls[1].path,"/api/v1/consumers/refresh");assert.equal(calls[1].options.method,"POST");
});

test("global Consumer model distinguishes unavailable, collecting and generation replacement",async()=>{
  for(const [status,body,kind,code] of [[503,{state:"unavailable"},"unavailable"],[503,{state:"collecting"},"collecting"],[409,{error:{code:"consumer_generation_changed"}},"generation-changed","consumer_generation_changed"]]){
    const api={request:async()=>{throw Object.assign(new Error("failed"),{status,body,code});}},model=createGlobalConsumers(api);await model.load(query);assert.equal(model.snapshot().failure.kind,kind);
  }
});

test("global Consumer decoder keeps repeated names on distinct Streams and rejects duplicate canonical identity",()=>{
  const row=page.items[0],other={...row,stream:"OTHER"};assert.equal(decodeGlobalConsumerPage({...page,items:[row,other],total:2},query).items.length,2);
  assert.throws(()=>decodeGlobalConsumerPage({...page,items:[row,{...row}],total:2},query));
});
