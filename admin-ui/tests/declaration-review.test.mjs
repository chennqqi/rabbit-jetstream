import test from "node:test";
import assert from "node:assert/strict";
import {declarationReview} from "../src/declaration-review.mjs";
const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{retention:{maxMessages:9223372036854775807n}}};
const review={status:"available",document,diff:{queue:"q",changes:[{path:"spec.maxPriority",from:"omitted",to:"0",impact:"disruptive"}]}};
const state={name:"q",create:false,etag:'"9007199254740993"',preview:{plan:{queue:"q"},result:{queue:"q"},create_only:false,base_revision:'"9007199254740993"',declaration_review:review}};
test("declaration review preserves exact server values and does not modify draft state",()=>{
  const before=structuredClone(state),result=declarationReview(state);
  assert.equal(result.status,"available");assert.deepEqual(result.changes,review.diff.changes);assert.deepEqual(result.document,document);assert.deepEqual(state,before);
  assert.equal(declarationReview({...state,preview:{...state.preview,declaration_review:undefined}}).status,"missing");
  assert.deepEqual(declarationReview({...state,preview:{...state.preview,declaration_review:{...review,diff:{queue:"q",changes:[]}}}}).changes,[]);
});
test("declaration identity, revision, row types and conflicting statuses fail closed",()=>{
  for(const value of [null,{}, {...review,document:{...document,metadata:{name:"other"}}},{...review,diff:{queue:"other",changes:[]}},
    {...review,diff:{queue:"q",changes:[review.diff.changes[0],review.diff.changes[0]]}},
    {...review,diff:{queue:"q",changes:[{path:"x",from:0,to:"1",impact:"safe"}]}},{status:"create",document},
    {status:"unavailable",reason:"base_unrepresentable"},{status:"unavailable",reason:"target_unrepresentable",document}])
    assert.equal(declarationReview({...state,preview:{...state.preview,declaration_review:value}}).status,"invalid");
  assert.equal(declarationReview({...state,etag:'"2"'}).status,"invalid");
  assert.equal(declarationReview({...state,name:"other"}).status,"invalid");
});
test("creation and unavailable review never fabricate an empty edit diff",()=>{
  const create={...state,create:true,preview:{...state.preview,create_only:true,base_revision:"",declaration_review:{status:"create",document}}};
  assert.deepEqual(declarationReview(create),{status:"create",document});
  for(const reason of ["base_unrepresentable","base_identity_mismatch"])
    assert.equal(declarationReview({...state,preview:{...state.preview,declaration_review:{status:"unavailable",reason,document}}}).status,"unavailable");
  assert.deepEqual(declarationReview({...state,preview:{...state.preview,declaration_review:{status:"unavailable",reason:"target_unrepresentable"}}}),{status:"unavailable",reason:"target_unrepresentable",document:undefined});
});
