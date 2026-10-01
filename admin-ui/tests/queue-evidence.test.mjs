import test from "node:test";
import assert from "node:assert/strict";
import {queueEvidence} from "../src/queue-evidence.mjs";
import {parseJSON} from "../src/api.mjs";

test("editor export preserves exact drafts, revisions, receipts and partial observations",()=>{
  const document={metadata:{name:"orders"},spec:{max:9223372036854775807n}};
  const state={name:"orders",phase:"uncertain",create:false,etag:'"9007199254740993"',
    base:document,draft:document,raw:' {"invalid": ',validation:"invalid-draft",mergeRaw:"unfinished",
    requestId:"request",submittedPlan:{queue:"orders"},error:{status:503,requestId:"response"},
    acceptedOperations:[{requestId:"old",document,originalETag:'"8"',returnedETag:'"9"',result:{status:"ready"}}],
    inspection:{declaration:{status:"available",body:{document},readAt:"first"},
      consumers:{status:"unavailable",error:{status:403}},
      audit:{status:"available",windows:[{readAt:"second",body:{nextBefore:9007199254740993n,items:[]}}],olderError:{status:503}}}};
  const before=structuredClone(state),data=parseJSON(queueEvidence(state,"export-time"));
  assert.deepEqual(state,before);
  assert.equal(data.schema,"rjs.queue-editor-evidence.v1");assert.equal(data.exportedAt,"export-time");
  for(const key of ["phase","raw","mergeRaw","requestId","submittedPlan","acceptedOperations","inspection"])assert.deepEqual(data[key],state[key]);
  assert.equal(data.document.spec.max,9223372036854775807n);assert.equal(data.originalETag,state.etag);
  assert.ok(data.limitations.some(value=>value.includes("Do not replay")));
});

test("export excludes session/transport and arbitrary error fields, retains accepted response evidence",()=>{
  const secret="DO_NOT_EXPORT_TRANSPORT_SECRET";
  const state={name:"q",phase:"accepted",create:true,raw:"{}",draft:{},requestId:"r",responseRequestId:"response",
    returnedETag:'"1"',result:{status:"ready"},token:secret,session:{token:secret},headers:{Authorization:secret},
    error:{status:503,message:secret,body:{token:secret},stack:secret},
    inspection:{audit:{status:"unavailable",error:{status:403,message:secret,body:secret}},session:secret},
    acceptedOperations:[{requestId:"old",token:secret}]};
  const content=queueEvidence(state),data=parseJSON(content);
  assert.equal(content.includes(secret),false);assert.equal(data.createOnly,true);
  assert.equal(data.responseRequestId,"response");assert.equal(data.returnedETag,'"1"');
  assert.deepEqual(data.result,{status:"ready"});assert.deepEqual(data.acceptedOperations,[{requestId:"old"}]);
});
