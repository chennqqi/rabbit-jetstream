import {test} from "node:test";
import assert from "node:assert/strict";
import {validateAuditWindow} from "../src/audit-window.mjs";
import {createQueueEditor} from "../src/queue-editor.mjs";

const windowFor = id => ({requestId:id,items:[],streamPresent:true,firstSequence:1,lastSequence:9007199254740999n,scanned:256,missing:2,nextBefore:9007199254740744n});
test("empty matching windows retain exact uint64 continuation",()=>{
  const window=windowFor("id");assert.equal(validateAuditWindow(window,"id").nextBefore,9007199254740744n);
  for(const patch of [{nextBefore:window.lastSequence+1n},{nextBefore:0},{missing:257},{scanned:0},{items:[{id:"x",requestId:"other",sequence:1}]}])assert.throws(()=>validateAuditWindow({...window,...patch},"id"));
  assert.throws(()=>validateAuditWindow(window,"id",window.nextBefore));
});

test("older audit read preserves empty pages and errors; never unlocks write",async()=>{
  let requestID,fail=false,resolve;
  const paths=[];
  const response=body=>({body,headers:new Headers({ETag:'"7"'})});
  const document={metadata:{name:"q"}};
  const api={clearToken(){},request:async(path,options)=>{
    paths.push(path);
    if(options?.method==="PUT"){requestID=options.headers["X-Request-ID"];throw new Error("lost");}
    if(path.endsWith("/preview"))return response({plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:false,base_revision:'"7"'});
    if(path.includes("/audit/requests/")){
      if(path.includes("?before=")){
        if(fail===true)throw new Error("unavailable");
        if(fail==="pending")return new Promise(done=>resolve=done);
        return response({...windowFor(requestID),scanned:5,missing:0,nextBefore:null,items:[{id:"event",requestId:requestID,sequence:2,phase:"outcome"}]});
      }
      return response(windowFor(requestID));
    }
    return response({queue:"q",document,items:[]});
  }};
  const editor=createQueueEditor(api);await editor.load("q");await editor.preview();await editor.apply();await editor.inspectUncertain();
  assert.equal(editor.snapshot().inspection.audit.windows.length,1);
  fail=true;await editor.inspectOlderAudit();assert.equal(editor.snapshot().inspection.audit.windows.length,1);assert.ok(editor.snapshot().inspection.audit.olderError);
  fail=false;await editor.inspectOlderAudit();assert.equal(editor.snapshot().inspection.audit.windows.length,2,editor.snapshot().inspection.audit.olderError?.message);
  assert.match(paths.at(-1),/before=9007199254740744$/);assert.equal(editor.snapshot().phase,"uncertain");assert.equal(editor.snapshot().etag,'"7"');await assert.rejects(editor.apply());
  await editor.inspectUncertain();fail="pending";const pending=editor.inspectOlderAudit();await assert.rejects(editor.inspectOlderAudit());
  editor.clear({confirmed:true});resolve(response({...windowFor(requestID),nextBefore:null}));await pending;assert.equal(editor.snapshot().phase,"idle");
});
