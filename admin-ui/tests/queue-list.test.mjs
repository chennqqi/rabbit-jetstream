import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueList,queueDeployment,queueObservation} from "../src/queue-list.mjs";

function apiFixture() {
  const calls = [];
  const items = Array.from({length: 201}, (_, i) => ({queue: `queue-${String(i).padStart(3,"0")}`, revision: `plan-${i}`}));
  return {calls, request: async path => {
    const q = new URL(path, "http://localhost").searchParams;
    calls.push(q);
    let filtered = items.filter(item => item.queue.includes(q.get("q")));
    if (q.get("order") === "desc") filtered.reverse();
    const offset = Math.min(Number(q.get("offset")), filtered.length), limit = Number(q.get("limit"));
    return {body: {items: filtered.slice(offset, offset + limit), total: filtered.length, offset, limit}};
  }};
}

test("Queue deployment columns require a consistent declaration Plan",()=>{
  const item={queue:"orders",revision:"plan-1",plan:{queue:"orders",revision:"plan-1",stream:{storage:"file",replicas:3}}};
  assert.deepEqual(queueDeployment(item),{storage:"file",replicas:3});
  for(const changed of [
    {...item,plan:{...item.plan,queue:"other"}},
    {...item,plan:{...item.plan,revision:"plan-2"}},
    {...item,plan:{...item.plan,stream:{storage:"disk",replicas:3}}},
    {...item,plan:{...item.plan,stream:{storage:"file",replicas:0}}},
    {...item,plan:{...item.plan,stream:{storage:"file",replicas:3n}}},
    {...item,plan:null},
  ])assert.equal(queueDeployment(changed),null);
});

test("Queue observations preserve explicit zeros and reject misleading partial evidence",()=>{
  const present={state:"present",stream:"RJSQ_orders",messages:0n,consumers:0,reasons:[]};
  assert.deepEqual(queueObservation({observation:present}),present);
  assert.equal(queueObservation({}),null);
  for(const observation of [
    {state:"healthy",stream:"RJSQ_orders",messages:0n,consumers:0,reasons:[]},
    {state:"present",stream:"RJSQ_orders",consumers:0,reasons:[]},
    {state:"missing",stream:"RJSQ_orders",messages:0n,reasons:["stream_missing"]},
    {state:"unavailable",stream:"RJSQ_orders",reasons:["private-error"]},
    {state:"degraded",stream:"RJSQ_orders",messages:-1,consumers:0,reasons:["stream_configuration_mismatch"]},
    {state:"degraded",stream:"RJSQ_orders",messages:1n,consumers:0,reasons:["stream_configuration_mismatch","stream_configuration_mismatch"]},
  ])assert.throws(()=>queueObservation({observation}));
});

test("server query searches beyond current page, resets offset and preserves total", async () => {
  const api = apiFixture(), model = createQueueList(api);
  await model.load({offset: 150});
  assert.equal(model.snapshot().page.items[0].queue, "queue-150");
  await model.load({q: "queue-200"});
  assert.equal(api.calls[1].get("offset"), "0");
  assert.equal(model.snapshot().page.total, 1);
  assert.equal(model.snapshot().page.items[0].queue, "queue-200");
  await model.load({q: "", order: "desc", limit: 25});
  assert.equal(model.snapshot().page.items[0].queue, "queue-200");
  assert.equal(model.snapshot().page.total, 201);
  assert.equal(model.snapshot().page.items.length, 25);
});

test("empty filtered result differs from page clamped after shrink", async () => {
  const model = createQueueList(apiFixture());
  await model.load({offset: 1000});
  assert.equal(model.snapshot().page.offset, 201);
  assert.equal(model.snapshot().page.total, 201);
  assert.deepEqual(model.snapshot().page.items, []);
  await model.load({q: "missing"});
  assert.equal(model.snapshot().page.total, 0);
  assert.equal(model.snapshot().page.offset, 0);
});

test("same-query failure retains prior page and time without becoming an empty success", async () => {
  const api = apiFixture(), model = createQueueList(api);
  await model.load();
  const previous=model.snapshot();
  api.request = async () => { throw Object.assign(new Error("sensitive backend detail"), {status: 503}); };
  await model.load();
  assert.equal(model.snapshot().phase, "error");
  assert.equal(model.snapshot().page, previous.page);
  assert.equal(model.snapshot().readAt, previous.readAt);
  assert.equal(model.snapshot().failure.kind, "unavailable");
  assert.equal(JSON.stringify(model.snapshot()).includes("sensitive"), false);
});

