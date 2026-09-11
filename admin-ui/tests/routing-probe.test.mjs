import test from "node:test";
import assert from "node:assert/strict";
import {createRoutingProbe,routingProbePath} from "../src/routing-probe.mjs";
import {createAPI} from "../src/api.mjs";

const body={queue:"orders",revision:"rev",stream:"RJSQ_orders",subject:"orders.created",streamSubjects:["orders.*"],matchedStreamSubjects:["orders.*"],bindings:[]};
const response=value=>({body:value,headers:new Headers({ETag:'"9007199254740993"'})});
function fixture(){let reply=async()=>response(structuredClone(body));const calls=[];
  const model=createRoutingProbe({request:(path,options)=>{calls.push({path,options});return reply();}},"orders","rev",'"9007199254740993"');
  return {model,calls,set(fn){reply=fn;},run:()=>model.run({subject:"orders.created"})};
}

test("routing query preserves literal values, bounds bytes, rejects mixed modes",()=>{
  assert.equal(routingProbePath("orders",{exchange:"events",type:"topic",routingKey:""}),"/api/v1/queues/orders/routing-probe?exchange=events&type=topic&routingKey=");
  for(const input of [null,[],{}, {subject:""},{subject:"a",exchange:""},{subject:"a",unknown:"x"},{subject:12},{type:"topic"},{exchange:"e",type:"headers"},{subject:"界".repeat(342)}])assert.throws(()=>routingProbePath("orders",input));
  assert.throws(()=>routingProbePath("../other",{subject:"a"}));
});

test("routing probe is manual, read-only, projected, exact-revision bound",async()=>{
  const f=fixture();assert.equal(f.calls.length,0);await f.run();
  assert.equal(f.model.snapshot().phase,"ready");assert.deepEqual(f.model.snapshot().result,body);
  assert.equal(f.calls[0].options.method,undefined);assert.equal(f.calls[0].options.body,undefined);
  f.set(async()=>response({...body,revision:"other"}));await f.run();assert.equal(f.model.snapshot().failure,"changed");assert.equal(f.model.snapshot().result,null);
  f.set(async()=>({...response(body),headers:new Headers({ETag:'"9007199254740994"'})}));await f.run();assert.equal(f.model.snapshot().failure,"changed");
});

test("routing decoder rejects inconsistent projections and handles fanout without keys",async()=>{
  const f=fixture();
  for(const value of [null,{}, {...body,queue:"other"},{...body,subject:"wrong"},{...body,stream:""},{...body,streamSubjects:null},{...body,matchedStreamSubjects:["not-in-stream"]},{...body,bindings:[{exchange:"e",type:"headers",keys:[],subjects:["a"],matchedSubjects:[]}]}]){
    f.set(async()=>response(value));await f.run();assert.equal(f.model.snapshot().failure,"invalid");assert.equal(f.model.snapshot().result,null);
  }
  f.set(async()=>response({...body,secret:"hidden",bindings:[{exchange:"e",type:"fanout",keys:null,subjects:["a"],matchedSubjects:[],secret:"hidden"}]}));await f.run();
  assert.equal(f.model.snapshot().result.secret,undefined);assert.deepEqual(f.model.snapshot().result.bindings[0].keys,[]);assert.equal(f.model.snapshot().result.bindings[0].secret,undefined);
});

test("routing errors never retain previous matches, distinguish unavailable from nonmatch",async()=>{
  for(const [error,want] of [[{status:401},"denied"],[{status:403,kind:"invalid-response"},"denied"],[{status:404,code:"not_found"},"missing"],[{status:404,code:"read_api_disabled"},"disabled"],[{status:409,code:"routing_declaration_unavailable"},"changed"],[{status:400},"query"],[{status:503,code:"routing_probe_limit"},"limit"],[{status:503},"unavailable"],[{kind:"invalid-response"},"invalid"]]){
    const f=fixture();await f.run();f.set(async()=>{throw error;});await f.run();assert.equal(f.model.snapshot().failure,want);assert.equal(f.model.snapshot().result,null);assert.equal(f.model.snapshot().readAt,null);
    f.set(async()=>response({...body,matchedStreamSubjects:[]}));await f.run();assert.equal(f.model.snapshot().phase,"ready");assert.deepEqual(f.model.snapshot().result.matchedStreamSubjects,[]);
  }
});

test("actual routing decoder requires recognized errors before reporting missing or changed",async()=>{
  let status=200,raw=JSON.stringify(body);
  const api=createAPI({origin:"http://localhost",fetch:async()=>new Response(raw,{status,headers:{ETag:'"9007199254740993"'}})});
  const model=createRoutingProbe(api,"orders","rev",'"9007199254740993"');
  const run=()=>model.run({subject:"orders.created"});
  for(const [nextStatus,nextRaw,want] of [
    [404,'{"error":{"code":"proxy_not_found"}}',"invalid"],
    [404,'{"error":"not found"}',"invalid"],
    [404,'<html>not found</html>',"invalid"],
    [409,'{"error":{"code":"proxy_conflict"}}',"invalid"],
    [409,'{"error":',"invalid"],
    [400,'<html>bad request</html>',"invalid"],
    [403,'<html>denied</html>',"denied"],
    [404,'{"error":{"code":"not_found"}}',"missing"],
    [404,'{"error":{"code":"read_api_disabled"}}',"disabled"],
    [409,'{"error":{"code":"routing_declaration_unavailable"}}',"changed"],
    [503,'{"error":{"code":"routing_probe_limit"}}',"limit"],
  ]){
    status=200;raw=JSON.stringify(body);await run();assert.equal(model.snapshot().phase,"ready");
    status=nextStatus;raw=nextRaw;await run();assert.equal(model.snapshot().failure,want,`${status}: ${raw}`);
    assert.equal(model.snapshot().result,null);assert.equal(model.snapshot().readAt,null);
    status=503;raw='{"error":{"code":"unavailable"}}';await run();assert.equal(model.snapshot().result,null);
  }
  status=200;raw=JSON.stringify(body);await run();assert.equal(model.snapshot().phase,"ready");
});

test("cancel, input edit, and replacement fence transport that ignores abort",async()=>{
  const f=fixture();let resolve;
  f.set(()=>new Promise(done=>{resolve=done;}));const old=f.run();assert.equal(f.model.snapshot().phase,"loading");
  f.model.clear();assert.equal(f.calls[0].options.signal.aborted,true);resolve(response(body));await old;assert.equal(f.model.snapshot().phase,"idle");
  const late=f.run();f.set(async()=>response({...body,matchedStreamSubjects:[]}));await f.run();resolve(response(body));await late;
  assert.deepEqual(f.model.snapshot().result.matchedStreamSubjects,[]);
});
