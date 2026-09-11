import test from "node:test";
import assert from "node:assert/strict";
import {dlqEvidence} from "../src/dlq-evidence.mjs";
import {parseJSON} from "../src/api.mjs";
const plan={queue:"orders",revision:"source-plan",deadLetter:{queue:"failed",stream:"RJSQ_failed",mechanism:"advisory-republish"},private:"secret"};
test("DLQ download binds original declaration evidence and preserves exact counters",()=>{
  const state={target:{phase:"available",readAt:"target-time",etag:'"8"',value:{queue:"failed",revision:"target-plan",stream:"RJSQ_failed",token:"secret"}},stream:{phase:"unobserved"},controller:{phase:"available",value:{instanceId:"one",dlqMoved:9007199254740993n,dlqFailed:0,lastError:"secret"}},token:"secret"};
  const text=dlqEvidence({plan,state,declarationETag:'"7"',declarationReadAt:"source-time"},"export-time"),value=parseJSON(text);
  assert.equal(value.schema,"rjs.dlq-diagnostic-evidence.v1");assert.equal(value.source.declarationETag,'"7"');assert.equal(value.source.readAt,"source-time");assert.equal(value.target.etag,'"8"');
  assert.equal(value.controller.value.dlqMoved,9007199254740993n);assert.equal(value.controller.value.dlqFailed,0);assert.equal(value.stream.phase,"unobserved");assert.equal(text.includes("secret"),false);
  assert.ok(value.limitations.some(text=>text.includes("all Queues")));
  assert.ok(value.limitations.some(text=>text.includes("not unique messages")&&text.includes("already-absent source message")&&text.includes("does not prove every transfer succeeded")));
});
test("nonavailable DLQ observations never export leftover values or raw failures",()=>{
  for(const phase of ["loading","missing","denied","unavailable","invalid","unobserved"]){
    const state={target:{phase,value:{queue:"secret"},error:{message:"secret"}}};
    const text=dlqEvidence({plan,state});assert.equal(text.includes("secret"),false);assert.equal(JSON.parse(text).target.phase,phase);assert.equal(JSON.parse(text).source.declarationETag,null);
  }
});

test("ignored advisory evidence exports reported exact counts but never fabricates absent counts",()=>{
  for(const ignored of [undefined,0,9007199254740993n]){
    const value=parseJSON(dlqEvidence({plan,state:{controller:{phase:"available",value:{dlqIgnored:ignored}}}}));
    assert.equal(value.controller.value.dlqIgnored,ignored);
    assert.equal(Object.hasOwn(value.controller.value,"dlqIgnored"),ignored!==undefined);
    assert.ok(value.limitations.some(text=>text.includes("not proof of message loss")));
  }
});
