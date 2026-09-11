import test from "node:test";
import assert from "node:assert/strict";
import {createRefreshLoop} from "../src/refresh-loop.mjs";

const settle=async()=>{await Promise.resolve();await Promise.resolve();};
function fixture(read){
  const timers=new Map();let id=0;
  const loop=createRefreshLoop(read,{schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:key=>timers.delete(key)});
  return {loop,timers,async tick(){assert.equal(timers.size,1);const [key,timer]=[...timers][0];timers.delete(key);timer.fn();await settle();}};
}
test("refresh is single flight, completion based, backed off and reset on success",async()=>{
  let finish,calls=0;
  const {loop,timers,tick}=fixture(()=>{calls++;return new Promise(resolve=>finish=resolve);});
  loop.start();assert.equal(calls,1);await loop.refresh();assert.equal(calls,1);assert.equal(timers.size,0);
  for(const delay of [20000,40000,60000,60000]){finish(false);await settle();assert.equal([...timers.values()][0].ms,delay);await tick();}
  finish(true);await settle();assert.equal([...timers.values()][0].ms,10000);loop.stop();assert.equal(timers.size,0);
});
test("hidden tabs pause without catch-up bursts and stop fences late completion",async()=>{
  let calls=0,finish;const {loop,timers,tick}=fixture(()=>{calls++;return new Promise(resolve=>finish=resolve);});
  loop.start(true);assert.equal(calls,0);loop.visibility(false);assert.equal(timers.size,1);await tick();
  loop.visibility(true);finish(true);await settle();assert.equal(timers.size,0);await loop.refresh();assert.equal(calls,1);
  loop.visibility(false);await tick();loop.stop();finish(false);await settle();assert.equal(timers.size,0);
});
test("thrown reads back off and explicit refresh replaces the pending timer",async()=>{
  let calls=0;const {loop,timers}=fixture(async()=>{calls++;throw new Error("offline");});
  loop.start();await settle();assert.equal([...timers.values()][0].ms,20000);
  await loop.refresh();assert.equal(calls,2);assert.equal(timers.size,1);assert.equal([...timers.values()][0].ms,40000);loop.stop();
});

test("manual preference removes timers without cancelling a batch and still allows explicit reads",async()=>{
  let finish,calls=0;const {loop,timers}=fixture(()=>{calls++;return new Promise(resolve=>finish=resolve);});
  loop.start();loop.setInterval(0);finish(true);await settle();assert.equal(timers.size,0);
  const reading=loop.refresh();assert.equal(calls,2);loop.setInterval(30000);assert.equal(timers.size,0);
  finish(true);await reading;assert.equal([...timers.values()][0].ms,30000);
  assert.equal(loop.setInterval(-1),false);assert.equal([...timers.values()][0].ms,30000);
  loop.setInterval(0);assert.equal(timers.size,0);loop.stop();
});
test("manual mode entered while hidden reads once when first shown",async()=>{
  let calls=0;const {loop,timers}=fixture(async()=>{calls++;return true;});
  loop.setInterval(0);loop.start(true);assert.equal(calls,0);loop.visibility(false);await settle();assert.equal(calls,1);
  loop.visibility(true);loop.visibility(false);await settle();assert.equal(calls,1);assert.equal(timers.size,0);loop.stop();
});
