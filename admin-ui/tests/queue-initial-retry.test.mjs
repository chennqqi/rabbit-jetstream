import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDraft} from "../src/queue-draft.mjs";
const document={metadata:{name:"q"},spec:{max:9007199254740993n}};
const response=()=>({body:{queue:"q",document},headers:new Headers({ETag:'"42"'})});

test("initial failures need explicit retry; successful retry establishes exact new base without writes",async()=>{
  for(const failure of [()=>{throw Object.assign(new Error("unavailable"),{status:503});},()=>({body:{queue:"q",document_error:"unsupported"},headers:new Headers()})]){
    let calls=0,read=failure;
    const model=createQueueDraft({clearToken(){},async request(path,options){calls++;assert.equal(path,"/api/v1/queues/q");assert.equal(options,undefined);return read();}},"q");
    await model.load();await model.load();assert.equal(calls,1);assert.ok(["load-error","uneditable"].includes(model.snapshot().phase));
    await model.retryLoad();assert.equal(calls,2);assert.equal(model.snapshot().draft,undefined);
    read=response;await model.retryLoad();assert.equal(model.snapshot().phase,"editing");assert.equal(model.snapshot().etag,'"42"');
    assert.match(model.snapshot().raw,/9007199254740993/);assert.equal(model.snapshot().modified,false);
    model.edit("{broken");await model.retryLoad();assert.equal(calls,3);assert.equal(model.snapshot().raw,"{broken");
  }
});

test("retry synchronously locks duplicate calls and late completion cannot resurrect a cleared session",async()=>{
  let calls=0,resolve;
  const model=createQueueDraft({clearToken(){},async request(){if(++calls===1)throw new Error("offline");return new Promise(done=>resolve=done);}},"q");
  await model.load();const pending=model.retryLoad();assert.equal(model.snapshot().phase,"loading");
  await model.retryLoad();await model.load();assert.equal(calls,2);
  model.discard();resolve(response());await pending;
  assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().raw,"");
});
