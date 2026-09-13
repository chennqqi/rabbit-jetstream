import React,{useEffect,useMemo,useRef,useState,useSyncExternalStore} from "react";
import {startAnotherCreation,canParkCreation,parkCreation,resumeCreation,archiveCreationBatch} from "./queue-create.mjs";
import {QueueEditor} from "./QueueEditor.jsx";
import {stringifyJSON} from "./api.mjs";
import {createCreationContract} from "./creation-contract.mjs";
import {QueueImport} from "./QueueImport.jsx";
import {BatchImport} from "./BatchImport.jsx";
import {BatchExecution} from "./BatchExecution.jsx";
import {createBatchExecution} from "./batch-execution.mjs";
import {BatchHistory} from "./BatchHistory.jsx";
import {QueueTemplates} from "./QueueTemplates.jsx";
import {useDialog,DialogHost} from "./dialog.jsx";
import {queueCreateLabels} from "./queue-create-labels.mjs";

function CreationDraft({model,language,onAnother,onPark}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  const text=queueCreateLabels(language);
  return <><QueueEditor model={model} language={language}/>
    {state.phase==="accepted"&&state.create&&<button onClick={onAnother}>{text.another}</button>}
    {canParkCreation(state)&&<div>
      <button onClick={onPark}>{text.park}</button><p>{text.parkNote}</p>
    </div>}
  </>;
}

function ParkedCreations({models,language,onResume}) {
  if(!models?.length)return null;
  const text=queueCreateLabels(language);
  return <details className="parked-creations"><summary>{text.parked}</summary><p>{text.parkedNote}</p>
    {onResume&&<p>{text.resumeNote}</p>}
    {models.map(model=>{const state=model.snapshot();return <div key={state.name}><h3>{state.name}</h3><pre className="declaration-json">{stringifyJSON({phase:state.phase,raw:state.raw,validation:state.validation,preview:state.preview,error:state.error})}</pre>{onResume&&canParkCreation(state)&&<button onClick={()=>onResume(model)}>{text.resumeDraft}: {state.name}</button>}</div>;})}
  </details>;
}

function CreationHistory({models,language}) {
  if(!models?.length)return null;
  const text=queueCreateLabels(language);
  return <details className="creation-history"><summary>{text.history}</summary><p>{text.historyNote}</p>
    {models.map(model=>{const state=model.snapshot();return <div key={state.name}><h3>{state.name}</h3><pre className="declaration-json">{stringifyJSON({requestId:state.requestId,responseRequestId:state.responseRequestId,returnedETag:state.returnedETag,document:state.draft,submittedPlan:state.submittedPlan,result:state.result})}</pre></div>;})}
  </details>;
}

