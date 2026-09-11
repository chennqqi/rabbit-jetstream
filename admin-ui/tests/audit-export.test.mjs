import {test} from "node:test";
import assert from "node:assert/strict";
import {createAuditExport} from "../src/audit-export.mjs";
import {readAuditQuery} from "../src/audit-query.mjs";
import {parseJSON} from "../src/api.mjs";
const query=readAuditQuery("?actor=owner"),filter={...query};delete filter.before;
const first={items:[],streamPresent:true,firstSequence:1,lastSequence:300,scanned:256,missing:0,nextBefore:45,filter};
const event={id:"e",requestId:"r",sequence:1,time:"2026-09-10T00:00:00Z",actor:"owner",resourceName:"q",phase:"outcome",action:"queue.apply",outcome:"accepted"};
const second={...first,items:[event],lastSequence:301,scanned:44,nextBefore:null};

test("Export crosses empty matching window and ignores current-page cursor",async()=>{
  const calls=[];const model=createAuditExport({request:async path=>{calls.push(new URL(path,"http://localhost").searchParams);return {body:calls.length===1?first:second};}});
  await model.start({...query,before:"99"});const result=model.snapshot();assert.equal(result.phase,"ready");assert.equal(result.events,1);
  assert.equal(calls[0].has("before"),false);assert.equal(calls[1].get("before"),"45");
  const body=parseJSON(result.content);assert.equal(body.initialLastSequence,300);assert.equal(body.windows.length,2);assert.match(body.coverage,/Not an atomic snapshot/);
});
test("Read failure or safety ceiling never exposes a partial download",async()=>{
  for(const mode of ["read","windows","bytes"]){
    let calls=0;const model=createAuditExport({request:async()=>{if(++calls===2)throw new Error("private");return {body:first};}},{maxWindows:mode==="windows"?1:256,maxBytes:mode==="bytes"?1:16000000});
    await model.start(query);assert.equal(model.snapshot().phase,"error");assert.equal(model.snapshot().content,null);
    assert.equal(model.snapshot().failure,mode==="read"?"read":"limit");
  }
});
test("Cancel suppresses a late response without additional windows",async()=>{
  let resolve,calls=0;const model=createAuditExport({request:()=>{calls++;return new Promise(done=>{resolve=done;});}});
  const pending=model.start(query);model.cancel();resolve({body:first});await pending;assert.equal(calls,1);assert.equal(model.snapshot().phase,"idle");assert.equal(model.snapshot().content,null);
});
