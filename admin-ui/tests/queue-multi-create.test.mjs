import {test} from "node:test";
import assert from "node:assert/strict";
import {emptyCreationForm,startAnotherCreation,retainCreationDraft,canParkCreation,parkCreation} from "../src/queue-create.mjs";

test("another creation requires accepted create, resets all selections, and retains its model once",()=>{
  const first={snapshot:()=>({phase:"accepted",create:true,name:"first",requestId:"receipt"})};
  const form={name:"first",subjects:"first.events",replicas:"3",storage:"file",maxMessages:"123"};
  const setup={model:first,form};
  assert.equal(startAnotherCreation(setup),true);assert.deepEqual(setup.form,emptyCreationForm());
  assert.equal(setup.model,null);assert.deepEqual(setup.completed,[first]);
  assert.equal(startAnotherCreation(setup),false);assert.equal(setup.completed.length,1);
  assert.equal(form.name,"first");assert.equal(first.snapshot().requestId,"receipt");
  const second={snapshot:()=>({phase:"accepted",create:true,name:"second"})};
  setup.model=second;assert.equal(startAnotherCreation(setup),true);assert.deepEqual(setup.completed,[first,second]);
  setup.form.name="new";assert.equal(emptyCreationForm().name,"");
});

test("editing, conflict, pending, uncertain and accepted update cannot leave through another creation",()=>{
  for(const [phase,create] of [["editing",true],["review",true],["conflict",true],["submitting",true],["uncertain",true],["inspecting",true],["accepted",false]]){
    const model={snapshot:()=>({phase,create})},form={name:"keep"},setup={model,form};
    assert.equal(startAnotherCreation(setup),false);assert.equal(setup.model,model);assert.equal(setup.form,form);assert.equal(setup.completed,undefined);
  }
});

test("session creation registry never silently reuses or overwrites an earlier name",()=>{
  const drafts=new Map();let count=0;const create=document=>({document,id:++count});
  const document={metadata:{name:"q"}},first=retainCreationDraft(drafts,document,create);
  assert.equal(drafts.get("create:q"),first);
  assert.throws(()=>retainCreationDraft(drafts,document,create),{code:"retained-name"});
  assert.equal(count,1);assert.equal(drafts.get("create:q"),first);
  const second=retainCreationDraft(drafts,{metadata:{name:"other"}},create);
  assert.notEqual(first,second);assert.equal(drafts.size,2);
  assert.throws(()=>retainCreationDraft(drafts,{metadata:{name:"failed"}},()=>{throw new Error("failed");}));
  assert.equal(drafts.has("create:failed"),false);
});

test("only pre-write creation states can be parked without losing invalid JSON or preview evidence",()=>{
  for(const phase of ["editing","review","blocked","preview-error","conflict","denied"]){
    const state={create:true,phase,raw:"{broken",validation:"invalid-draft",error:{status:409}},model={snapshot:()=>state};
    const setup={model,form:{name:"old"},completed:[]};
    assert.equal(canParkCreation(state),true);assert.equal(parkCreation(setup),true);
    assert.equal(setup.model,null);assert.deepEqual(setup.form,emptyCreationForm());assert.deepEqual(setup.parked,[model]);
    assert.equal(setup.parked[0].snapshot().raw,"{broken");assert.equal(setup.parked[0].snapshot().error.status,409);
    assert.equal(parkCreation(setup),false);assert.equal(setup.parked.length,1);assert.deepEqual(setup.completed,[]);
  }
});

test("pending, dispatched, accepted, update and missing states cannot bypass write protection by parking",()=>{
  for(const state of [undefined,...["idle","loading","previewing","submitting","uncertain","inspecting","accepted"].map(phase=>({create:true,phase})),
    {phase:"conflict",create:true,requestId:"previous-dispatch"},{phase:"editing",create:false}]){
    const model={snapshot:()=>state},form={name:"keep"},setup={model,form};
    assert.equal(canParkCreation(state),false);assert.equal(parkCreation(setup),false);
    assert.equal(setup.model,model);assert.equal(setup.form,form);assert.equal(setup.parked,undefined);
  }
});
