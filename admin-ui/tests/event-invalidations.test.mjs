import {test} from "node:test";
import assert from "node:assert/strict";
import {runEventInvalidations} from "../src/event-invalidations.mjs";

test("invalidation stream resumes from the last event and stops on abort",async()=>{
  const controller=new AbortController(),calls=[],seen=[];
  const api={async events(options){calls.push(options.lastEventID);return{events:(async function*(){yield{id:calls.length===1?"4":"5",resource:"audit"};})()};}};
  await runEventInvalidations(api,{signal:controller.signal,onInvalidate:resource=>{seen.push(resource);if(seen.length===2)controller.abort();},wait:async()=>{}});
  assert.deepEqual(calls,[undefined,"4"]);assert.deepEqual(seen,["audit","audit"]);
});

test("expired replay resets identity and forces bounded refresh",async()=>{
  const controller=new AbortController(),calls=[],seen=[];
  const api={async events(options){calls.push(options.lastEventID);if(calls.length===1)throw Object.assign(Error("expired"),{status:409});controller.abort();return{events:(async function*(){})()};}};
  await runEventInvalidations(api,{signal:controller.signal,onInvalidate:resource=>seen.push(resource),wait:async()=>{}});
  assert.deepEqual(calls,[undefined,undefined]);assert.deepEqual(seen,["audit","alerts"]);
});