export function QueueCreate({api,setup,prepare,language}) {
  const [form,setForm]=useState(setup.form),[model,setModel]=useState(setup.model),[error,setError]=useState(null);
  const [showBatch,setShowBatch]=useState(!!setup.batch&&setup.batchVisible!==false);
  const dialog=useDialog();
  const contract=useMemo(()=>createCreationContract(api),[api]);
  const rules=useSyncExternalStore(contract.subscribe,contract.snapshot);
  useEffect(()=>()=>contract.dispose(),[contract]);
  useEffect(()=>{if(!model)void contract.load();},[contract,model]);
  const choices=key=>rules.controls?.fields.find(field=>field.key===key)?.options??[];
  const nameInput=useRef(null),focusNext=useRef(false);
  useEffect(()=>{if(focusNext.current){if(model)document.getElementById("queue-draft")?.focus();else nameInput.current?.focus();focusNext.current=false;}},[model]);
  const text=queueCreateLabels(language);
  const host=<DialogHost dialog={dialog.dialog} language={language}/>;
  function change(key,value){const next={...form,[key]:value};setup.form=next;setForm(next);setError(null);}
  async function resume(previous){
    const dirty=Object.values(setup.form).some(Boolean);
    if(dirty&&!await dialog.confirm(text.discardResume,{tone:"danger"}))return;
    if(resumeCreation(setup,previous,{discardForm:dirty})){focusNext.current=true;setModel(setup.model);setForm(setup.form);setError(null);}
  }
  const history=<><CreationHistory models={setup.completed} language={language}/><ParkedCreations models={setup.parked} language={language} onResume={model?undefined:resume}/><BatchHistory records={setup.batchHistory} language={language}/></>;
  function transition(action){if(action(setup)){contract.dispose();focusNext.current=true;setModel(null);setForm(setup.form);setError(null);}}
  if(model)return <><CreationDraft model={model} language={language} onAnother={()=>transition(startAnotherCreation)} onPark={()=>transition(parkCreation)}/>{history}{host}</>;
  if(showBatch&&setup.batch)return <><button type="button" disabled={rules.phase==="loading"} onClick={()=>void contract.load()}>{text.reloadSchema}</button><BatchExecution model={setup.batch} ready={rules.phase==="ready"} prepare={document=>prepare(contract.prepareImport(document))} language={language} onBack={()=>{setup.batchVisible=false;setShowBatch(false);}} onArchive={async()=>{
    if(!await dialog.confirm(text.archiveBatch))return;
    if(archiveCreationBatch(setup,{confirmed:true}))setShowBatch(false);
  }}/>{history}{host}</>;
  return <><section className="queue-list" aria-labelledby="create-heading">
    <h2 id="create-heading">{text.title}</h2>
    {setup.batch&&<button type="button" onClick={()=>{setup.batchVisible=true;setShowBatch(true);}}>{text.resumeBatch}</button>}
    <p>{text.deploymentNote}</p>
    <button type="button" disabled={rules.phase==="loading"} onClick={()=>void contract.load()}>{text.reloadSchema}</button>
    {rules.phase==="loading"&&<p role="status">{text.loading}</p>}
    {rules.phase==="error"&&<p role="alert">{text.schemaError}</p>}
    <form onSubmit={event=>{event.preventDefault();try{const next=prepare(contract.prepare(form));setup.model=next;setModel(next);}catch(error){setError(error.code??"invalid");}}}>
      <label htmlFor="create-name">{text.name}</label><input ref={nameInput} id="create-name" required value={form.name} onChange={event=>change("name",event.target.value)} />
      <label htmlFor="create-subjects">{text.subjects}</label><textarea id="create-subjects" required value={form.subjects} onChange={event=>change("subjects",event.target.value)} />
      <label htmlFor="create-replicas">{text.replicas}</label><select id="create-replicas" required disabled={rules.phase!=="ready"} value={form.replicas} onChange={event=>change("replicas",event.target.value)}><option value="">{text.select}</option>{form.replicas&&!choices("replicas").includes(form.replicas)&&<option value={form.replicas}>{form.replicas}</option>}{choices("replicas").map(n=><option key={n} value={n}>{n}</option>)}</select>
      <label htmlFor="create-storage">{text.storage}</label><select id="create-storage" required disabled={rules.phase!=="ready"} value={form.storage} onChange={event=>change("storage",event.target.value)}><option value="">{text.select}</option>{form.storage&&!choices("storage").includes(form.storage)&&<option value={form.storage}>{form.storage}</option>}{choices("storage").map(value=><option key={value} value={value}>{value==="file"?"File":value==="memory"?"Memory":value}</option>)}</select>
      <label htmlFor="create-max">{text.maxMessages}</label><input id="create-max" required inputMode="numeric" value={form.maxMessages} onChange={event=>change("maxMessages",event.target.value)} />
      <QueueTemplates form={form} change={change} controls={rules.phase==="ready"?rules.controls:null} language={language}/>
      <p>{text.additional}</p>
      {error&&<p role="alert">{error==="retained-name"?text.retainedName:text.invalid}</p>}
      {error&&form.templateId&&<p role="alert">{text.templateError}</p>}
      <button type="submit" disabled={rules.phase!=="ready"}>{text.prepare}</button>
    </form>
    <QueueImport ready={rules.phase==="ready"} language={language} onPrepare={async document=>{
      if(Object.values(setup.form).some(Boolean)&&!await dialog.confirm(text.discardImport,{tone:"danger"}))return;
      const next=prepare(contract.prepareImport(document));setup.model=next;focusNext.current=true;setModel(next);
    }}/>
    <BatchImport api={api} binding={rules.phase==="ready"?rules.binding:null} language={language} retained={!!setup.batch} onStart={planning=>{
      if(setup.batch||rules.phase!=="ready"||planning.binding!==rules.binding)return;
      setup.batch=createBatchExecution(planning);setup.batchVisible=true;setShowBatch(true);
    }}/>
  </section>{history}{host}</>;
}
