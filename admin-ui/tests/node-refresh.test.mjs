import test from "node:test";
import assert from "node:assert/strict";
import {createNodeRefresh} from "../src/node-refresh.mjs";

const settle=()=>new Promise(resolve=>setImmediate(resolve));
const observation=(status="available",id="N1")=>({body:{total:1,nodes:[{endpoint:"http://node",status,...(id?{id}:{})}]}});
function fixture(){
  const timers=new Map(),calls=[];let key=0,respond=async()=>observation();
  const view=createNodeRefresh({request:(path,options)=>{calls.push({path,options});return respond();}},
    {schedule:(fn,ms)=>{timers.set(++key,{fn,ms});return key;},unschedule:id=>timers.delete(id)});
  return {...view,timers,calls,setResponse:fn=>respond=fn,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}
test("node partial/unavailable observations replace old identities and back off without a top-level error",async()=>{
  const f=fixture();f.refresh.start();await settle();assert.equal([...f.timers.values()][0].ms,10000);
  f.setResponse(async()=>observation("degraded"));await f.tick();assert.equal(f.model.snapshot().failure,null);assert.equal([...f.timers.values()][0].ms,20000);
  f.setResponse(async()=>observation("unavailable",null));await f.tick();assert.equal(f.model.snapshot().snapshot.nodes[0].id,undefined);assert.equal([...f.timers.values()][0].ms,40000);
  await f.tick();assert.equal([...f.timers.values()][0].ms,60000);
  f.setResponse(async()=>observation("available","N2"));await f.tick();assert.equal(f.model.snapshot().snapshot.nodes[0].id,"N2");assert.equal([...f.timers.values()][0].ms,10000);
  assert.ok(f.calls.every(call=>call.path==="/api/v1/nodes"&&call.options.method===undefined));f.clear();
});
test("node network failure preserves the old snapshot/time; authorization denial clears it",async()=>{
  for(const status of [401,403]){
    const f=fixture();f.refresh.start();await settle();const before=f.model.snapshot();
    f.setResponse(async()=>{throw {status:503};});await f.tick();assert.equal(f.model.snapshot().snapshot,before.snapshot);assert.equal(f.model.snapshot().readAt,before.readAt);
    assert.equal(f.model.snapshot().failure,"unavailable");
    f.setResponse(async()=>{throw {status};});await f.tick();assert.equal(f.model.snapshot().snapshot,null);assert.equal(f.model.snapshot().readAt,null);assert.equal(f.model.snapshot().failure,"denied");f.clear();
  }
});
test("node refresh remains single flight and disposal prevents late route data or timers",async()=>{
  for(const fail of [false,true]){
    const f=fixture();let finish,reject;f.setResponse(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));f.refresh.start();
    await f.refresh.refresh();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);f.clear();assert.equal(f.calls[0].options.signal.aborted,true);
    if(fail)reject({status:503});else finish(observation());await settle();assert.equal(f.model.snapshot().snapshot,null);assert.equal(f.model.snapshot().phase,"idle");assert.equal(f.timers.size,0);
  }
});
test("node manual preference and hidden state stop scheduling but preserve explicit visible reads",async()=>{
  const f=fixture();f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);
  f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
  f.refresh.visibility(true);await f.refresh.refresh();assert.equal(f.calls.length,1);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);
  await f.refresh.refresh();assert.equal(f.calls.length,2);f.refresh.setInterval(30000);assert.equal([...f.timers.values()][0].ms,30000);
  f.refresh.visibility(true);assert.equal(f.timers.size,0);f.refresh.visibility(false);assert.equal(f.calls.length,2);assert.equal(f.timers.size,1);f.clear();
});
