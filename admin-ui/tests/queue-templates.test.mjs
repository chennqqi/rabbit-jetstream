import {test} from "node:test";
import assert from "node:assert/strict";
import {queueTemplates,templateQueueDocument} from "../src/queue-templates.mjs";
import {newQueueDocument} from "../src/queue-create.mjs";

const base={name:"orders",subjects:"orders.events",replicas:"1",storage:"file",maxMessages:"9223372036854775807"};
test("templates preserve legacy creation and require explicit deployment choices",()=>{
  assert.deepEqual(templateQueueDocument(base),newQueueDocument(base));
  assert.deepEqual(templateQueueDocument({...base,templateId:"basic"}),newQueueDocument(base));
  for(const key of ["replicas","storage","maxMessages","name","subjects"]){
    assert.throws(()=>templateQueueDocument({...base,[key]:"",templateId:"priority",templatePriority:"5"}));
  }
  for(const templateId of ["unknown","__proto__","constructor"])assert.throws(()=>templateQueueDocument({...base,templateId}));
  assert.equal(Object.isFrozen(queueTemplates),true);
  assert.ok(queueTemplates.every(Object.isFrozen));
});
test("priority templates validate integer bounds and do not retain fields from other templates",()=>{
  for(const templatePriority of ["1","255"]){
    const form={...base,templateId:"priority",templatePriority,templateDLQ:"old-target"};
    const before={...form},document=templateQueueDocument(form);
    assert.deepEqual(form,before);
    assert.equal(document.spec.maxPriority,Number(templatePriority));
    assert.equal(document.spec.deadLetter,undefined);
    assert.equal(document.spec.retention.maxMessages,9223372036854775807n);
    document.spec.subjects.push("changed");
    assert.deepEqual(templateQueueDocument(form).spec.subjects,["orders.events"]);
  }
  for(const templatePriority of [undefined,"","0","256","-1","1.5","1e2","Infinity",255]){
    assert.throws(()=>templateQueueDocument({...base,templateId:"priority",templatePriority}));
  }
  assert.equal(templateQueueDocument({...base,templateId:"basic",templatePriority:"5"}).spec.maxPriority,undefined);
});
test("DLQ templates preserve exact target, reject self reference and never create dependencies",()=>{
  const form={...base,templateId:"dead-letter",templateDLQ:"orders_dlq",templatePriority:"5"};
  assert.deepEqual(templateQueueDocument(form),{...newQueueDocument(base),spec:{...newQueueDocument(base).spec,deadLetter:{queue:"orders_dlq"}}});
  for(const templateDLQ of [undefined,"","orders"," bad","a/b","a".repeat(257)])assert.throws(()=>templateQueueDocument({...form,templateDLQ}));
  assert.equal(templateQueueDocument({...form,templateId:""}).spec.deadLetter,undefined);
});
test("templates obey current schema deployment choices, without defaults or widening",()=>{
  const controls={fields:[{key:"replicas",options:["3"]},{key:"storage",options:["file"]},{key:"maxMessages",minimum:1,maximum:100n}]};
  assert.throws(()=>templateQueueDocument(base,controls));
  const form={...base,replicas:"3",maxMessages:"100",templateId:"priority",templatePriority:"2"};
  assert.equal(templateQueueDocument(form,controls).spec.maxPriority,2);
  assert.throws(()=>templateQueueDocument({...form,maxMessages:"101"},controls));
});
