import test from "node:test";
import assert from "node:assert/strict";
import {createDLQDiagnostics,dlqTarget} from "../src/dlq-diagnostics.mjs";
import {createAPI} from "../src/api.mjs";
import {dlqEvidence} from "../src/dlq-evidence.mjs";
const plan={queue:"orders",deadLetter:{queue:"failed",stream:"RJSQ_failed",mechanism:"advisory-republish"}};
function fixture(){
  const calls=[];let errorPath,error;
  const api={request:async(path,options)=>{calls.push({path,options});if(path===errorPath)throw error;return {headers:new Headers({ETag:'"42"'}),body:path.endsWith("/controller")?{instanceId:"controller-1",enabled:true,leader:false,dlqMoved:9007199254740993n}:path.includes("/streams/")?{name:"RJSQ_failed"}:{queue:"failed",revision:"r",plan:{queue:"failed",revision:"r",stream:{name:"RJSQ_failed"},deadLetter:{queue:"another"}}}};}};
  return {api,calls,fail(path,value){errorPath=path;error=value;}};
}
test("DLQ reads one target hop and reports exact process-wide counters",async()=>{
  const f=fixture(),model=createDLQDiagnostics(f.api,plan);await model.load();
  assert.equal(f.calls.length,3);assert.ok(f.calls.every(call=>!call.options.method));
  assert.equal(model.snapshot().target.etag,'"42"');assert.equal(model.snapshot().stream.phase,"available");
  assert.equal(model.snapshot().controller.value.dlqMoved,9007199254740993n);assert.equal(model.snapshot().controller.value.dlqFailed,undefined);
  assert.equal(model.snapshot().controller.value.dlqIgnored,undefined);
});
test("missing, denied and unavailable targets do not invent missing Streams",async()=>{
  for(const [error,phase] of [[{status:404,code:"not_found"},"missing"],[{status:403},"denied"],[{status:503},"unavailable"]]){
    const f=fixture();f.fail("/api/v1/queues/failed",error);const model=createDLQDiagnostics(f.api,plan);await model.load();
    assert.equal(model.snapshot().target.phase,phase);assert.equal(model.snapshot().stream.phase,"unobserved");assert.equal(model.snapshot().controller.phase,"available");assert.equal(f.calls.length,2);
  }
});
test("controller failure clears old counters without discarding target evidence",async()=>{
  const f=fixture(),model=createDLQDiagnostics(f.api,plan);await model.load();f.fail("/api/v1/controller",{status:503});await model.load();
  assert.equal(model.snapshot().controller.phase,"unavailable");assert.equal(model.snapshot().controller.value,undefined);assert.equal(model.snapshot().stream.phase,"available");
});
test("unconfigured or invalid DLQ never follows arbitrary links",async()=>{
  assert.equal(dlqTarget({queue:"orders"}),null);const f=fixture();await createDLQDiagnostics(f.api,{queue:"orders"}).load();assert.equal(f.calls.length,0);
  for(const patch of [{queue:"../bad"},{queue:"orders"},{stream:"http://external"},{mechanism:"future"}])assert.throws(()=>dlqTarget({...plan,deadLetter:{...plan.deadLetter,...patch}}));
});
test("clearing a pending declaration prevents chained Stream reads",async()=>{
  const calls=[],pending=[];const model=createDLQDiagnostics({request:(path)=>{calls.push(path);return new Promise(resolve=>pending.push(resolve));}},plan);
  const done=model.load();model.clear();pending.forEach(resolve=>resolve({body:{}}));await done;
  assert.equal(calls.length,2);assert.equal(model.snapshot().target.phase,"unobserved");assert.equal(model.snapshot().controller.phase,"unobserved");
});
test("mismatched target identity does not authorize a Stream read",async()=>{
  const f=fixture(),original=f.api.request;f.api.request=async(path,options)=>{const result=await original(path,options);if(path==="/api/v1/queues/failed")result.body.plan.stream.name="unrelated";return result;};
  const model=createDLQDiagnostics(f.api,plan);await model.load();assert.equal(model.snapshot().target.phase,"invalid");assert.equal(f.calls.length,2);
});

test("controller timestamps and error presence are evidence, never copied private error text",async()=>{
  const f=fixture(),original=f.api.request;f.api.request=async(path,options)=>{const result=await original(path,options);if(path.endsWith("/controller"))Object.assign(result.body,{lastRun:"2026-09-10T00:00:00Z",lastSuccess:"0001-01-01T00:00:00Z",lastError:"private-detail"});return result;};
  const model=createDLQDiagnostics(f.api,plan);await model.load();const value=model.snapshot().controller.value;
  assert.equal(value.lastRun,"2026-09-10T00:00:00Z");assert.equal(value.lastSuccess,undefined);assert.equal(value.errorReported,true);assert.equal(value.lastError,undefined);
});

test("ignored advisory count preserves zero and exact integers; malformed values clear controller evidence",async()=>{
  for(const ignored of [0,9,9007199254740993n,-1,1.5,Number.MAX_SAFE_INTEGER+1,"0",null]){
    const f=fixture(),original=f.api.request,model=createDLQDiagnostics(f.api,plan);await model.load();
    f.api.request=async(path,options)=>{const result=await original(path,options);if(path.endsWith("/controller"))result.body.dlqIgnored=ignored;return result;};
    await model.load();const state=model.snapshot();
    if(typeof ignored==="bigint"||Number.isSafeInteger(ignored)&&ignored>=0)assert.equal(state.controller.value.dlqIgnored,ignored);
    else{assert.equal(state.controller.phase,"invalid");assert.equal(state.controller.value,undefined);}
    assert.equal(state.target.phase,"available");
  }
});

