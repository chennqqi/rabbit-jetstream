import {test} from "node:test";
import assert from "node:assert/strict";
import {newQueueDocument} from "../src/queue-create.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
const form={name:"new",subjects:"orders.events",replicas:"1",storage:"file",maxMessages:"9223372036854775807"};
test("creation requires explicit deployment choices and exact integer limits",()=>{
  const document=newQueueDocument(form);assert.equal(document.spec.retention.maxMessages,9223372036854775807n);assert.equal(document.metadata.name,"new");
  for(const patch of [{name:"a/b"},{replicas:""},{replicas:"2"},{replicas:"4"},{storage:""},{maxMessages:"0"},{maxMessages:"9223372036854775808"},{subjects:"a\na"}])assert.throws(()=>newQueueDocument({...form,...patch}));
});
test("new draft never reads existing declaration and always previews/applies create-only",async()=>{
  const calls=[];
  const document=newQueueDocument(form);
  const model=createQueueDraft({clearToken(){},request:async(path,options)=>{
    calls.push({path,options});assert.equal(options.headers["If-None-Match"],"*");assert.equal(options.headers["If-Match"],undefined);
    return {body:options.method==="POST"?{plan:{queue:"new"},result:{queue:"new",status:"ready",blocked:false, operations: []},create_only:true,base_revision:""}:{queue:"new",status:"ready",blocked:false},headers:new Headers()};
  }},"new",{document});
  await model.load();assert.equal(calls.length,0);await model.preview();model.confirmApply(true);await model.apply();assert.equal(model.snapshot().phase,"accepted");assert.equal(calls.length,2);
});
test("create conflict cannot rebase into an overwrite",async()=>{
  let calls=0;
  const model=createQueueDraft({clearToken(){},request:async()=>{calls++;throw Object.assign(new Error("exists"),{status:409});}},"new",{document:newQueueDocument(form)});
  await model.preview();assert.equal(model.snapshot().phase,"conflict");await model.readConflict();model.confirmMerge(true);model.rebase();await model.apply();
  assert.equal(calls,1);assert.equal(model.snapshot().create,true);assert.equal(model.snapshot().etag,undefined);
});
