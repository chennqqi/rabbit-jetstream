import test from "node:test";
import assert from "node:assert/strict";
import {mutationEvidence,errorMutationEvidence} from "../src/mutation-evidence.mjs";
import {queueEvidence} from "../src/queue-evidence.mjs";
import {deleteEvidence} from "../src/delete-evidence.mjs";

const evidence={schemaVersion:"rjs.mutation-evidence.v1",scope:"receiving-attempt",phase:"audit_intent",resourceEffects:"none",intentId:"a".repeat(32)};
test("attempt evidence validates version, scope, phase, effects and strict identifier types",()=>{
  assert.deepEqual(mutationEvidence(evidence),evidence);
  for(const phase of ["backend","audit_outcome"])assert.ok(mutationEvidence({...evidence,phase,resourceEffects:"possible"}));
  for(const patch of [{schemaVersion:"v2"},{scope:"request"},{phase:"complete"},{resourceEffects:"possible"},{phase:"backend"},{intentId:null},{intentId:11111111111111111111111111111111n},{intentId:["a".repeat(32)]},{intentId:"a".repeat(33)}])assert.equal(mutationEvidence({...evidence,...patch}),undefined);
  for(const value of [null,undefined,{},"text"])assert.equal(mutationEvidence(value),undefined);
});
test("attempt evidence downloads allowlist fields and retain uncertainty",()=>{
  const error={status:503,code:"audit_unavailable",message:"secret-message",token:"secret-token",body:{error:{mutation:{...evidence,token:"secret-token"}}}};
  assert.deepEqual(errorMutationEvidence(error),evidence);
  for(const serialize of [queueEvidence,deleteEvidence]){
    const output=serialize({name:"orders",phase:"uncertain",requestId:"correlation",error});
    assert.deepEqual(JSON.parse(output).error.mutation,evidence);
    assert.equal(JSON.parse(output).phase,"uncertain");assert.equal(output.includes("secret-"),false);
  }
});
