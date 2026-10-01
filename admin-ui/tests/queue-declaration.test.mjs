import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDeclaration} from "../src/queue-declaration.mjs";
const body={queue:"Q",revision:"content-hash",plan:{queue:"Q",revision:"content-hash",stream:{name:"S"}}};
const response=(etag='"7"')=>({body,headers:new Headers({ETag:etag})});

test("Declaration refresh is read-only and retains exact ETag separately from Plan revision",async()=>{
  const calls=[],api={request:async(path,options)=>{calls.push({path,method:options.method});return response();}};
  const model=createQueueDeclaration(api,"Q");await model.load();await model.load();
  assert.deepEqual(calls,[{path:"/api/v1/queues/Q",method:undefined},{path:"/api/v1/queues/Q",method:undefined}]);
  assert.equal(model.snapshot().etag,'"7"');assert.equal(model.snapshot().body.revision,"content-hash");assert.ok(model.snapshot().readAt);
});
test("Declaration refresh removes previous evidence immediately and failure cannot advance its time",async()=>{
  const api={request:async()=>response()},model=createQueueDeclaration(api,"Q");await model.load();
  let reject;api.request=()=>new Promise((_,fail)=>{reject=fail;});const pending=model.load();
  assert.deepEqual(model.snapshot(),{phase:"loading"});reject(Object.assign(new Error(),{status:503,code:"unavailable"}));await pending;
  assert.deepEqual(model.snapshot(),{phase:"error",status:503,code:"unavailable"});
});
test("Overlapping declaration reads and clear suppress obsolete success",async()=>{
  const finishes=[],model=createQueueDeclaration({request:()=>new Promise(resolve=>finishes.push(resolve))},"Q");
  const first=model.load(),second=model.load();finishes[1](response('"8"'));await second;finishes[0](response());await first;
  assert.equal(model.snapshot().etag,'"8"');const third=model.load();model.clear();finishes[2](response());await third;assert.deepEqual(model.snapshot(),{phase:"idle"});
});
test("Declaration identity and Plan revision must agree before panels can mount",async()=>{
  for(const invalid of [{...body,queue:"other"},{...body,plan:{...body.plan,queue:"other"}},{...body,plan:{...body.plan,revision:"other"}},{...body,plan:null}]){
    const model=createQueueDeclaration({request:async()=>({...response(),body:invalid})},"Q");await model.load();assert.equal(model.snapshot().phase,"error");assert.equal(model.snapshot().body,undefined);
  }
});
