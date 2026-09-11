import test from "node:test";
import assert from "node:assert/strict";
import {archiveQueueEditors,canArchiveEditor} from "../src/editor-handoff.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";

const document={metadata:{name:"orders"},spec:{max:9007199254740993n}};
function fixture(){
  const calls=[];let clears=0;
  const api={clearToken(){clears++;},async request(path,options){calls.push({path,options});return {body:{queue:"orders",document},headers:new Headers({ETag:'"7"'})};}};
  return {api,calls,clears:()=>clears};
}
test("handoff preserves invalid raw draft and credentials while disabling old editor",async()=>{
  const f=fixture(),model=createQueueDraft(f.api,"orders");await model.load();model.edit("{invalid unsaved");
  const drafts=new Map([["orders",model],["other",{unchanged:true}]]),archives=new Map(),creation={model:null,form:{name:"unrelated"}};
  assert.equal(archiveQueueEditors({drafts,archives,creation,name:"orders"}),false);
  assert.equal(archiveQueueEditors({drafts,archives,creation,name:"orders",confirmed:true}),true);
  const saved=archives.get("orders")[0].state;
  assert.equal(saved.raw,"{invalid unsaved");assert.equal(saved.base.spec.max,9007199254740993n);assert.equal(saved.etag,'"7"');
  assert.equal(drafts.has("orders"),false);assert.equal(drafts.get("other").unchanged,true);assert.equal(creation.form.name,"unrelated");
  assert.equal(model.snapshot().phase,"archived");model.edit("changed");await model.load();await model.preview();model.confirmApply(true);await model.apply();
  assert.equal(f.calls.length,1);assert.equal(f.clears(),0);assert.equal(model.snapshot().raw,"{invalid unsaved");
});
test("every pending and uncertain phase is rejected before either Queue model changes",()=>{
  for(const phase of ["loading","previewing","submitting","uncertain","inspecting","reading-conflict","reading-next","archived"]){
    let count=0;
    const model={snapshot:()=>({phase:"editing"}),archive(){count++;}},blocked={snapshot:()=>({phase}),archive(){count++;}};
    const drafts=new Map([["orders",model],["create:orders",blocked]]),archives=new Map(),creation={model:blocked};
    assert.equal(canArchiveEditor({phase}),false);
    assert.equal(archiveQueueEditors({drafts,archives,creation,name:"orders",confirmed:true}),false);
    assert.equal(count,0);assert.equal(drafts.size,2);assert.equal(archives.size,0);assert.equal(creation.model,blocked);
  }
});
test("accepted/create evidence moves out of all creation references without changing unrelated models",()=>{
  let archived=0;
  const accepted={snapshot:()=>({phase:"accepted",name:"orders",create:true,requestId:"receipt",acceptedOperations:[{requestId:"previous"}]}),archive(){archived++;}};
  const other={snapshot:()=>({name:"other"})},drafts=new Map([["create:orders",accepted],["other",other]]),archives=new Map();
  const creation={model:accepted,form:{name:"orders"},parked:[accepted,other],completed:[accepted,other]};
  assert.equal(archiveQueueEditors({drafts,archives,creation,name:"orders",confirmed:true}),true);
  assert.equal(archived,1);assert.equal(creation.model,null);assert.equal(creation.form.name,"");
  assert.deepEqual(creation.parked,[other]);assert.deepEqual(creation.completed,[other]);
  assert.equal(archives.get("orders")[0].state.requestId,"receipt");assert.equal(archives.get("orders")[0].state.acceptedOperations[0].requestId,"previous");
});
test("real archived creation cannot be resumed or submit through retained reference",()=>{
  const f=fixture(),model=createQueueDraft(f.api,"orders",{document});
  model.archive();model.confirmApply(true);model.edit("changed");assert.equal(model.snapshot().phase,"archived");assert.equal(model.snapshot().applyConfirmed,false);assert.equal(f.clears(),0);
});
