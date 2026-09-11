import test from "node:test";
import assert from "node:assert/strict";
import {editQueueRouting,routingMode,routingNeedsConfirmation} from "../src/queue-routing.mjs";
import {parseJSON} from "../src/api.mjs";
const source={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q",labels:{owner:"team"}},spec:{subjects:["a.*"],replicas:1,storage:"file",retention:{maxMessages:9223372036854775807n}}};
const edit=(value,action,confirmed=false)=>parseJSON(editQueueRouting(value,action,{confirmed}));

test("routing mode changes require confirmation and never guess a conversion",()=>{
  const before=structuredClone(source),action={kind:"mode",value:"bindings"};
  assert.equal(routingNeedsConfirmation(source,action),true);assert.throws(()=>edit(source,action));
  assert.deepEqual(source,before);
  const bindings=edit(source,action,true);assert.equal(routingMode(bindings),"bindings");
  assert.equal(bindings.spec.subjects,undefined);assert.deepEqual(bindings.spec.bindings,[]);
  assert.deepEqual(bindings.metadata,source.metadata);assert.equal(bindings.spec.retention.maxMessages,9223372036854775807n);
  const subjects=edit(bindings,{kind:"mode",value:"subjects"});assert.equal(routingMode(subjects),"subjects");
  assert.equal(subjects.spec.bindings,undefined);assert.deepEqual(subjects.spec.subjects,[]);
  const mixed={...source,spec:{...source.spec,bindings:[{exchange:"e",type:"fanout"}]}};
  assert.equal(routingMode(mixed),"mixed");assert.throws(()=>edit(mixed,{kind:"add-subject"}));
  assert.throws(()=>edit(mixed,{kind:"mode",value:"subjects"}));
  const chosenSubjects=edit(mixed,{kind:"mode",value:"subjects"},true);
  assert.deepEqual(chosenSubjects.spec.subjects,mixed.spec.subjects);assert.equal(chosenSubjects.spec.bindings,undefined);
  const chosenBindings=edit(mixed,{kind:"mode",value:"bindings"},true);
  assert.deepEqual(chosenBindings.spec.bindings,mixed.spec.bindings);assert.equal(chosenBindings.spec.subjects,undefined);
});

test("binding rows retain exact input and fanout conversion removes keys only with consent",()=>{
  let doc=edit(source,{kind:"mode",value:"bindings"},true);
  doc=edit(doc,{kind:"add-binding"});assert.deepEqual(doc.spec.bindings,[{exchange:"",type:"",keys:[]}]);
  doc=edit(doc,{kind:"exchange",index:0,value:"orders"});doc=edit(doc,{kind:"type",index:0,value:"topic"});
  doc=edit(doc,{kind:"add-key",index:0});doc=edit(doc,{kind:"key",index:0,keyIndex:0,value:" a.# "});
  assert.equal(doc.spec.bindings[0].keys[0]," a.# ");
  const before=structuredClone(doc);assert.throws(()=>edit(doc,{kind:"type",index:0,value:"fanout"}));assert.deepEqual(doc,before);
  doc=edit(doc,{kind:"type",index:0,value:"fanout"},true);assert.equal(doc.spec.bindings[0].keys,undefined);
  assert.throws(()=>edit(doc,{kind:"add-key",index:0}));
  assert.throws(()=>edit(doc,{kind:"remove-binding",index:0}));
  doc=edit(doc,{kind:"remove-binding",index:0},true);assert.deepEqual(doc.spec.bindings,[]);
});

test("repeating rows preserve duplicates, blanks and exact characters until explicitly edited",()=>{
  let doc=edit(source,{kind:"add-subject"});assert.deepEqual(doc.spec.subjects,["a.*",""]);
  doc=edit(doc,{kind:"subject",index:1,value:"a.*"});assert.deepEqual(doc.spec.subjects,["a.*","a.*"]);
  assert.throws(()=>edit(doc,{kind:"remove-subject",index:0}));
  doc=edit(doc,{kind:"remove-subject",index:0},true);assert.deepEqual(doc.spec.subjects,["a.*"]);
  for(const index of [-1,1,0.5,"0"])assert.throws(()=>edit(doc,{kind:"subject",index,value:"b"}));
  assert.throws(()=>edit(doc,{kind:"mode",value:"unknown"}));
  assert.throws(()=>edit({...source,spec:{...source.spec,future:true}},{kind:"add-subject"}));
});
