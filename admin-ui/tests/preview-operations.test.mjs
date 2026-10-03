import test from "node:test";
import assert from "node:assert/strict";
import {previewOperations} from "../src/preview-operations.mjs";
const operation={resource:"stream",name:"S",action:"update",impact:"safe",blocked:false,changes:[{path:"maxMessages",from:"9007199254740993",to:"9223372036854775807",impact:"safe"}]};
test("preview operation presentation preserves exact server values and missing versus empty",()=>{
  const result={operations:[operation,{resource:"consumer",name:"S",action:"create",impact:"safe",blocked:false,changes:[]}]};
  const before=structuredClone(result);assert.deepEqual(previewOperations(result),before.operations);assert.deepEqual(result,before);
  const omitted={...operation,changes:[{path:"metadata.x",to:"",impact:"safe"}]};
  const [row]=previewOperations({operations:[omitted]});assert.equal(Object.hasOwn(row.changes[0],"from"),false);assert.equal(row.changes[0].to,"");
  assert.deepEqual(previewOperations({operations:[]}),[]);
});
test("malformed operation collections fail as a whole without a misleading partial summary",()=>{
  for(const invalid of [null,{},[],{...operation,blocked:"false"},{...operation,changes:null},{...operation,changes:[{path:"x",from:1,impact:"safe"}]},{...operation,reason:42}])assert.equal(previewOperations({operations:[operation,invalid]}),null);
  assert.equal(previewOperations({operations:[operation,operation]}),null);
  assert.equal(previewOperations({}),null);
  const future={...operation,action:"future-action",impact:"future-impact"};assert.deepEqual(previewOperations({operations:[future]}),[future]);
});
