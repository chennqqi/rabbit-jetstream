import test from "node:test";
import assert from "node:assert/strict";
import {editQueueLabel} from "../src/queue-labels.mjs";
import {parseJSON} from "../src/api.mjs";
const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q",labels:{owner:"team",region:"cn"}},spec:{subjects:["a"],replicas:1,storage:"file",retention:{maxMessages:9223372036854775807n}}};
const edit=(doc,action,confirmed=false)=>parseJSON(editQueueLabel(doc,action,{confirmed}));

test("label add/rename/value/remove preserve exact text and unrelated draft data",()=>{
  const before=structuredClone(document);
  let result=edit(document,{kind:"add",key:"环境"});
  assert.equal(result.metadata.labels["环境"],"");
  result=edit(result,{kind:"value",key:"环境",value:" 第一行\nsecond\t "});
  result=edit(result,{kind:"rename",key:"环境",value:"Environment"});
  assert.equal(result.metadata.labels.Environment," 第一行\nsecond\t ");
  assert.equal(Object.hasOwn(result.metadata.labels,"环境"),false);
  assert.throws(()=>edit(result,{kind:"remove",key:"Environment"}),{code:"confirmation-required"});
  result=edit(result,{kind:"remove",key:"Environment"},true);
  assert.deepEqual(result,document);assert.deepEqual(document,before);
});

test("label duplicate keys never overwrite and absent rows cannot be edited",()=>{
  for(const action of [{kind:"add",key:"owner"},{kind:"rename",key:"region",value:"owner"}]){
    const before=structuredClone(document);assert.throws(()=>edit(document,action),{code:"duplicate-label"});assert.deepEqual(document,before);
  }
  for(const kind of ["rename","value","remove"])assert.throws(()=>edit(document,{kind,key:"missing",value:"x"},true),{code:"missing-label"});
  assert.deepEqual(edit(document,{kind:"rename",key:"owner",value:"owner"}),document);
  assert.equal(edit(document,{kind:"add",key:"Owner"}).metadata.labels.Owner,"");
});

test("special keys are own properties, not prototype writes; empty keys/values are not guessed away",()=>{
  let result=structuredClone(document);
  for(const key of ["__proto__","constructor","toString",""]){
    result=edit(result,{kind:"add",key});result=edit(result,{kind:"value",key,value:"exact"});
    assert.equal(Object.hasOwn(result.metadata.labels,key),true);assert.equal(result.metadata.labels[key],"exact");
  }
  assert.equal(Object.getPrototypeOf(result.metadata.labels),Object.prototype);
  assert.equal({}.exact,undefined);assert.equal(result.spec.retention.maxMessages,9223372036854775807n);
  const omitted=structuredClone(document);delete omitted.metadata.labels;
  assert.deepEqual(edit(omitted,{kind:"add",key:"first"}).metadata.labels,{first:""});
  const unsupported={...document,spec:{...document.spec,unknown:true}};
  assert.throws(()=>edit(unsupported,{kind:"add",key:"new"}));
});
