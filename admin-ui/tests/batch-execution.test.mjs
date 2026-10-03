import test from "node:test";
import assert from "node:assert/strict";
import {createBatchExecution} from "../src/batch-execution.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {retainCreationDraft,archiveCreationBatch} from "../src/queue-create.mjs";
import {archiveQueueEditors} from "../src/editor-handoff.mjs";
import {parseJSON,stringifyJSON} from "../src/api.mjs";
import {validateBatchResult} from "../src/batch-import.mjs";
import {batchEvidence} from "../src/batch-evidence.mjs";

function planning(){
  const files=["source","target","independent"].map(name=>({name:`${name}.json`,status:"loaded",document:{apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{replicas:1,subjects:[`${name}.events`],retention:{maxMessages:9223372036854775807n},...(name==="source"?{deadLetter:{queue:"target"}}:{})}}}));
  return {phase:"ready",binding:{revision:"binding"},files,result:{ready:true,order:[2,1,0],external:[],items:files.map((file,index)=>({index,queue:file.document.metadata.name,revision:"revision",dependencies:index===0?["target"]:[],problems:[]}))}};
}
function fixture(){
  const calls=[],drafts=new Map(),api={clearToken(){},request:async(path,options)=>{
    const name=path.split("/")[4];calls.push({path,options});
    return {body:options.method==="POST"?{plan:{queue:name},result:{queue:name,status:"ready",blocked:false,operations:[]},create_only:true,base_revision:""}:{queue:name,status:"ready",blocked:false},headers:new Headers()};
  }};
  const factory=document=>retainCreationDraft(drafts,document,value=>createQueueDraft(api,value.metadata.name,{document:value}));
  return {calls,drafts,api,factory};
}

function externalPlanning(status="present"){
  const input=planning();input.files[1].document.spec.deadLetter={queue:"outside"};
  input.result.items[1].dependencies=["outside"];input.result.items[1].problems=[{code:"external_dependency_unverified",dependency:"outside"}];
  input.result.items[0].problems=[{code:"blocked_dependency",dependency:"target"}];input.result.order=[2];input.result.review_order=[2,1,0];input.result.ready=false;
  input.result.external=[{queue:"outside",status,...(status==="present"?{etag:'"1"'}:{})}];return input;
}

test("external review order opens preview only, retaining in-batch acceptance and independent confirmation",async()=>{
  const input=externalPlanning("missing"),batch=createBatchExecution(input),f=fixture();
  assert.equal(batch.prepare(0,f.factory),false);assert.equal(batch.prepare(1,f.factory),true);const target=batch.selectedModel();await target.apply();assert.equal(f.calls.length,0);
  // The saved planning observation may be stale; only the current server
  // preview can decide whether the repaired destination now satisfies it.
  await target.preview();await target.apply();assert.equal(f.calls.length,1);assert.equal(batch.prepare(0,f.factory),false);
  target.confirmApply(true);await target.apply();assert.equal(f.calls.length,2);assert.equal(batch.prepare(0,f.factory),true);
  assert.equal(input.result.ready,false);assert.equal(input.result.items[1].problems[0].code,"external_dependency_unverified");
});

test("every external observation status still requires a successful current preview before writing",async()=>{
  for(const status of ["present","missing","unavailable","unrepresentable"]){
    const batch=createBatchExecution(externalPlanning(status)),f=fixture();f.api.request=async(path,options)=>{f.calls.push({path,options});throw Object.assign(Error("target check failed"),{status:503});};
    assert.equal(batch.prepare(1,f.factory),true);const model=batch.selectedModel();await model.preview();model.confirmApply(true);await model.apply();assert.equal(f.calls.length,1);assert.equal(f.calls[0].options.method,"POST");assert.equal(batch.prepare(0,f.factory),false);
  }
});

test("review order rejects omissions, dependency inversions, duplicates and structurally blocked items",()=>{
  for(const mutate of [value=>value.result.review_order=[],value=>value.result.review_order=[0,1,2],value=>value.result.review_order=[2,1,1,0],value=>value.result.review_order=[2,1],value=>value.result.review_order=null,value=>value.result.items[1].problems.push({code:"invalid_declaration"}),value=>value.result.items[1].problems.push({code:"dependency_cycle"})]){
    const input=externalPlanning();mutate(input);assert.throws(()=>createBatchExecution(input));
  }
  const old=externalPlanning();delete old.result.review_order;
  old.result=validateBatchResult({scope:"declaration-only-not-apply-authorization",plan:old.result,external_declarations:old.result.external},old.files);
  assert.equal(old.result.review_order,undefined);const batch=createBatchExecution(old);assert.equal(batch.prepare(1,fixture().factory),false,"legacy projection must not upgrade eligibility");
});

