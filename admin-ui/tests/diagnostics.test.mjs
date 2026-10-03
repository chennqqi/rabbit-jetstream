import {test} from "node:test";
import assert from "node:assert/strict";
import {createDiagnostics,diagnosticJob} from "../src/diagnostics.mjs";

const id="0123456789abcdef0123456789abcdef",base={id,profile:"metadata-v1",state:"collecting",createdAt:"2026-09-11T00:00:00Z",expiresAt:"2026-09-11T00:10:00Z"};

test("diagnostic job validation rejects invented or oversized evidence",()=>{
  assert.equal(diagnosticJob(base).state,"collecting");
  for(const value of [{...base,id:"x"},{...base,state:"done"},{...base,finishedAt:"bad"},{...base,manifest:{schema:"wrong",generatedAt:base.createdAt,entries:[]}},
    {...base,manifest:{schema:"rjs.diagnostics-manifest.v1",generatedAt:base.createdAt,entries:[{source:"x",collectedAt:base.createdAt,size:4194305}]}}])assert.throws(()=>diagnosticJob(value));
});

test("diagnostics explicitly creates, tracks, downloads and revokes one object URL",async()=>{
  const calls=[],revoked=[];let status={...base};
  const api={request:async(path,options={})=>{calls.push({path,options});if(options.method==="POST")return{body:base};if(options.method==="DELETE")return{body:{...base,state:"cancelled",finishedAt:base.createdAt,errorCode:"cancelled"}};return{body:status};},download:async path=>{calls.push({path,download:true});return{blob:new Blob(["zip"]),size:3};}};
  const model=createDiagnostics(api,{createURL:()=>"blob:one",revokeURL:value=>revoked.push(value)});
  assert.equal(calls.length,0);await model.create();assert.equal(model.snapshot().job.state,"collecting");
  assert.deepEqual(calls[0].options.body,{schema:"rjs.diagnostics-request.v1",profile:"metadata-v1"});
  status={...base,state:"ready",finishedAt:base.createdAt,manifest:{schema:"rjs.diagnostics-manifest.v1",generatedAt:base.createdAt,entries:[]}};
  await model.refresh();assert.equal(model.snapshot().job.state,"ready");await model.download();assert.equal(model.snapshot().download.url,"blob:one");
  model.clear();assert.deepEqual(revoked,["blob:one"]);assert.equal(model.snapshot().phase,"idle");
});

test("clearing diagnostics suppresses a late create response",async()=>{
  let resolve;const api={request:()=>new Promise(done=>{resolve=done;}),download:async()=>{throw Error("unexpected");}};
  const model=createDiagnostics(api,{createURL:()=>"blob:x",revokeURL:()=>{}}),pending=model.create();model.clear();resolve({body:base});await pending;assert.equal(model.snapshot().job,null);
});
