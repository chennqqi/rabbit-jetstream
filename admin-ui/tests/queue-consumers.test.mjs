import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueConsumers, consumerCounter, validateConsumerPage} from "../src/queue-consumers.mjs";

const query = {q:"",mode:"",order:"asc",offset:0,limit:50};
function row(name, observed = true, expected = true) {
  return {stream:"S",name,expected:expected ? {stream:"S",name,mode:"pull",filterSubjects:["orders.*"]} : null,
    observed:observed ? {stream:"S",name,mode:"pull",filter_subjects:["orders.*"],pending:9007199254740993n,ack_pending:0} : null,
    status:observed ? "present" : "missing",ownership:expected && observed ? "matching" : "unknown"};
}
function page(items) {return {queue:"q",stream:"S",declaration_revision:'"9007199254740993"',stream_status:"present",stream_ownership:"matching",items,total:items.length,offset:0,limit:50};}

test("collection preserves missing/expected/external and exact per-Consumer metrics", async () => {
  const value=page([row("primary"),row("priority",false),row("external",true,false)]);
  const model=createQueueConsumers({request:async()=>({body:value})},"q");
  await model.load();
  assert.equal(model.snapshot().phase,"ready");
  assert.equal(consumerCounter(value.items[0],"pending"),"9007199254740993");
  assert.equal(consumerCounter(value.items[0],"ack_pending"),"0");
  assert.equal(consumerCounter(value.items[1],"pending"),null);
  assert.equal(value.items[2].expected,null);
  assert.equal(model.snapshot().page.declaration_revision,'"9007199254740993"');
});

test("filters are server queries and reset paging beyond 200 resources", async () => {
  const calls=[];
  const model=createQueueConsumers({request:async path=>{
    const params=new URL(path,"http://localhost").searchParams;calls.push(params);
    const offset=Number(params.get("offset"));
    return {body:{...page(params.get("q") ? [row("consumer-200")] : [row("consumer-200")]),total:params.get("q") ? 1 : 201,offset,limit:50}};
  }},"q");
  await model.load({offset:200});
  assert.equal(model.snapshot().page.total,201);
  await model.load({q:"consumer-200",mode:"pull"});
  assert.equal(calls[1].get("offset"),"0");
  assert.equal(calls[1].get("mode"),"pull");
  assert.equal(model.snapshot().page.total,1);
});

test("inconsistent identities, missing fields, partial pages cannot render", () => {
  const valid=page([row("a")]);
  for(const bad of [{...valid,queue:"other"},{...valid,total:2},{...valid,items:[{...row("a"),stream:"other"}]},{...valid,items:[{...row("a"),observed:null}]},{...valid,items:[{...row("a"),expected:undefined}]},{...valid,stream_status:"missing"},{...valid,items:[{...row("a"),observed:{...row("a").observed,filter_subjects:"bad"}}]}]) assert.throws(()=>validateConsumerPage(bad,"q",query));
});

test("conflict and failure are not empty collections or fresh observations", async () => {
  for(const [status,kind] of [[409,"changed"],[503,"unavailable"],[401,"denied"],[400,"query"]]) {
    const model=createQueueConsumers({request:async()=>{throw Object.assign(new Error("private"),{status});}},"q");
    await model.load();
    assert.equal(model.snapshot().failure,kind);assert.equal(model.snapshot().page,null);assert.equal(model.snapshot().readAt,null);
  }
});

test("clear suppresses late Consumer results", async () => {
  let resolve;
  const model=createQueueConsumers({request:()=>new Promise(done=>resolve=done)},"q");
  const pending=model.load();model.clear();resolve({body:page([row("late")])});await pending;
  assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().page,null);
});

test("clamped Consumer page preserves requested offset across refresh and regrowth",async()=>{
  let total=1;const offsets=[];
  const model=createQueueConsumers({request:async path=>{
    const offset=Number(new URL(path,"http://localhost").searchParams.get("offset"));offsets.push(offset);
    const actual=Math.min(offset,total),items=Array.from({length:Math.min(50,total-actual)},(_,i)=>row(`c-${actual+i}`));
    return {body:{...page(items),total,offset:actual}};
  }},"q");
  await model.load({offset:200});assert.equal(model.snapshot().query.offset,200);assert.equal(model.snapshot().page.offset,1);
  total=201;await model.load();assert.deepEqual(offsets,[200,200]);assert.equal(model.snapshot().page.items[0].name,"c-200");
  await model.load({q:"c",mode:"pull"});assert.equal(offsets.at(-1),0);
});
