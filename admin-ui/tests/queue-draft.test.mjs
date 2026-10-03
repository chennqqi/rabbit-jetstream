import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDraft} from "../src/queue-draft.mjs";
const document={metadata:{name:"q"},spec:{max:9007199254740993n}};
function fixture(){
  const calls=[];let clears=0;
  const api={clearToken(){clears++;},request:async(path,options)=>{
    calls.push({path,options});
    if(path.endsWith("/preview"))return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:false,base_revision:'"9007199254740993"'}};
    return {body:{queue:"q",document},headers:new Headers({ETag:'"9007199254740993"'})};
  }};
  return {model:createQueueDraft(api,"q"),calls,clears:()=>clears};
}
test("canonical draft preserves original ETag and exact integers for preview only",async()=>{
  const {model,calls}=fixture();await model.load();
  assert.match(model.snapshot().raw,/9007199254740993/);
  model.edit('{"metadata":{"name":"q"},"spec":{"max":9007199254740994}}');
  await model.preview();assert.equal(model.snapshot().phase,"review");
  assert.equal(calls[1].options.headers["If-Match"],'"9007199254740993"');
  assert.equal(calls[1].options.body.spec.max,9007199254740994n);
  assert.ok(calls.every(call=>!call.options||call.options.method==="POST"));
});
test("invalid text invalidates preview and cannot issue another preview",async()=>{
  const {model,calls}=fixture();await model.load();await model.preview();
  model.edit("{broken");assert.equal(model.snapshot().preview,undefined);assert.equal(model.snapshot().validation,"invalid-draft");
  await model.preview();assert.equal(calls.length,2);assert.equal(model.snapshot().raw,"{broken");
});
test("identity cannot change; remount load retains draft and original base",async()=>{
  const {model,calls}=fixture();await model.load();
  model.edit('{"metadata":{"name":"other"}}');await model.load();
  assert.equal(model.snapshot().validation,"invalid-draft");assert.equal(model.snapshot().name,"q");assert.equal(calls.length,1);
  assert.equal(model.snapshot().modified,true);
});
test("discard removes draft and credentials",async()=>{
  const f=fixture();await f.model.load();f.model.edit("bad");f.model.discard();
  assert.equal(f.model.snapshot().raw,"");assert.equal(f.model.snapshot().modified,false);assert.equal(f.clears(),1);
});

test("conflict comparison requires explicit merge and fresh preview without a write",async()=>{
  const calls=[];let reads=0;
  const remote={metadata:{name:"q",labels:{remote:"kept"}},spec:{max:123}};
  const model=createQueueDraft({clearToken(){},request:async(path,options)=>{
    calls.push({path,options});
    if(path.endsWith("/preview")){
      if(options.headers["If-Match"]==='"7"')throw Object.assign(new Error("conflict"),{status:409});
      return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:false,base_revision:'"8"'}};
    }
    return {body:{queue:"q",document:++reads===1?document:remote},headers:new Headers({ETag:reads===1?'"7"':'"8"'})};
  }},"q");
  await model.load();model.edit('{"metadata":{"name":"q"},"spec":{"max":456}}');await model.preview();
  await model.readConflict();assert.equal(model.snapshot().etag,'"7"');assert.deepEqual(model.snapshot().base,document);
  assert.deepEqual(model.snapshot().comparison.document,remote);assert.match(model.snapshot().mergeRaw,/456/);
  model.confirmMerge(true);await model.preview();
  assert.equal(model.snapshot().comparison,undefined);assert.equal(model.snapshot().mergeConfirmed,false);
  await model.readConflict();
  model.rebase();assert.equal(model.snapshot().etag,'"7"');
  model.editMerge('{"metadata":{"name":"q","labels":{"remote":"kept"}},"spec":{"max":456}}');
  model.confirmMerge(true);model.editMerge("broken");assert.equal(model.snapshot().mergeConfirmed,false);
  model.confirmMerge(true);model.rebase();assert.equal(model.snapshot().etag,'"7"');
  model.editMerge('{"metadata":{"name":"q","labels":{"remote":"kept"}},"spec":{"max":456}}');model.confirmMerge(true);model.rebase();
  assert.equal(model.snapshot().etag,'"8"');assert.equal(model.snapshot().phase,"editing");assert.equal(model.snapshot().preview,undefined);
  assert.deepEqual(model.snapshot().base,remote);assert.equal(model.snapshot().draft.spec.max,456);
  await model.preview();assert.equal(model.snapshot().phase,"review");assert.ok(calls.every(call=>!call.options||call.options.method==="POST"));
});

test("confirmed apply locks synchronously; lost response stays locked after inspection",async()=>{
  let resolveWrite, writes=0;
  const model=createQueueDraft({clearToken(){},request:async(path,options)=>{
    if(options?.method==="PUT"){writes++;return new Promise(resolve=>resolveWrite=resolve);}
    if(path.endsWith("/preview"))return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:false,base_revision:'"7"'}};
    if(path.includes("/consumers?"))return {body:{queue:"q",items:[],total:0},headers:new Headers()};
    return {body:{queue:"q",document},headers:new Headers({ETag:'"7"'})};
  }},"q");
  await model.load();await model.preview();await model.apply();assert.equal(writes,0);
  model.confirmApply(true);const pending=model.apply();assert.equal(model.snapshot().phase,"submitting");
  assert.throws(()=>model.discard(),/pending/);await model.apply();assert.equal(writes,1);
  resolveWrite({body:{unexpected:true},headers:new Headers()});await pending;assert.equal(model.snapshot().phase,"uncertain");
  await model.editNext();assert.equal(model.snapshot().phase,"uncertain");assert.equal(writes,1);
  const raw=model.snapshot().raw;model.edit("bad");assert.equal(model.snapshot().raw,raw);
  await model.inspect();assert.equal(model.snapshot().phase,"uncertain");assert.equal(writes,1);
  assert.ok(model.snapshot().requestId);assert.ok(model.snapshot().inspection);assert.equal(model.snapshot().etag,'"7"');
});