test("missing target does not finish an independent pending controller read",async()=>{
  let finishController;const calls=[];
  const model=createDLQDiagnostics({request:async path=>{
    calls.push(path);
    if(path.endsWith("/controller"))return new Promise(resolve=>{finishController=resolve;});
    throw {status:404,code:"not_found"};
  }},plan);
  const done=model.load();await new Promise(resolve=>setImmediate(resolve));
  assert.equal(model.snapshot().target.phase,"missing");assert.equal(model.snapshot().stream.phase,"unobserved");
  assert.equal(model.snapshot().controller.phase,"loading");assert.equal(calls.length,2);
  finishController({body:{instanceId:"C",enabled:false,leader:false}});await done;
  assert.equal(model.snapshot().controller.phase,"available");
  assert.ok(Object.values(model.snapshot()).every(source=>source.phase!=="loading"));
});

test("real transport deadline settles a stalled controller and allows a fresh DLQ read",async()=>{
  let stall=true,aborts=0;
  const transport=createAPI({origin:"http://localhost",fetch:async(url,{signal})=>{
    if(url.endsWith("/controller")){
      if(stall)return new Promise((resolve,reject)=>signal.addEventListener("abort",()=>{aborts++;reject(signal.reason);},{once:true}));
      return new Response(JSON.stringify({instanceId:"C",enabled:false,leader:false}));
    }
    return new Response('{"error":{"code":"not_found"}}',{status:404});
  }});
  const model=createDLQDiagnostics({request:(path,options)=>transport.request(path,{...options,timeout:25})},plan);
  await model.load();assert.equal(aborts,1);
  assert.equal(model.snapshot().target.phase,"missing");assert.equal(model.snapshot().stream.phase,"unobserved");
  assert.equal(model.snapshot().controller.phase,"unavailable");
  assert.ok(Object.values(model.snapshot()).every(source=>source.phase!=="loading"));
  stall=false;await model.load();assert.equal(model.snapshot().controller.phase,"available");model.clear();
});

test("superseded controller completion cannot overwrite a newer DLQ observation",async()=>{
  const pending=[];
  const model=createDLQDiagnostics({request:async path=>{
    if(path.endsWith("/controller"))return new Promise((resolve,reject)=>pending.push({resolve,reject}));
    throw {status:404,code:"not_found"};
  }},plan);
  const first=model.load(),second=model.load();
  pending[1].resolve({body:{instanceId:"new",enabled:false,leader:false}});await second;
  pending[0].reject(new Error("late failure"));await first;
  assert.equal(model.snapshot().controller.value.instanceId,"new");
  const third=model.load();model.clear();
  pending[2].resolve({body:{instanceId:"after-clear",enabled:true,leader:true}});await third;
  assert.ok(Object.values(model.snapshot()).every(source=>source.phase==="unobserved"));
});

test("invalid controller calendar dates cannot enter DLQ evidence and recovery preserves precision",async()=>{
  for(const field of ["lastRun","lastSuccess"]){
    for(const value of ["2026-02-30T00:00:00Z","2025-02-29T00:00:00Z","2026-09-10T24:00:00Z","2026-09-10T00:00:00","2026-09-10T00:00:00.1234567890Z","0001-01-01T00:00:00.1Z",null]){
      const f=fixture(),original=f.api.request,model=createDLQDiagnostics(f.api,plan);let time=value;
      f.api.request=async(path,options)=>{const result=await original(path,options);if(path.endsWith("/controller"))result.body[field]=time;return result;};
      await model.load();assert.equal(model.snapshot().controller.phase,"invalid",`${field}: ${value}`);
      let exported=JSON.parse(dlqEvidence({plan,state:model.snapshot()}));assert.equal(exported.controller.value,undefined);assert.equal(exported.target.phase,"available");assert.equal(exported.stream.phase,"available");
      time="2026-09-10T08:00:00.123456789+08:00";await model.load();
      exported=JSON.parse(dlqEvidence({plan,state:model.snapshot()}));assert.equal(exported.controller.value[field],time);
      for(const zero of ["0001-01-01T00:00:00Z","0001-01-01T00:00:00.000000000Z"]){time=zero;await model.load();assert.equal(model.snapshot().controller.phase,"available");assert.equal(model.snapshot().controller.value[field],undefined);}
      model.clear();
    }
  }
});

test("DLQ real decoder failures are incompatible, while authorization retains precedence",async()=>{
  for(const endpoint of ["/api/v1/queues/failed","/api/v1/streams/RJSQ_failed","/api/v1/controller"]){
    const f=fixture();let malformed=false,status=200;
    const transport=createAPI({origin:"http://localhost",fetch:async url=>{
      const path=new URL(url).pathname;
      if(malformed&&path===endpoint)return new Response('{"',{status});
      const reply=await f.api.request(path,{});
      // This fixture's controller has one exact bigint counter.
      return new Response(JSON.stringify(reply.body,(_key,value)=>typeof value==="bigint"?1:value));
    }});
    const model=createDLQDiagnostics(transport,plan),key=endpoint.endsWith("/controller")?"controller":endpoint.includes("/streams/")?"stream":"target";
    await model.load();assert.equal(model.snapshot()[key].phase,"available");malformed=true;
    await model.load();assert.equal(model.snapshot()[key].phase,"invalid");assert.equal(model.snapshot()[key].value,undefined);assert.equal(model.snapshot()[key].readAt,null);
    status=403;await model.load();assert.equal(model.snapshot()[key].phase,"denied");
    malformed=false;await model.load();assert.equal(model.snapshot()[key].phase,"available");model.clear();
  }
});
