import test from "node:test";
import assert from "node:assert/strict";
import {createAPI} from "../src/api.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {createQueueDelete} from "../src/queue-delete.mjs";

const contract=()=>({schemaVersion:"rjs.console-capabilities.v1",deployment:{profile:"unknown",source:"unspecified"},queue:{apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",supportedReplicas:[1,3,5],supportedStorage:["file","memory"],minimumPriority:0,maximumPriority:255,requiresExplicitReplicas:true,defaults:{storage:"file",delivery:{ackWait:"30s",maxDeliver:5}}},features:["queue-preview","queue-delete-preview","conditional-queue-writes","capability-preconditions"],qualification:{status:"unreported"}});
const reply=(body,tag='"7"',status=200)=>new Response(JSON.stringify(body),{status,headers:{ETag:tag}});
function fixture(){
  let body=contract(),failure=false,releaseWrite,entered;
  const enteredWrite=new Promise(resolve=>entered=resolve),calls=[];
  const api=createAPI({origin:"http://local.test",requireMutationCapabilities:true,fetch:async(url,options)=>{
    const path=new URL(url).pathname;calls.push({path,method:options.method});
    if(path.endsWith("/capabilities"))return failure?reply({error:{code:"unavailable"}},'"7"',503):reply(body,`"rjs-capabilities-v1:${"a".repeat(64)}"`);
    const name=path.split("/")[4];
    if(options.method==="PUT"){entered();return new Promise(resolve=>releaseWrite=()=>resolve(reply({error:{code:"unavailable"}},'"7"',503)));}
    if(path.endsWith("/delete-preview"))return reply({queue:name,stream:`RJSQ_${name}`,base_revision:'"7"',observed_at:"2026-09-10T01:00:00Z",stream_present:true,ownership:"matching",messages:0,consumers:1,requires_force:false,blocked:false});
    if(path.endsWith("/preview"))return reply({plan:{queue:name},result:{queue:name,status:"ready",blocked:false,operations:[]},create_only:false,base_revision:'"7"'});
    return reply({queue:name,document:{metadata:{name}},plan:{queue:name,stream:{name:`RJSQ_${name}`}}});
  }});
  return {api,calls,change(){body={...body,deployment:{profile:"cluster",source:"configuration"}};},fail(){failure=true;},enteredWrite,releaseWrite:()=>releaseWrite()};
}
test("capability refresh invalidates multiple retained reviews but preserves drafts and original versions",async()=>{
  for(const mode of ["change","fail"]){
    const f=fixture(),one=createQueueDraft(f.api,"one"),two=createQueueDraft(f.api,"two"),deletion=createQueueDelete(f.api,"three");
    for(const model of [one,two]){await model.load();await model.preview();model.confirmApply(true);}
    await deletion.preview();deletion.force(true);deletion.type("three");deletion.acknowledge(true);
    const raws=[one.snapshot().raw,two.snapshot().raw];
    await f.api.request("/api/v1/console/capabilities");assert.equal(one.snapshot().phase,"review");
    f[mode]();await f.api.request("/api/v1/console/capabilities").catch(()=>{});
    for(const [index,model] of [one,two].entries()){assert.equal(model.snapshot().phase,"preview-error");assert.equal(model.snapshot().raw,raws[index]);assert.equal(model.snapshot().etag,'"7"');assert.equal(model.snapshot().applyConfirmed,false);}
    assert.equal(deletion.snapshot().phase,"preview-error");assert.equal(deletion.snapshot().typed,"");assert.equal(deletion.snapshot().force,false);assert.equal(deletion.snapshot().acknowledged,false);
    assert.ok(f.calls.every(call=>!["PUT","DELETE"].includes(call.method)));
  }
});
test("notifications do not rewrite in-flight or unknown writes and archived snapshots",async()=>{
  const f=fixture(),model=createQueueDraft(f.api,"one"),archived=createQueueDraft(f.api,"two");
  await archived.load();await archived.preview();archived.archive();const archivedState=archived.snapshot();
  await model.load();await model.preview();model.confirmApply(true);const pending=model.apply();await f.enteredWrite;
  f.change();await f.api.request("/api/v1/console/capabilities");assert.equal(model.snapshot().phase,"submitting");
  f.releaseWrite();await pending;const unknown=model.snapshot();assert.equal(unknown.phase,"uncertain");
  f.fail();await f.api.request("/api/v1/console/capabilities").catch(()=>{});assert.equal(model.snapshot(),unknown);assert.equal(archived.snapshot(),archivedState);
});
test("old credential responses do not notify a new session; observer errors cannot alter API result",async()=>{
  let finish;const observations=[];
  const api=createAPI({origin:"http://local.test",fetch:()=>new Promise(resolve=>finish=resolve)});
  api.subscribeCapabilityReads(value=>observations.push(value));api.setToken("old");
  const pending=api.request("/api/v1/console/capabilities");api.clearToken();api.setToken("new");finish(reply(contract()));await pending;assert.equal(observations.length,0);
  const stop=api.subscribeCapabilityReads(()=>{throw new Error("observer failure");});
  const current=api.request("/api/v1/console/capabilities");finish(reply(contract()));assert.equal((await current).status,200);assert.equal(observations.length,1);stop();
});
