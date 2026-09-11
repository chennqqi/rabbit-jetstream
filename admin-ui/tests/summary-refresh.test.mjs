import test from "node:test";
import assert from "node:assert/strict";
import {createSummaryRefresh} from "../src/summary-refresh.mjs";
import {summaryConsumerCounts} from "../src/summary-consumer.mjs";
const plan={stream:{name:"S"},consumer:{stream:"S",name:"primary"},priorityConsumers:[{name:"not-primary"}]};
const stream={name:"S",subjects:[],messages:9007199254740993n};
const consumer={stream:"S",name:"primary",mode:"pull",filter_subjects:[],pending:9,ack_pending:0};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(value=plan){
  const timers=new Map(),calls=[];let id=0,reply=async path=>({body:path.endsWith("/S")?stream:consumer});
  const view=createSummaryRefresh({request:(path,options)=>{calls.push({path,options});return reply(path);}},value,{schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:id=>timers.delete(id)});
  return {...view,timers,calls,reply:fn=>reply=fn,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}
test("summary automatic batch reads only Stream and declared primary; manual source selection is not sticky",async()=>{
  const f=fixture();f.refresh.start();await settle();assert.deepEqual(f.calls.map(v=>v.path),["/api/v1/streams/S","/api/v1/streams/S/consumers/primary"]);
  const priorConsumer=f.consumer.snapshot();await f.refresh.refresh("stream");assert.equal(f.consumer.snapshot(),priorConsumer);assert.equal(f.calls.at(-1).path,"/api/v1/streams/S");
  const priorStream=f.stream.snapshot();await f.refresh.refresh("consumer");assert.equal(f.stream.snapshot(),priorStream);
  await f.tick();assert.deepEqual(f.calls.slice(-2).map(v=>v.path),["/api/v1/streams/S","/api/v1/streams/S/consumers/primary"]);assert.ok(f.calls.every(v=>!v.options.method));f.clear();
});
test("summary batch blocks both manual sources until all current reads finish",async()=>{
  const f=fixture(),pending=[];f.reply(path=>new Promise(resolve=>pending.push(()=>resolve({body:path.endsWith("/S")?stream:consumer}))));
  f.refresh.start();assert.equal(f.calls.length,2);pending[0]();await settle();await f.refresh.refresh("stream");await f.refresh.refresh("consumer");assert.equal(f.calls.length,2);assert.equal(f.timers.size,0);
  pending[1]();await settle();assert.equal(f.timers.size,1);f.clear();
});
test("summary source failures retain only their own historical time while successful siblings advance",async()=>{
  const f=fixture();f.refresh.start();await settle();const old=f.consumer.snapshot(),oldStream=f.stream.snapshot();
  f.reply(async path=>{if(path.endsWith("/primary"))throw {status:503};return {body:stream};});await f.tick();
  assert.equal(f.consumer.snapshot().resource,old.resource);assert.equal(f.consumer.snapshot().readAt,old.readAt);assert.notEqual(f.stream.snapshot(),oldStream);assert.equal([...f.timers.values()][0].ms,20000);
  assert.equal(summaryConsumerCounts(f.scope,f.consumer.snapshot(),{allowHistorical:true}).pending,"9");
  f.reply(async path=>({body:path.endsWith("/S")?stream:consumer}));await f.tick();assert.equal([...f.timers.values()][0].ms,10000);f.clear();
});
test("missing, denied, disabled or invalid summary source clears only that source",async()=>{
  for(const selection of ["stream","consumer"]){
    for(const error of [{status:404,code:"not_found"},{status:401},{status:403},{status:404,code:"read_api_disabled"},"invalid"]){
      const f=fixture();f.refresh.start();await settle();const sibling=selection==="stream"?f.consumer:f.stream,old=sibling.snapshot();
      f.reply(async()=>{if(error==="invalid")return {body:{}};throw error;});await f.refresh.refresh(selection);assert.equal(f[selection].snapshot().resource,null);assert.equal(f[selection].snapshot().readAt,null);assert.equal(sibling.snapshot(),old);f.clear();
    }
  }
});
test("manual/hidden summary scheduling and disposal do not restart or restore late reads",async()=>{
  const f=fixture();f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,2);assert.equal(f.timers.size,0);
  f.refresh.visibility(true);await f.refresh.refresh("consumer");assert.equal(f.calls.length,2);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,2);
  let finish;f.reply(()=>new Promise(resolve=>finish=resolve));const pending=f.refresh.refresh("stream");f.clear();assert.equal(f.calls.at(-1).options.signal.aborted,true);finish({body:stream});await pending;assert.equal(f.stream.snapshot().resource,null);assert.equal(f.consumer.snapshot().resource,null);assert.equal(f.timers.size,0);
});
test("invalid primary scope never selects a priority Consumer or enumerates the collection",async()=>{
  const f=fixture({...plan,consumer:{stream:"other",name:"primary"}});f.refresh.start();await settle();assert.equal(f.scope,null);assert.equal(f.consumer.snapshot().resource,null);assert.deepEqual(f.calls.map(v=>v.path),["/api/v1/streams/S"]);f.clear();
});
