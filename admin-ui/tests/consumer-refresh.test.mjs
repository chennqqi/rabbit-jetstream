import test from "node:test";
import assert from "node:assert/strict";
import {createConsumerRefresh} from "../src/consumer-refresh.mjs";
const body={stream:"S",name:"same",mode:"pull",filter_subjects:[],pending:9007199254740993n};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(stream="S",name="same"){
  const timers=new Map(),calls=[];let id=0,reply=async()=>({body});
  const view=createConsumerRefresh({request:(path,options)=>{calls.push({path,options});return reply();}},stream,name,{schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:id=>timers.delete(id)});
  return {...view,timers,calls,setReply:fn=>reply=fn,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}
test("Consumer refresh retains exact same-identity data on network failure and resets backoff on recovery",async()=>{
  const f=fixture();f.refresh.start();await settle();const before=f.model.snapshot();
  f.setReply(async()=>{throw {status:503};});await f.tick();assert.equal(f.model.snapshot().resource,before.resource);assert.equal(f.model.snapshot().readAt,before.readAt);
  assert.equal(f.model.snapshot().resource.pending,9007199254740993n);assert.equal([...f.timers.values()][0].ms,20000);
  f.setReply(async()=>({body:{...body,mode:"push",pending:0}}));await f.tick();assert.equal(f.model.snapshot().resource.mode,"push");assert.equal(f.model.snapshot().resource.pending,0);assert.equal([...f.timers.values()][0].ms,10000);
  assert.ok(f.calls.every(call=>call.path==="/api/v1/streams/S/consumers/same"&&!call.options.method));f.clear();
});
test("Consumer disappearance, denial, disablement and identity mismatch clear retained metrics",async()=>{
  for(const failure of [{status:404,code:"not_found"},{status:401},{status:403},{status:404,code:"read_api_disabled"},"identity"]){
    const f=fixture();f.refresh.start();await settle();f.setReply(async()=>{if(failure==="identity")return {body:{...body,stream:"other"}};throw failure;});await f.tick();
    assert.equal(f.model.snapshot().resource,null);assert.equal(f.model.snapshot().readAt,null);assert.notEqual(f.model.snapshot().failure,null);f.clear();
  }
});
test("Consumer manual mode, hidden pause and in-flight guard never queue duplicate reads",async()=>{
  const f=fixture();f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
  let finish;f.setReply(()=>new Promise(resolve=>finish=resolve));const pending=f.refresh.refresh();await f.refresh.refresh();assert.equal(f.calls.length,2);
  f.refresh.visibility(true);finish({body});await pending;assert.equal(f.timers.size,0);f.refresh.setInterval(10000);assert.equal(f.timers.size,0);f.refresh.visibility(false);assert.equal(f.calls.length,2);assert.equal(f.timers.size,1);f.clear();
});
test("changing Stream with the same Consumer name discards old response and scheduler",async()=>{
  for(const fail of [false,true]){
    const old=fixture();let finish,reject;old.setReply(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));old.refresh.start();old.clear();assert.equal(old.calls[0].options.signal.aborted,true);
    const next=fixture("other");next.setReply(async()=>({body:{...body,stream:"other",pending:2}}));next.refresh.start();await settle();
    if(fail)reject({status:503});else finish({body});await settle();
    assert.equal(old.model.snapshot().resource,null);assert.equal(old.timers.size,0);assert.equal(next.model.snapshot().resource.stream,"other");assert.equal(next.model.snapshot().resource.pending,2);next.clear();
  }
});
