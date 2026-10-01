import test from "node:test";
import assert from "node:assert/strict";
import {connectionSubscriptions,createConnectionSubscriptions,subscriptionPage} from "../src/connection-subscriptions.mjs";

const time="2026-09-10T08:00:00.123456789+08:00",cid="18446744073709551615";
const body=()=>({node_id:"N",cid:BigInt(cid),observed_at:time,read_at:time,items:[{sid:"001",subject:"orders.*",messages:0},{sid:"02",subject:"orders.*",queue:"workers",messages:9223372036854775806n,maximum:9}]});

test("subscription observation preserves opaque SID, duplicate Subjects and exact counters",()=>{
  const result=connectionSubscriptions(body(),"N",cid);
  assert.equal(result.items[0].sid,"001");assert.equal(result.items[1].messages,9223372036854775806n);assert.equal(result.items[0].maximum,undefined);
  assert.equal(subscriptionPage(result,"workers",0,50).items[0].sid,"02");assert.equal(subscriptionPage(result,"orders",1,1).items[0].sid,"02");
});

test("subscription observation rejects partial, reordered and malformed rows",()=>{
  for(const patch of [
    {node_id:"other"},{cid:7},{observed_at:"bad"},{items:Array(1001).fill({sid:"x",subject:"x",messages:0})},
    {items:[{sid:"02",subject:"a",messages:0},{sid:"001",subject:"b",messages:0}]},
    {items:[{sid:"x",subject:"a",messages:-1}]},{items:[{sid:"x\n",subject:"a",messages:0}]},{items:[{sid:"x",subject:"",messages:0}]},
  ])assert.throws(()=>connectionSubscriptions({...body(),...patch},"N",cid));
});

test("explicit subscription load has no automatic read and clears sensitive rows",async()=>{
  const calls=[];let response={body:body()};const model=createConnectionSubscriptions({request:async(path,options)=>{calls.push({path,options});return response;}},"N",cid);
  assert.equal(calls.length,0);await model.load();assert.equal(calls.length,1);assert.equal(calls[0].path,`/api/v1/nodes/N/connections/${cid}/subscriptions`);assert.equal(model.snapshot().observation.items.length,2);
  model.clear();assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().observation,null);
});

test("only transient unavailability retains historical subscription evidence",async()=>{
  let reply={body:body()};const model=createConnectionSubscriptions({request:async()=>{if(reply instanceof Error||reply?.status)throw reply;return reply;}},"N",cid);
  await model.load();const prior=model.snapshot().observation;
  reply={status:503};await model.load();assert.equal(model.snapshot().failure,"unavailable");assert.equal(model.snapshot().observation,prior);
  for(const [error,failure] of [[{status:422,code:"subscription_limit_exceeded"},"limit"],[{status:404,code:"connection_not_found"},"missing"],[{status:403},"denied"],[{kind:"invalid-response"},"invalid"]]){
    reply={body:body()};await model.load();reply=error;await model.load();assert.equal(model.snapshot().failure,failure);assert.equal(model.snapshot().observation,null);
  }
});

test("clearing fences a late subscription response",async()=>{
  let resolve;const model=createConnectionSubscriptions({request:()=>new Promise(done=>resolve=done)},"N",cid);const pending=model.load();model.clear();resolve({body:body()});await pending;assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().observation,null);
});
