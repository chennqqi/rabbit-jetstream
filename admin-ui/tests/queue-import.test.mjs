import test from "node:test";
import assert from "node:assert/strict";
import {createQueueImport,parseQueueImport,queueImportLimit} from "../src/queue-import.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {retainCreationDraft} from "../src/queue-create.mjs";

const raw='{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"imported","labels":{"owner":"review-me"}},"spec":{"replicas":1,"subjects":["orders.created"],"maxPriority":0,"retention":{"maxMessages":9223372036854775807},"deadLetter":{"queue":"failed"}}}';
const file=text=>{const bytes=new TextEncoder().encode(text);return {size:bytes.byteLength,arrayBuffer:async()=>bytes.buffer};};

test("single Queue import preserves exact values and rejects envelopes without rewriting fields",()=>{
  const value=parseQueueImport(raw);assert.equal(value.spec.retention.maxMessages,9223372036854775807n);assert.equal(value.spec.maxPriority,0);assert.equal(value.metadata.labels.owner,"review-me");assert.equal(value.spec.deadLetter.queue,"failed");
  for(const text of ["",`[${raw}]`,raw+raw,raw.replace("v1alpha1","v9"),raw.replace('"Queue"','"Plan"'),raw.replace('"metadata":','"credential":"x","metadata":'),raw.replace('"owner":"review-me"','"owner":42'),"a".repeat(queueImportLimit+1)])assert.throws(()=>parseQueueImport(text));
  assert.equal(parseQueueImport(raw.replace('"replicas":1','"unsupported":123,"replicas":1')).spec.unsupported,123);
});

test("file import rejects empty/oversized/mismatched size and invalid UTF-8 before draft preparation",async()=>{
  const model=createQueueImport();await model.read({...file(raw),name:"queue.json"});assert.equal(model.snapshot().phase,"ready");assert.equal(model.snapshot().filename,"queue.json");
  for(const bad of [{size:0,arrayBuffer(){throw Error("must not read");}},{size:queueImportLimit+1,arrayBuffer(){throw Error("must not read");}},{size:2,arrayBuffer:async()=>new Uint8Array([255,255]).buffer},{size:3,arrayBuffer:async()=>new Uint8Array([123]).buffer}]){
    await model.read(bad);assert.equal(model.snapshot().phase,"error");assert.equal(model.snapshot().document,null);
  }
  await model.read(file(raw));assert.equal(model.snapshot().phase,"ready");
});

test("canceled and superseded file reads cannot replace the current document",async()=>{
  const model=createQueueImport();let resolve;const delayed={size:file(raw).size,arrayBuffer:()=>new Promise(done=>{resolve=done;})};
  const pending=model.read(delayed);model.clear();resolve(await file(raw).arrayBuffer());await pending;assert.equal(model.snapshot().phase,"idle");
  const old=model.read(delayed);await model.read(file(raw.replace('"imported"','"newer"')));resolve(await file(raw).arrayBuffer());await old;assert.equal(model.snapshot().document.metadata.name,"newer");
});

test("import preparation performs no IO and retains create-only conflict protection",async()=>{
  const document=parseQueueImport(raw),drafts=new Map(),calls=[];
  const model=retainCreationDraft(drafts,document,value=>createQueueDraft({request:async(path,options)=>{calls.push({path,options});throw {status:409};}},value.metadata.name,{document:value}));
  await model.load();assert.equal(calls.length,0);assert.equal(model.snapshot().create,true);
  assert.throws(()=>retainCreationDraft(drafts,document,()=>{throw Error("must not replace");}),{code:"retained-name"});
  await model.preview();assert.equal(calls.length,1);assert.equal(calls[0].options.method,"POST");assert.equal(calls[0].options.headers["If-None-Match"],"*");assert.equal(calls[0].options.headers["If-Match"],undefined);
  model.confirmApply(true);await model.apply();assert.equal(calls.length,1);assert.equal(model.snapshot().phase,"conflict");
});