test("changed query or denied refresh never retains previous rows",async()=>{
  for(const changes of [{q:"other"},{order:"desc"},{offset:50},{limit:25}]){
    const api=apiFixture(),model=createQueueList(api);await model.load();
    api.request=async()=>{throw {status:503};};await model.load(changes);
    assert.equal(model.snapshot().page,null);assert.equal(model.snapshot().readAt,null);
  }
  for(const status of [400,401,403]){
    const api=apiFixture(),model=createQueueList(api);await model.load();
    api.request=async()=>{throw {status};};await model.load();assert.equal(model.snapshot().page,null);
  }
});

test("in-flight refresh retains same-query rows; clear fences late failure",async()=>{
  const api=apiFixture(),model=createQueueList(api);await model.load();const previous=model.snapshot();
  let reject;api.request=()=>new Promise((resolve,no)=>reject=no);
  const pending=model.load();assert.equal(model.snapshot().phase,"loading");assert.equal(model.snapshot().page,previous.page);
  model.clear();reject({status:503});await pending;assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().page,null);
});

test("Stream refresh retains exact counters but changed query cannot reuse them",async()=>{
  let fail=false;const model=createQueueList({request:async()=>{if(fail)throw {status:503};return {body:{items:[{name:"stream",messages:9007199254740993n}],total:1,offset:0,limit:50}};}},"streams");
  await model.load();const time=model.snapshot().readAt;fail=true;await model.load();
  assert.equal(model.snapshot().page.items[0].messages,9007199254740993n);assert.equal(model.snapshot().readAt,time);
  await model.load({q:"different"});assert.equal(model.snapshot().page,null);
});

test("late success/failure cannot replace a newer query or cleared session", async () => {
  for (const fail of [false, true]) {
    let resolve, reject;
    const api = {request: () => new Promise((yes, no) => { resolve = yes; reject = no; })};
    const model = createQueueList(api);
    const pending = model.load();
    model.clear();
    if (fail) reject(new Error("late")); else resolve({body: {items: [], total: 0, offset: 0, limit: 50}});
    await pending;
    assert.equal(model.snapshot().phase, "idle");
    assert.equal(model.snapshot().page, null);
  }
});

test("invalid envelopes and duplicate identities fail closed", async () => {
  const valid = {items: [], total: 0, offset: 0, limit: 50};
  for (const page of [null, {...valid, items: null}, {...valid, total: 1}, {...valid, offset: 1}, {...valid, limit: 200}, {...valid, total: 2, items: [{queue: "a", revision: "x"}, {queue: "a", revision: "x"}]}]) {
    const model = createQueueList({request: async () => ({body: page})});
    await model.load();
    assert.equal(model.snapshot().failure.kind, "invalid-response");
    assert.equal(model.snapshot().page, null);
  }
});

test("new query wins when the old transport ignores abort", async () => {
  let resolve;
  let count = 0;
  const api = {request: () => ++count === 1 ? new Promise(done => { resolve = done; }) :
    Promise.resolve({body: {items: [{queue: "new", revision: "revision-new"}], total: 1, offset: 0, limit: 50}})};
  const model = createQueueList(api);
  const old = model.load({q: "old"});
  await model.load({q: "new"});
  resolve({body: {items: [{queue: "old", revision: "revision-old"}], total: 1, offset: 0, limit: 50}});
  await old;
  assert.equal(model.snapshot().query.q, "new");
  assert.equal(model.snapshot().page.items[0].queue, "new");
});

test("history restoration keeps an explicit offset when restoring filters", async () => {
  const model = createQueueList(apiFixture());
  await model.load({q: "queue-", order: "desc", offset: 50, limit: 25}, {restore: true});
  assert.equal(model.snapshot().query.offset, 50);
  assert.equal(model.snapshot().page.items[0].queue, "queue-150");
});

test("clamped Queue and Stream responses cannot rewrite refresh query after regrowth",async()=>{
  for(const resource of ["queues","streams"]){
    let total=201;const offsets=[];
    const model=createQueueList({request:async path=>{
      const offset=Number(new URL(path,"http://localhost").searchParams.get("offset"));offsets.push(offset);
      const actual=Math.min(offset,total),items=Array.from({length:Math.min(50,total-actual)},(_,i)=>resource==="queues"?{queue:`q-${actual+i}`,revision:"r"}:{name:`s-${actual+i}`});
      return {body:{items,total,offset:actual,limit:50}};
    }},resource);
    await model.load({offset:1000});assert.equal(model.snapshot().query.offset,1000);assert.equal(model.snapshot().page.offset,201);
    total=1050;await model.load();assert.deepEqual(offsets,[1000,1000]);assert.equal(model.snapshot().page.offset,1000);assert.equal(model.snapshot().page.items.length,50);
    await model.load({offset:950});assert.equal(offsets.at(-1),950);
  }
});
