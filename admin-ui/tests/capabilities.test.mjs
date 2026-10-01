import test from "node:test";
import assert from "node:assert/strict";
import {createCapabilities,validCapabilities} from "../src/capabilities.mjs";
const body=()=>({schemaVersion:"rjs.console-capabilities.v1",deployment:{profile:"unknown",source:"unspecified"},queue:{apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",supportedReplicas:[1,3,5],supportedStorage:["file","memory"],minimumPriority:0,maximumPriority:255,requiresExplicitReplicas:true,defaults:{storage:"file",delivery:{ackWait:"30s",maxDeliver:5}}},features:["queue-preview"],qualification:{status:"unreported"}});
test("capabilities separate declared intent, parser defaults and unreported qualification",()=>{
  assert.equal(validCapabilities(body()),true);
  for(const profile of ["standalone","cluster"])assert.equal(validCapabilities({...body(),deployment:{profile,source:"configuration"}}),true);
  const reported={...body(),qualification:{status:"reported",statement:"local-release-gates-passed; native-linux-soak-and-canary-required",manifestDigest:"a".repeat(64)}};
  assert.equal(validCapabilities(reported),true);
  for(const qualification of [{status:"qualified"},{status:"reported",statement:"",manifestDigest:"a".repeat(64)},{status:"reported",statement:"pending",manifestDigest:"bad"},{status:"unreported",statement:"pending"}])assert.equal(validCapabilities({...body(),qualification}),false);
  for(const patch of [{schemaVersion:"future"},{deployment:{profile:"cluster",source:"observed"}},{features:["duplicate","duplicate"]}])assert.equal(validCapabilities({...body(),...patch}),false);
  for(const patch of [{supportedReplicas:[1,1]},{requiresExplicitReplicas:false},{maximumPriority:-1},{defaults:{storage:"disk",delivery:{ackWait:"30s",maxDeliver:5}}}])assert.equal(validCapabilities({...body(),queue:{...body().queue,...patch}}),false);
});
test("failed refresh removes stale capabilities and retries only explicit reads",async()=>{
  let error=false;const calls=[];
  const model=createCapabilities({async request(path,options){calls.push({path,options});if(error)throw {status:503};return {body:body()};}});
  await model.load();assert.equal(model.snapshot().phase,"ready");error=true;await model.load();assert.equal(model.snapshot().phase,"error");assert.equal(model.snapshot().body,undefined);
  assert.equal(calls.length,2);assert.ok(calls.every(call=>call.path==="/api/v1/console/capabilities"&&!call.options.method));
});
test("late capabilities do not restore cleared or superseded state",async()=>{
  let resolve;const model=createCapabilities({request:()=>new Promise(done=>resolve=done)});
  const pending=model.load();model.clear();resolve({body:body()});await pending;assert.equal(model.snapshot().phase,"idle");
});
