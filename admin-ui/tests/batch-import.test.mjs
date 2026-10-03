import test from "node:test";
import assert from "node:assert/strict";
import {createBatchImport} from "../src/batch-import.mjs";

const binding={revision:'"capability-test"'};
const file=(name,raw)=>{const bytes=new TextEncoder().encode(raw);return {name,size:bytes.length,arrayBuffer:async()=>bytes.buffer};};
const queue=(name="orders")=>file(`${name}.json`,`{"metadata":{"name":"${name}"},"spec":{"retention":{"maxMessages":9223372036854775807}}}`);
const item=(index,queue,dependencies=[],problems=[])=>({index,queue,dependencies,problems,revision:"content-revision"});
const response=(items=[item(0,"orders")],order=[0],external=[])=>({body:{scope:"declaration-only-not-apply-authorization",plan:{items,order,ready:order.length===items.length},external_declarations:external}});
const deferred=()=>{let resolve;const promise=new Promise(done=>{resolve=done;});return {promise,resolve};};

test("batch selection is local, preserves indexes and exact integers, and planning only posts declarations",async()=>{
  const calls=[],model=createBatchImport({request:async(path,options)=>{calls.push({path,options});return response([item(0,"orders",["failed"]),item(1,"failed")],[1,0]);}});
  await model.read([queue(),queue("failed")]);assert.equal(calls.length,0);
  await model.plan(null);assert.equal(calls.length,0);
  await model.plan(binding);assert.equal(calls.length,1);
  const {path,options}=calls[0];assert.equal(path,"/api/v1/queues/import-plan");assert.equal(options.method,"POST");assert.equal(options.headers["X-RJS-If-Capabilities-Match"],binding.revision);
  assert.deepEqual(Object.keys(options.body),["documents"]);assert.equal(options.body.documents[0].spec.retention.maxMessages,9223372036854775807n);
  assert.deepEqual(model.snapshot().result.order,[1,0]);assert.equal(model.snapshot().files[0].name,"orders.json");
});

test("invalid files are retained and block dispatch; bounds fail before reading",async()=>{
  let calls=0,reads=0;const model=createBatchImport({request:()=>{calls++;throw Error("unexpected");}});
  for(const input of [[],Array(101).fill(queue()),[{size:0}],[{size:1048577}],Array(5).fill({size:1048576,arrayBuffer:()=>{reads++;}})]){
    await model.read(input);assert.equal(model.snapshot().failure,"bounds");await model.plan(binding);
  }
  assert.equal(reads,0);
  for(const bad of [file("bad.json","{"),{name:"bad",size:1,arrayBuffer:async()=>new Uint8Array([255]).buffer},{size:2,arrayBuffer:async()=>new Uint8Array([123]).buffer}]){
    await model.read([queue(),bad,queue("last")]);assert.deepEqual(model.snapshot().files.map(row=>row.status),["loaded","invalid","loaded"]);await model.plan(binding);
  }
  assert.equal(calls,0);
});

test("external presence never unblocks an item and invalid declarations keep their input row",async()=>{
  const model=createBatchImport({request:async()=>response([item(0,"orders",["outside"],[{code:"external_dependency_unverified",dependency:"outside"}]),item(1,"",[],[{code:"invalid_declaration"}]),item(2,"independent")],[2],[{queue:"outside",status:"present",etag:'"18446744073709551615"'}])});
  await model.read([queue(),file("invalid.json","null"),queue("independent")]);await model.plan(binding);
  assert.equal(model.snapshot().phase,"ready");assert.equal(model.snapshot().result.ready,false);assert.deepEqual(model.snapshot().result.order,[2]);assert.equal(model.snapshot().result.external[0].etag,'"18446744073709551615"');
});

test("malformed ordering, identity, problem and external evidence clear the whole plan",async()=>{
  const mutations=[body=>body.scope="apply",body=>body.plan.items[0].index=1,body=>body.plan.items[0].queue="other",body=>body.plan.order=[0,0],body=>body.plan.order=[],body=>body.plan.ready=false,body=>body.plan.items[0].dependencies=["missing"],body=>body.plan.items[0].problems=[{code:"unknown"}],body=>body.external_declarations=[{queue:"other",status:"missing"}]];
  for(const mutate of mutations){let value=response();const model=createBatchImport({request:async()=>value});await model.read([queue()]);await model.plan(binding);assert.ok(model.snapshot().result);value=response();mutate(value.body);await model.plan(binding);assert.equal(model.snapshot().failure,"invalid");assert.equal(model.snapshot().result,null);}
  const model=createBatchImport({request:async()=>response([item(0,"orders"),item(1,"orders")],[0,1])});await model.read([queue(),queue()]);await model.plan(binding);assert.equal(model.snapshot().failure,"invalid");
});

test("dependency-first ordering and complete external observations are enforced",async()=>{
  for(const external of [[],[{queue:"outside",status:"present",etag:"1"}],[{queue:"outside",status:"unknown"}],[{queue:"outside",status:"missing"},{queue:"outside",status:"missing"}]]){
    const model=createBatchImport({request:async()=>response([item(0,"orders",["outside"],[{code:"external_dependency_unverified",dependency:"outside"}])],[],external)});await model.read([queue()]);await model.plan(binding);assert.equal(model.snapshot().failure,"invalid");
  }
  const model=createBatchImport({request:async()=>response([item(0,"orders",["failed"]),item(1,"failed")],[0,1])});await model.read([queue(),queue("failed")]);await model.plan(binding);assert.equal(model.snapshot().failure,"invalid");
});

test("clear, file replacement and capability invalidation fence late responses",async()=>{
  for(const action of [model=>model.clear(),model=>model.read([queue("newer")]),model=>model.invalidatePlan()]){
    const gate=deferred();let signal;const model=createBatchImport({request:async(path,options)=>{signal=options.signal;return gate.promise;}});await model.read([queue()]);const pending=model.plan(binding);await action(model);assert.equal(signal.aborted,true);gate.resolve(response());await pending;assert.equal(model.snapshot().result,null);assert.notEqual(model.snapshot().phase,"ready");
  }
  const gate=deferred(),model=createBatchImport({});const pending=model.read([{...queue(),arrayBuffer:()=>gate.promise}]);await model.read([queue("newer")]);gate.resolve(await queue().arrayBuffer());await pending;assert.equal(model.snapshot().files[0].name,"newer.json");
});

test("planning failures are explicit and never retain prior order",async()=>{
  for(const [error,failure] of [[{status:401},"denied"],[{status:403},"denied"],[{status:412},"changed"],[{kind:"invalid-response"},"invalid"],[{status:503},"unavailable"]]){
    let fail=false;const model=createBatchImport({request:async()=>{if(fail)throw error;return response();}});await model.read([queue()]);await model.plan(binding);fail=true;await model.plan(binding);assert.equal(model.snapshot().failure,failure);assert.equal(model.snapshot().result,null);
  }
});