test("batch execution freezes planning inputs, performs no implicit IO and requires accepted prerequisites",async()=>{
  const input=planning(),batch=createBatchExecution(input),f=fixture();input.files[1].document.metadata.name="mutated";input.result.order.length=0;
  assert.equal(batch.prepare(0,f.factory),false);assert.equal(batch.snapshot().failure,"prerequisite-not-accepted");assert.equal(f.drafts.size,0);
  assert.equal(batch.prepare(1,f.factory),true);const target=batch.selectedModel();assert.equal(target.snapshot().name,"target");assert.equal(target.snapshot().draft.spec.retention.maxMessages,9223372036854775807n);assert.equal(f.calls.length,0);
  await target.preview();await target.apply();assert.equal(f.calls.length,1,"preview is not approval");target.confirmApply(true);await target.apply();assert.equal(target.snapshot().phase,"accepted");
  assert.equal(batch.prepare(0,f.factory),true);assert.equal(f.calls.length,2,"preparing dependent performs no IO");
  assert.ok(f.calls.every(call=>call.options.headers["If-None-Match"]==="*"&&call.options.headers["If-Match"]===undefined));
});

test("switching drafts invalidates old approval and stale model references cannot submit",async()=>{
  const batch=createBatchExecution(planning()),f=fixture();batch.prepare(1,f.factory);const target=batch.selectedModel();await target.preview();target.confirmApply(true);
  batch.prepare(2,f.factory);assert.equal(target.snapshot().phase,"editing");assert.equal(target.snapshot().applyConfirmed,false);
  await target.preview();await target.apply();assert.equal(f.calls.length,1);assert.equal(batch.snapshot().failure,"not-selected");
  assert.equal(batch.select(1),true);await target.apply();assert.equal(f.calls.length,1);await target.preview();target.confirmApply(true);await target.apply();assert.equal(f.calls.length,3);
});

test("changed dependency graph blocks preview/apply until restored and re-reviewed",async()=>{
  const batch=createBatchExecution(planning()),f=fixture();batch.prepare(1,f.factory);const target=batch.selectedModel();target.edit(stringifyJSON({...target.snapshot().draft,spec:{...target.snapshot().draft.spec,deadLetter:{queue:"other"}}}));
  await target.preview();target.confirmApply(true);await target.apply();assert.equal(f.calls.length,0);assert.equal(batch.snapshot().failure,"dependency-changed");
  target.edit(stringifyJSON(planning().files[1].document));await target.preview();assert.equal(f.calls.length,1);
});

test("in-flight and unknown writes lock selection and preserve original request evidence",async()=>{
  const batch=createBatchExecution(planning()),f=fixture();let release;
  f.api.request=async(path,options)=>{f.calls.push({path,options});if(options.method==="POST")return {body:{plan:{queue:"target"},result:{queue:"target",status:"ready",blocked:false,operations:[]},create_only:true,base_revision:""},headers:new Headers()};await new Promise(done=>{release=done;});throw Error("lost response");};
  batch.prepare(2,f.factory);batch.prepare(1,f.factory);const target=batch.selectedModel();await target.preview();target.confirmApply(true);const pending=target.apply();
  assert.equal(target.snapshot().phase,"submitting");assert.equal(batch.select(2),false);assert.equal(batch.prepare(0,f.factory),false);
  assert.equal(batch.snapshot().canArchive,false);assert.equal(batch.archive({confirmed:true}),null);
  release();await pending;assert.equal(target.snapshot().phase,"uncertain");const requestId=target.snapshot().requestId;assert.ok(requestId);
  assert.equal(batch.select(2),false);assert.equal(batch.snapshot().selected,1);await target.apply();assert.equal(f.calls.length,2);assert.equal(target.snapshot().requestId,requestId);
  assert.equal(batch.archive({confirmed:true}),null);assert.equal(target.snapshot().requestId,requestId);
});

