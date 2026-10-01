import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDraft} from "../src/queue-draft.mjs";

const document={metadata:{name:"q"},spec:{max:9007199254740993n}};
function fixture({create=false}={}){
  const calls=[];let latest={body:{queue:"q",document},headers:new Headers({ETag:'"7"'})};
  const model=createQueueDraft({clearToken(){},async request(path,options){
    calls.push({path,options});
    if(options?.method==="PUT")return {body:{queue:"q",status:"ready",blocked:false},headers:new Headers()};
    if(path.endsWith("/preview"))return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:create,base_revision:options.headers["If-Match"]}};
    return typeof latest==="function"?latest():latest;
  }},"q",create?{document}:undefined);
  return {model,calls,setRead(value){latest=value;}};
}
async function accept(model){await model.load();await model.preview();model.confirmApply(true);await model.apply();assert.equal(model.snapshot().phase,"accepted");}

test("next edit reads a new base, retains receipts and requires a new preview and confirmation",async()=>{
  const f=fixture();await accept(f.model);const receipt=f.model.snapshot();
  const remote={metadata:{name:"q",labels:{concurrent:"kept"}},spec:{max:9007199254740994n}};
  f.setRead({body:{queue:"q",document:remote},headers:new Headers({ETag:'"19"'})});
  await f.model.editNext();const next=f.model.snapshot();
  assert.equal(next.phase,"editing");assert.equal(next.etag,'"19"');assert.deepEqual(next.base,remote);
  assert.equal(next.modified,false);assert.equal(next.applyConfirmed,false);assert.equal(next.preview,undefined);assert.equal(next.requestId,undefined);
  assert.match(next.raw,/9007199254740994/);assert.equal(next.acceptedOperations[0].requestId,receipt.requestId);
  assert.equal(next.acceptedOperations[0].returnedETag,null);assert.equal(next.acceptedOperations[0].originalETag,'"7"');
  await f.model.apply();assert.equal(f.calls.filter(c=>c.options?.method==="PUT").length,1);
  await f.model.preview();await f.model.apply();assert.equal(f.calls.filter(c=>c.options?.method==="PUT").length,1);
  f.model.confirmApply(true);await f.model.apply();
  assert.equal(f.calls.at(-1).options.headers["If-Match"],'"19"');assert.notEqual(f.model.snapshot().requestId,receipt.requestId);
  await f.model.editNext();assert.equal(f.model.snapshot().acceptedOperations.length,2);
});

test("failed or malformed next reads retain accepted evidence and never unlock old drafts",async()=>{
  const f=fixture();await accept(f.model);const accepted=f.model.snapshot();
  for(const response of [()=>{throw Object.assign(new Error("unavailable"),{status:503});},
    {body:{queue:"q",document},headers:new Headers()},
    {body:{queue:"other",document},headers:new Headers({ETag:'"8"'})},
    {body:{queue:"q",document:{metadata:{name:"other"}}},headers:new Headers({ETag:'"8"'})}]){
    f.setRead(response);await f.model.editNext();const state=f.model.snapshot();
    assert.equal(state.phase,"accepted");assert.ok(state.nextEditError);
    assert.equal(state.requestId,accepted.requestId);assert.equal(state.raw,accepted.raw);assert.equal(state.etag,accepted.etag);
    f.model.edit("bad");await f.model.preview();await f.model.apply();assert.equal(f.model.snapshot().raw,accepted.raw);
  }
  assert.equal(f.calls.filter(c=>c.options?.method==="PUT").length,1);
});

test("next read locks synchronously and discard suppresses late completion",async()=>{
  const f=fixture();await accept(f.model);let resolve;
  f.setRead(()=>new Promise(done=>{resolve=done;}));
  const pending=f.model.editNext();assert.equal(f.model.snapshot().phase,"reading-next");
  const count=f.calls.length;await f.model.editNext();await f.model.apply();await f.model.load();assert.equal(f.calls.length,count);
  f.model.discard();resolve({body:{queue:"q",document},headers:new Headers({ETag:'"8"'})});await pending;
  assert.equal(f.model.snapshot().phase,"idle");assert.equal(f.model.snapshot().raw,"");assert.equal(f.model.snapshot().acceptedOperations,undefined);
});

test("creation acceptance cannot convert to overwrite through next edit",async()=>{
  const f=fixture({create:true});await accept(f.model);const count=f.calls.length;
  await f.model.editNext();assert.equal(f.calls.length,count);assert.equal(f.model.snapshot().create,true);
  assert.equal(f.model.snapshot().phase,"accepted");
});
