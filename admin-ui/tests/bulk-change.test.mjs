import test from "node:test";
import assert from "node:assert/strict";
import {createBulkChange,validateChangePackage,validateChangePlan} from "../src/bulk-change.mjs";
import {stringifyJSON} from "../src/api.mjs";

const etag='"9007199254740993"',binding={revision:'"capability"'};
const document=name=>({apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name,labels:{owner:"sensitive"}},spec:{replicas:1,subjects:[`${name}.events`],retention:{maxMessages:9223372036854775807n}}});
const pack=name=>({schema:"rjs.queue-change.v1",etag,document:document(name)});
const file=(name,value)=>{const bytes=new TextEncoder().encode(typeof value==="string"?value:stringifyJSON(value));return {name,size:bytes.length,arrayBuffer:async()=>bytes.buffer};};
const preview=(name,status="ready")=>({plan:{queue:name},result:{queue:name,status,blocked:status==="blocked",operations:[]},observed_at:"2026-09-11T00:00:00Z",base_revision:etag,create_only:false,declaration_review:{status:"available",document:document(name),diff:{queue:name,changes:[]}}});
const response=(items,ready=items.every(item=>item.status==="ready"))=>({body:{scope:"bulk-change-preview-not-apply-authorization",ready,items}});

test("versioned change packages preserve exact Queue values and reject ambiguous envelopes",()=>{
  const value=validateChangePackage(pack("orders"));assert.equal(value.document.spec.retention.maxMessages,9223372036854775807n);assert.equal(value.document.metadata.labels.owner,"sensitive");
  for(const bad of [{...pack("orders"),extra:true},{...pack("orders"),schema:"v2"},{...pack("orders"),etag:'"01"'},{...pack("orders"),document:{...document("orders"),kind:"Other"}}])assert.throws(()=>validateChangePackage(bad));
});

test("bulk planning posts only documents and original ETags then retains itemized partial results",async()=>{
  const calls=[],api={request:async(path,options)=>{calls.push({path,options});return response([{index:0,queue:"orders",etag,status:"ready",preview:preview("orders")},{index:1,queue:"failed",etag,status:"conflict",code:"conflict"}]);}},model=createBulkChange(api);
  await model.read([file("orders.json",pack("orders")),file("failed.json",pack("failed"))]);assert.equal(calls.length,0);await model.plan(binding);assert.equal(calls.length,1);assert.equal(calls[0].path,"/api/v1/queues/change-plan");assert.equal(calls[0].options.method,"POST");assert.equal(calls[0].options.headers["X-RJS-If-Capabilities-Match"],binding.revision);assert.deepEqual(Object.keys(calls[0].options.body),["items"]);assert.equal(calls[0].options.body.items[0].document.spec.retention.maxMessages,9223372036854775807n);assert.equal(model.snapshot().result.ready,false);assert.deepEqual(model.snapshot().result.items.map(item=>item.status),["ready","conflict"]);
});

test("malformed itemized evidence fails closed without retaining an earlier plan",async()=>{
  const packages=[{...pack("orders"),filename:"orders.json",status:"loaded"}],good=response([{index:0,queue:"orders",etag,status:"ready",preview:preview("orders")}]).body;
  assert.equal(validateChangePlan(good,packages).ready,true);
  for(const mutate of [body=>body.scope="apply",body=>body.ready=false,body=>body.items[0].index=1,body=>body.items[0].queue="other",body=>body.items[0].etag='"2"',body=>body.items[0].preview.create_only=true,body=>body.items[0].preview.result.blocked=true,body=>body.items[0].preview.declaration_review.document.metadata.name="other"]){const body=structuredClone(good);mutate(body);assert.throws(()=>validateChangePlan(body,packages));}
});

test("invalid files and bounds block requests; clearing fences late planning",async()=>{
  let calls=0;const model=createBulkChange({request:async()=>{calls++;throw Error("unexpected");}});await model.read([file("bad.json","{")]);await model.plan(binding);assert.equal(model.snapshot().failure,"files");assert.equal(calls,0);
  await model.read([]);assert.equal(model.snapshot().failure,"bounds");
  let resolve,signal;const pendingModel=createBulkChange({request:async(_path,options)=>{signal=options.signal;return new Promise(done=>{resolve=done;});}});await pendingModel.read([file("orders.json",pack("orders"))]);const pending=pendingModel.plan(binding);pendingModel.clear();assert.equal(signal.aborted,true);resolve(response([{index:0,queue:"orders",etag,status:"ready",preview:preview("orders")} ]));await pending;assert.equal(pendingModel.snapshot().phase,"idle");
});
