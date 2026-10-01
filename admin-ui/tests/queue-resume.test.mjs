import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {emptyCreationForm,parkCreation,resumeCreation} from "../src/queue-create.mjs";

test("resume preserves exact or invalid raw drafts, discards old approval and sends no request",async()=>{
  let calls=0;
  const api={clearToken(){},async request(){calls++;return {body:{plan:{queue:"q"},result:{queue:"q",blocked:false,status:"ready", operations: []},create_only:true}};}};
  const model=createQueueDraft(api,"q",{document:{metadata:{name:"q"},spec:{max:9007199254740993n}}});
  await model.preview();model.confirmApply(true);
  const setup={model,form:emptyCreationForm()};parkCreation(setup);
  const original=model.snapshot().raw;
  setup.form.name="new";assert.equal(resumeCreation(setup,model),false);assert.equal(model.snapshot().applyConfirmed,true);
  assert.equal(resumeCreation(setup,model,{discardForm:true}),true);
  assert.equal(model.snapshot().raw,original);assert.equal(model.snapshot().phase,"editing");assert.equal(model.snapshot().preview,undefined);assert.equal(model.snapshot().applyConfirmed,false);
  await model.apply();assert.equal(calls,1);assert.equal(setup.parked.length,0);
  model.edit("{broken");parkCreation(setup);assert.equal(resumeCreation(setup,model),true);
  assert.equal(model.snapshot().raw,"{broken");assert.equal(model.snapshot().validation,"invalid-draft");assert.equal(calls,1);
});

test("resume cannot replace active editor or import unowned/dispatched models",()=>{
  const model={snapshot:()=>({create:true,phase:"uncertain",requestId:"sent"}),edit(){throw new Error("must not edit");}};
  assert.equal(resumeCreation({model:null,parked:[model],form:emptyCreationForm()},model),false);
  assert.equal(resumeCreation({model:null,parked:[],form:emptyCreationForm()},model),false);
  assert.equal(resumeCreation({model:{},parked:[model],form:emptyCreationForm()},model,{discardForm:true}),false);
});
