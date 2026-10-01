import test from "node:test";
import assert from "node:assert/strict";
import {editQueueField,queueFields,queueFieldValue,supportsQueueFields} from "../src/queue-fields.mjs";
import {parseJSON} from "../src/api.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q",labels:{owner:"team"}},spec:{
  subjects:null,bindings:[{exchange:"events",type:"topic",keys:["a.*"]}],replicas:3,storage:"file",
  retention:{maxAge:"10ns",maxBytes:"9223372036854775807",maxMessages:9223372036854775807n},
  delivery:{ackWait:"30s",maxDeliver:5},maxPriority:0,deadLetter:{queue:"failed"}}};

test("structured field edits preserve unrelated configuration, units and exact integers",()=>{
  assert.equal(supportsQueueFields(document),true);
  for(const field of queueFields){
    const before=structuredClone(document),text=queueFieldValue(document,field);
    assert.deepEqual(parseJSON(editQueueField(document,field.key,text)),document);
    assert.deepEqual(document,before);
  }
  const changed=parseJSON(editQueueField(document,"maxMessages","9007199254740993"));
  assert.equal(changed.spec.retention.maxMessages,9007199254740993n);
  assert.deepEqual(changed.spec.bindings,document.spec.bindings);assert.deepEqual(changed.metadata,document.metadata);
  assert.equal(changed.spec.retention.maxAge,"10ns");
});

test("omission, zero, unfinished numeric input and oversized values are never silently normalized",()=>{
  assert.equal(parseJSON(editQueueField(document,"maxPriority","")).spec.maxPriority,undefined);
  assert.equal(parseJSON(editQueueField(document,"maxPriority","0")).spec.maxPriority,0);
  assert.equal(parseJSON(editQueueField(document,"deadLetter","")).spec.deadLetter,undefined);
  for(const input of ["-","1e3","01","1.5"," 42 "]){
    const next=parseJSON(editQueueField(document,"maxMessages",input));
    assert.equal(next.spec.retention.maxMessages,input);assert.equal(supportsQueueFields(next),true);
  }
  assert.equal(parseJSON(editQueueField(document,"maxMessages","9223372036854775808")).spec.retention.maxMessages,9223372036854775808n);
  const omitted=parseJSON(editQueueField(document,"maxDeliver",""));
  assert.equal(omitted.spec.delivery.maxDeliver,undefined);assert.equal(omitted.spec.delivery.ackWait,"30s");
});

test("unknown fields and nonrepresentable shapes block conversion without deleting data",()=>{
  const variants=[{...document,extra:true},{...document,apiVersion:"future"},
    {...document,spec:{...document.spec,future:true}},
    {...document,spec:{...document.spec,retention:null}},
    {...document,spec:{...document.spec,deadLetter:{queue:"q",future:true}}},
    {...document,spec:{...document.spec,bindings:[{exchange:"e",type:"topic",keys:[3]}]}},
    {...document,spec:{...document.spec,maxPriority:{nested:true}}}];
  for(const value of variants){
    const before=structuredClone(value);assert.equal(supportsQueueFields(value),false);
    assert.throws(()=>editQueueField(value,"storage","memory"));assert.deepEqual(value,before);
  }
  assert.throws(()=>editQueueField(document,"metadata.name","other"));
});

test("field edits use session draft, invalidate confirmation and retain original precondition",async()=>{
  const calls=[];
  const model=createQueueDraft({clearToken(){},async request(path,options){
    calls.push({path,options});
    if(!options)return {body:{queue:"q",document},headers:new Headers({ETag:'"9007199254740993"'})};
    return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false, operations: []},create_only:false,base_revision:'"9007199254740993"'}};
  }},"q");
  await model.load();await model.preview();model.confirmApply(true);
  model.edit(editQueueField(model.snapshot().draft,"maxMessages","9007199254740993"));
  assert.equal(model.snapshot().phase,"editing");assert.equal(model.snapshot().preview,undefined);
  assert.equal(model.snapshot().applyConfirmed,false);await model.apply();assert.equal(calls.length,2);
  await model.preview();assert.equal(calls.at(-1).options.headers["If-Match"],'"9007199254740993"');
  assert.equal(calls.at(-1).options.body.spec.retention.maxMessages,9007199254740993n);
});
