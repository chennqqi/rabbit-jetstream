import test from "node:test";
import assert from "node:assert/strict";
import {createListRefresh} from "../src/list-refresh.mjs";

const settle=()=>new Promise(resolve=>setImmediate(resolve));
const query={q:"selected",order:"desc",offset:200,limit:25};
function fixture(resource="queues",requested=query){
  const timers=new Map(),calls=[];let id=0,respond=async()=>({body:{items:[],total:1,offset:1,limit:25}});
  const list=createListRefresh({request:(path,options)=>{calls.push({path,options});return respond();}},resource,requested,{
    schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:key=>timers.delete(key),
  });
  return {...list,timers,calls,setResponse:fn=>respond=fn,async tick(){assert.equal(timers.size,1);const [key,{fn}]=[...timers][0];timers.delete(key);fn();await settle();}};
}
test("automatic Queue/Stream refresh repeats exact requested query, not clamped page offset",async()=>{
  for(const resource of ["queues","streams"]){
    const supplied={...query},f=fixture(resource,supplied);supplied.offset=0;f.refresh.start();await settle();
    assert.equal(f.model.snapshot().page.offset,1);assert.equal(f.model.snapshot().query.offset,200);
    assert.equal([...f.timers.values()][0].ms,10000);await f.tick();
    assert.equal(f.calls.length,2);assert.equal(f.calls[0].path,f.calls[1].path);
    const url=new URL(f.calls[1].path,"http://localhost");assert.equal(url.pathname,`/api/v1/${resource}`);
    for(const [key,value] of Object.entries(query))assert.equal(url.searchParams.get(key),String(value));
    assert.equal(f.calls[1].options.method,undefined);f.clear();assert.equal(f.timers.size,0);
  }
});
test("manual clicks and interval changes cannot overlap a running list read",async()=>{
  const f=fixture();let finish;f.setResponse(()=>new Promise(resolve=>finish=resolve));
  f.refresh.start();await f.refresh.refresh();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
  f.refresh.setInterval(30000);finish({body:{items:[],total:0,offset:0,limit:25}});await settle();
  assert.equal([...f.timers.values()][0].ms,30000);f.clear();
});
test("refresh failure retains rows/time, backs off, and denial clears protected data",async()=>{
  for(const resource of ["queues","streams"]){
    const f=fixture(resource);f.refresh.start();await settle();const old=f.model.snapshot();
    f.setResponse(async()=>{throw {status:503};});await f.tick();
    assert.equal(f.model.snapshot().page,old.page);assert.equal(f.model.snapshot().readAt,old.readAt);
    assert.equal(f.model.snapshot().failure.kind,"unavailable");assert.equal([...f.timers.values()][0].ms,20000);
    f.setResponse(async()=>{throw {status:403};});await f.tick();assert.equal(f.model.snapshot().page,null);assert.equal(f.model.snapshot().readAt,null);
    assert.equal([...f.timers.values()][0].ms,40000);
    f.setResponse(async()=>({body:{items:[],total:0,offset:0,limit:25}}));await f.tick();
    assert.equal(f.model.snapshot().failure,null);assert.equal([...f.timers.values()][0].ms,10000);f.clear();
  }
});
test("query disposal aborts the old read and fences late success or failure",async()=>{
  for(const failed of [false,true]){
    const old=fixture();let finish,reject;old.setResponse(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));old.refresh.start();
    old.clear();assert.equal(old.calls[0].options.signal.aborted,true);
    const current=fixture("queues",{...query,q:"other"});current.refresh.start();await settle();
    if(failed)reject({status:503});else finish({body:{items:[],total:0,offset:0,limit:25}});await settle();
    assert.equal(old.model.snapshot().page,null);assert.equal(old.model.snapshot().phase,"idle");assert.equal(old.timers.size,0);
    assert.equal(current.model.snapshot().query.q,"other");assert.equal(current.model.snapshot().phase,"ready");current.clear();
  }
});
test("hidden list and manual mode do not schedule periodic reads or catch-up",async()=>{
  const f=fixture();f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);
  f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
  f.refresh.visibility(true);await f.refresh.refresh();assert.equal(f.calls.length,1);
  f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);
  await f.refresh.refresh();assert.equal(f.calls.length,2);f.refresh.setInterval(10000);
  f.refresh.visibility(true);assert.equal(f.timers.size,0);f.refresh.visibility(false);
  assert.equal(f.calls.length,2);assert.equal(f.timers.size,1);f.clear();
});
