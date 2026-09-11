import test from "node:test";
import assert from "node:assert/strict";
import {createQueueDelete} from "../src/queue-delete.mjs";
import {deleteEvidence} from "../src/delete-evidence.mjs";
import {parseJSON,stringifyJSON} from "../src/api.mjs";

const windowFor=id=>({requestId:id,items:[],streamPresent:true,firstSequence:1,lastSequence:9007199254741999n,scanned:256,missing:0,nextBefore:9007199254741744n});
async function fixture(accepted=false){
  let mode="ok",resolve;const calls=[];
  const response=body=>({body,headers:new Headers({ETag:'"7"'})});
  let requestId;
  const api={async request(path,options={}){
    calls.push({path,...options});
    if(options.method==="DELETE"){
      requestId=options.headers["X-Request-ID"];
      if(!accepted)throw new Error("lost response");
      return response({queue:"orders",stream:"RJSQ_orders",status:"deleted",blocked:false,forced:false,messages:0});
    }
    if(path.endsWith("/delete-preview"))return response({queue:"orders",stream:"RJSQ_orders",base_revision:'"7"',observed_at:"2026-09-10T01:00:00Z",stream_present:true,ownership:"matching",messages:0,consumers:3,requires_force:false,blocked:false});
    if(path.includes("/audit/requests/")){
      if(!path.includes("?before="))return response(windowFor(requestId));
      if(mode==="failure")throw {kind:"network",message:"do not export this"};
      if(mode==="pending")return new Promise(done=>{resolve=done;});
      if(mode==="invalid")return response(windowFor(requestId));
      return response({...windowFor(requestId),scanned:2,nextBefore:null,items:[{id:"old",requestId,sequence:2,phase:"outcome"}]});
    }
    if(path.includes("/streams/"))return response({name:"RJSQ_orders",messages:0});
    return response({queue:"orders",plan:{queue:"orders",stream:{name:"RJSQ_orders"}}});
  }};
  const model=createQueueDelete(api,"orders");await model.preview();model.type("orders");model.acknowledge(true);await model.submit();await model.inspect();
  return {model,calls,setMode:value=>{mode=value;},resolve:()=>resolve(response({...windowFor(requestId),scanned:0,nextBefore:null})),response};
}

test("deletion audit walks an empty window with exact uint64 cursor and retains failures",async()=>{
  for(const accepted of [false,true]){
    const {model,calls,setMode}=await fixture(accepted),phase=accepted?"accepted":"uncertain";
    assert.equal(model.snapshot().inspection.audit.windows.length,1);
    for(const mode of ["failure","invalid"]){
      setMode(mode);await model.inspectOlderAudit();assert.equal(model.snapshot().phase,phase);
      assert.equal(model.snapshot().inspection.audit.windows.length,1);assert.ok(model.snapshot().inspection.audit.olderError);
      assert.match(calls.at(-1).path,/before=9007199254741744$/);
    }
    setMode("ok");await model.inspectOlderAudit();assert.equal(model.snapshot().inspection.audit.windows.length,2);
    assert.equal(model.snapshot().inspection.audit.olderError,undefined);assert.equal(model.snapshot().phase,phase);
    const count=calls.length;await model.inspectOlderAudit();await model.submit();assert.equal(calls.length,count);
    assert.equal(model.canSubmit(),false);
    await model.inspect();assert.equal(model.snapshot().inspectionHistory.length,1);
    assert.equal(model.snapshot().inspectionHistory[0].audit.windows.length,2);
    assert.equal(model.snapshot().inspection.audit.windows.length,1);
    assert.equal(calls.filter(c=>c.method==="DELETE").length,1);
  }
});

test("pending older deletion audit is single flight; discard suppresses late completion",async()=>{
  const {model,calls,setMode,resolve}=await fixture();setMode("pending");
  const pending=model.inspectOlderAudit(),count=calls.length;
  await model.inspectOlderAudit();await model.inspect();await model.submit();assert.equal(calls.length,count);
  assert.equal(model.snapshot().phase,"inspecting");assert.equal(model.snapshot().returnPhase,"uncertain");
  model.discard();resolve();await pending;assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().inspection,undefined);
});

test("deletion evidence preserves exact windows/history but excludes session and raw failures",async()=>{
  const {model,calls,setMode}=await fixture();setMode("failure");await model.inspectOlderAudit();await model.inspect();
  const state=model.snapshot(),before=stringifyJSON(state),count=calls.length;
  const raw=deleteEvidence({...state,token:"secret",session:{token:"secret"},headers:{Authorization:"secret"},typed:"secret",
    error:{...state.error,body:"secret",raw:"secret",message:"secret"}},"2026-09-10T02:00:00Z");
  assert.ok(!raw.includes("secret"));assert.ok(!raw.includes("do not export this"));
  const data=parseJSON(raw);assert.equal(data.schema,"rjs.queue-delete-evidence.v1");assert.equal(data.originalETag,'"7"');
  assert.equal(data.inspection.audit.windows[0].body.nextBefore,9007199254741744n);
  assert.equal(data.inspectionHistory[0].audit.olderError.kind,"network");assert.equal(data.requestId,state.requestId);
  assert.equal(stringifyJSON(state),before);assert.equal(calls.length,count);assert.equal(model.canSubmit(),false);
});
