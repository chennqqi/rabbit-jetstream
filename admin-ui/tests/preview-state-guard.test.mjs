import test from "node:test";
import assert from "node:assert/strict";
import {createQueueEditor} from "../src/queue-editor.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{subjects:["q"],replicas:1}};
const operation={resource:"stream",name:"S",action:"update",impact:"safe",blocked:false,changes:[]};
const valid=()=>({plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false,operations:[structuredClone(operation)]},create_only:false,base_revision:'"7"'});
const invalidCases={
  missing:preview=>{delete preview.result.operations;},
  malformed:preview=>{preview.result.operations=[{name:"S"}];},
  duplicate:preview=>{preview.result.operations.push(structuredClone(operation));},
  hiddenBlock:preview=>{preview.result.operations[0].blocked=true;},
  inventedBlock:preview=>{preview.result.blocked=true;preview.result.status="blocked";},
  wrongDeclaration:preview=>{preview.declaration_review={status:"available",document:{...document,metadata:{name:"other"}},diff:{queue:"q",changes:[]}};},
  wrongMode:preview=>{preview.declaration_review={status:"create",document};},
};

test("runtime editor rejects incomplete/contradictory previews before any direct apply",async()=>{
  for(const [name,alter] of Object.entries(invalidCases)){
    let puts=0,current=valid();alter(current);
    const editor=createQueueEditor({clearToken(){},async request(path,options){
      if(options?.method==="PUT"){puts++;return {body:{queue:"q",status:"ready",blocked:false},headers:new Headers()};}
      return options?{body:current}:{body:{queue:"q",document},headers:new Headers({ETag:'"7"'})};
    }});
    await editor.load("q");await editor.preview();
    assert.equal(editor.snapshot().phase,"preview-error",name);assert.equal(editor.snapshot().preview,undefined,name);
    assert.equal(editor.snapshot().error.code,"invalid_preview",name);assert.equal(editor.snapshot().etag,'"7"');
    await assert.rejects(editor.apply());assert.equal(puts,0,name);
    current=valid();await editor.preview();assert.equal(editor.snapshot().phase,"review",name);
    await editor.apply();assert.equal(puts,1,name);
  }
});

test("session controller invalidates prior approval on malformed re-preview and requires fresh confirmation",async()=>{
  let current=valid(),puts=0;
  const model=createQueueDraft({clearToken(){},async request(path,options){
    if(options?.method==="PUT"){puts++;return {body:{queue:"q",status:"ready",blocked:false},headers:new Headers()};}
    return options?{body:current}:{body:{queue:"q",document},headers:new Headers({ETag:'"7"'})};
  }},"q");
  await model.load();await model.preview();model.confirmApply(true);
  current=valid();delete current.result.operations;await model.preview();
  model.confirmApply(true);await model.apply();
  assert.equal(model.snapshot().applyConfirmed,false);assert.equal(puts,0);assert.equal(model.snapshot().phase,"preview-error");
  current=valid();await model.preview();await model.apply();assert.equal(puts,0);
  model.confirmApply(true);await model.apply();assert.equal(puts,1);
});