test("batch archive requires confirmation, preserves frozen evidence and performs no API operations",async()=>{
  const batch=createBatchExecution(planning()),f=fixture();batch.prepare(1,f.factory);const target=batch.selectedModel();await target.preview();target.confirmApply(true);
  assert.equal(batch.archive(),null);assert.equal(target.snapshot().applyConfirmed,true);
  const record=batch.archive({confirmed:true});assert.equal(record.schema,"rjs.batch-execution-evidence.v1");assert.equal(record.scope,"archived-batch-not-write-authorization");assert.equal(record.items[1].outcome.applyConfirmed,true,"historical evidence is retained, not reused");
  assert.ok(record.limitations.some(value=>value.includes("never atomic")));assert.ok(record.limitations.some(value=>value.includes("excludes messages")));
  const evidence=parseJSON(batchEvidence(record));assert.equal(evidence.schema,"rjs.batch-execution-evidence.v1");assert.equal(evidence.items[1].document.spec.retention.maxMessages,9223372036854775807n);
  assert.equal(record.items[0].outcome.phase,"not-prepared");assert.equal(record.items[1].document.spec.retention.maxMessages,9223372036854775807n);assert.equal(target.snapshot().applyConfirmed,false);assert.equal(batch.snapshot().archived,true);assert.equal(batch.selectedModel(),null);
  await target.preview();target.confirmApply(true);await target.apply();assert.equal(f.calls.length,1);assert.equal(batch.prepare(2,f.factory),false);assert.equal(batch.select(1),false);assert.equal(batch.archive({confirmed:true}),null);
  const raw=target.snapshot().raw;target.edit("{");assert.equal(target.snapshot().raw,raw);assert.notEqual(record.items[1].outcome.raw,"{");assert.equal(f.drafts.has("create:target"),true);
});

test("archival revokes all commands on stale batch views without clearing the shared token",async()=>{
  const f=fixture(),batch=createBatchExecution(planning());let cleared=0;
  f.api.clearToken=()=>{cleared++;};batch.prepare(1,f.factory);const view=batch.selectedModel();
  const record=batch.archive({confirmed:true}),before=stringifyJSON(view.snapshot());
  for(const [key,command] of Object.entries(view)){
    if(typeof command==="function"&&!['snapshot','subscribe'].includes(key))await command();
  }
  assert.equal(cleared,0);assert.equal(f.calls.length,0);assert.equal(stringifyJSON(view.snapshot()),before);assert.equal(record.items[1].outcome.phase,"editing");
  const real=f.drafts.get("create:target");real.discard();assert.equal(cleared,1,"session owner must retain cleanup authority");
});

test("archiving a batch permits a new batch but retains names and normal deletion handoff",async()=>{
  const f=fixture(),setup={batch:createBatchExecution(planning()),form:{name:"keep"}};setup.batch.prepare(1,f.factory);const target=setup.batch.selectedModel();await target.preview();target.confirmApply(true);await target.apply();const requestId=target.snapshot().requestId;
  assert.equal(archiveCreationBatch(setup),false);assert.equal(setup.batchHistory,undefined);
  assert.equal(archiveCreationBatch(setup,{confirmed:true}),true);assert.equal(setup.batch,null);assert.equal(setup.form.name,"keep");assert.equal(setup.batchHistory[0].items[1].outcome.requestId,requestId);
  setup.batch=createBatchExecution(planning());assert.equal(setup.batch.prepare(1,f.factory),false);assert.equal(setup.batch.snapshot().failure,"retained-name");assert.equal(setup.batch.prepare(2,f.factory),true);
  assert.equal(archiveQueueEditors({drafts:f.drafts,archives:new Map(),creation:setup,name:"target",confirmed:true}),true,"batch archival must not strand the existing deletion handoff");
  assert.equal(setup.batchHistory[0].items[1].outcome.phase,"accepted");assert.equal(f.calls.length,2);
});

test("external and invalid planning items remain blocked without treating presence as authorization",()=>{
  const input=planning();input.result.items[1].dependencies=["outside"];input.result.items[1].problems=[{code:"external_dependency_unverified",dependency:"outside"}];input.result.items[0].problems=[{code:"blocked_dependency",dependency:"target"}];input.result.external=[{queue:"outside",status:"present",etag:'"1"'}];input.result.order=[2];input.result.ready=false;
  const batch=createBatchExecution(input),f=fixture();assert.equal(batch.prepare(1,f.factory),false);assert.equal(batch.prepare(0,f.factory),false);assert.equal(f.drafts.size,0);assert.equal(batch.prepare(2,f.factory),true);
  assert.throws(()=>createBatchExecution({...input,phase:"files"}));const bad=planning();bad.result.order=[0,1,2];assert.throws(()=>createBatchExecution(bad));
});

test("retained-name failures do not overwrite drafts, and archived prerequisites revoke dependent dispatch",async()=>{
  const batch=createBatchExecution(planning()),f=fixture();f.factory(planning().files[2].document);assert.equal(batch.prepare(2,f.factory),false);assert.equal(batch.snapshot().failure,"retained-name");assert.equal(batch.snapshot().items[2].phase,"not-prepared");
  batch.prepare(1,f.factory);const target=batch.selectedModel();await target.preview();target.confirmApply(true);await target.apply();batch.prepare(0,f.factory);const source=batch.selectedModel();await source.preview();source.confirmApply(true);
  target.archive();await source.apply();assert.equal(f.calls.length,3);assert.equal(batch.snapshot().failure,"prerequisite-not-accepted");assert.equal(batch.snapshot().items[1].phase,"archived");
});
