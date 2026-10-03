import test from "node:test";
import assert from "node:assert/strict";
import {capabilityFingerprint,capabilityCheckPreventedDispatch,readMutationCapabilities} from "../src/mutation-capabilities.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {createQueueDelete} from "../src/queue-delete.mjs";

const capabilities=()=>({schemaVersion:"rjs.console-capabilities.v1",deployment:{profile:"unknown",source:"unspecified"},queue:{apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",supportedReplicas:[1,3,5],supportedStorage:["file","memory"],minimumPriority:0,maximumPriority:255,requiresExplicitReplicas:true,defaults:{storage:"file",delivery:{ackWait:"30s",maxDeliver:5}}},features:["queue-preview","queue-delete-preview","conditional-queue-writes"],qualification:{status:"unreported"}});
const document={metadata:{name:"orders"},spec:{replicas:1}};
test("contract revision required and build-only changes invalidate binding; 412 after dispatch is not no-write evidence",async()=>{
  const body={...capabilities(),features:[...capabilities().features,"capability-preconditions"]};let revision=`"rjs-capabilities-v1:${"a".repeat(64)}"`;
  const api={request:async()=>({body,headers:new Headers(revision?{ETag:revision}:{})})};
  const original=await readMutationCapabilities(api,[]);revision=`"rjs-capabilities-v1:${"b".repeat(64)}"`;
  await assert.rejects(readMutationCapabilities(api,[],original),{code:"capabilities_changed"});revision=null;
  await assert.rejects(readMutationCapabilities(api,[]),{code:"capabilities_unavailable"});
  const error={status:412,code:"capabilities_changed"};
  assert.equal(capabilityCheckPreventedDispatch({phase:"uncertain",error}),false);
  assert.equal(capabilityCheckPreventedDispatch({phase:"preview-error",error}),true);
});
function fixture(){
  let body=capabilities(),fail=false,afterPreview=false,writeError=false;const calls=[];
  const response=body=>({body,headers:new Headers({ETag:'"7"'})});
  const changed=()=>{body={...body,deployment:{profile:"cluster",source:"configuration"}};};
  const api={requireMutationCapabilities:true,clearToken(){},async request(path,options={}){
    calls.push({path,...options});
    if(path.endsWith("/capabilities")){if(fail)throw {status:503};return {body:{...body,features:[...body.features,"capability-preconditions"]},headers:new Headers({ETag:`"rjs-capabilities-v1:${"a".repeat(64)}"`})};}
    if(path.endsWith("/delete-preview"))return response({queue:"orders",stream:"RJSQ_orders",base_revision:'"7"',observed_at:"2026-09-10T01:00:00Z",stream_present:true,ownership:"matching",messages:0,consumers:1,requires_force:false,blocked:false});
    if(path.endsWith("/preview")){if(afterPreview)changed();return response({plan:{queue:"orders"},result:{queue:"orders",status:"ready",blocked:false,operations:[]},create_only:false,base_revision:'"7"'});}
    if(["PUT","DELETE"].includes(options.method)){if(writeError)throw typeof writeError==="object"?writeError:new Error("lost");return response(options.method==="PUT"?{queue:"orders",status:"ready",blocked:false}:{queue:"orders",stream:"RJSQ_orders",status:"deleted",blocked:false,forced:false,messages:0});}
    return response({queue:"orders",document,plan:{queue:"orders",stream:{name:"RJSQ_orders"}}});
  }};
  return {api,calls,changed,fail:()=>{fail=true;},duringPreview:()=>{afterPreview=true;},loseWrite:(error=true)=>{writeError=error;}};
}
test("412 from a dispatched write keeps unknown state and never claims no write sent",async()=>{
  for(const deletion of [false,true]){
    const f=fixture();f.loseWrite({status:412,code:"capabilities_changed",kind:"http"});
    const model=deletion?createQueueDelete(f.api,"orders"):createQueueDraft(f.api,"orders");
    if(!deletion)await model.load();await model.preview();
    if(deletion){model.type("orders");model.acknowledge(true);await model.submit();}else{model.confirmApply(true);await model.apply();}
    assert.equal(model.snapshot().phase,"uncertain");assert.equal(model.snapshot().error.code,"capabilities_changed");assert.equal(capabilityCheckPreventedDispatch(model.snapshot()),false);
    assert.equal(f.calls.filter(call=>["PUT","DELETE"].includes(call.method)).length,1);
  }
});
test("fingerprint ignores object/set ordering but detects deployment/default/feature changes",()=>{
  const first=capabilities(),second=Object.fromEntries(Object.entries(first).reverse());second.features=[...first.features].reverse();second.queue={...first.queue,supportedReplicas:[5,1,3]};
  assert.equal(capabilityFingerprint(first),capabilityFingerprint(second));
  assert.notEqual(capabilityFingerprint(first),capabilityFingerprint({...first,features:first.features.slice(1)}));
  assert.notEqual(capabilityFingerprint(first),capabilityFingerprint({...first,queue:{...first.queue,defaults:{...first.queue.defaults,delivery:{ackWait:"40s",maxDeliver:5}}}}));
});
test("editor invalidates a preview if capabilities change during observation",async()=>{
  const f=fixture(),model=createQueueDraft(f.api,"orders");await model.load();f.duringPreview();await model.preview();
  assert.equal(model.snapshot().phase,"preview-error");assert.equal(model.snapshot().error.code,"capabilities_changed");assert.equal(model.snapshot().preview,undefined);
  assert.equal(f.calls.filter(c=>c.method==="PUT").length,0);
});
test("editor pre-dispatch checks preserve draft/etag and do not fabricate request evidence",async()=>{
  for(const mode of ["changed","fail"]){
    const f=fixture(),model=createQueueDraft(f.api,"orders");await model.load();await model.preview();model.confirmApply(true);
    const raw=model.snapshot().raw;f[mode]();await model.apply();
    assert.equal(model.snapshot().phase,"preview-error");assert.equal(model.snapshot().applyConfirmed,false);assert.equal(model.snapshot().raw,raw);assert.equal(model.snapshot().etag,'"7"');
    assert.equal(model.snapshot().requestId,undefined);assert.equal(model.snapshot().submittedPlan,undefined);assert.equal(f.calls.filter(c=>c.method==="PUT").length,0);
  }
});
test("deletion check failures clear typed confirmation/force without issuing DELETE",async()=>{
  for(const mode of ["changed","fail"]){const f=fixture(),model=createQueueDelete(f.api,"orders");await model.preview();model.type("orders");model.acknowledge(true);f[mode]();await model.submit();
    assert.equal(model.snapshot().phase,"preview-error");assert.equal(model.snapshot().typed,"");assert.equal(model.snapshot().force,false);assert.equal(model.snapshot().requestId,undefined);assert.equal(model.snapshot().preview,undefined);
    assert.equal(f.calls.filter(c=>c.method==="DELETE").length,0);
  }
});
test("failures after actual dispatch still remain unknown and never auto-retry",async()=>{
  for(const deletion of [false,true]){const f=fixture();f.loseWrite();const model=deletion?createQueueDelete(f.api,"orders"):createQueueDraft(f.api,"orders");
    if(!deletion)await model.load();await model.preview();
    if(deletion){model.type("orders");model.acknowledge(true);await model.submit();await model.submit();}else{model.confirmApply(true);await model.apply();await model.apply();}
    assert.equal(model.snapshot().phase,"uncertain");assert.ok(model.snapshot().requestId);assert.equal(f.calls.filter(c=>["PUT","DELETE"].includes(c.method)).length,1);
    assert.equal(f.calls.find(c=>["PUT","DELETE"].includes(c.method)).headers["X-RJS-If-Capabilities-Match"],`"rjs-capabilities-v1:${"a".repeat(64)}"`);
  }
});
