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

function CreationDraft({model,language,onAnother,onPark}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  return <><QueueEditor model={model} language={language}/>
    {state.phase==="accepted"&&state.create&&<button onClick={onAnother}>{language==="zh"?"创建另一个 Queue":"Create another Queue"}</button>}
    {canParkCreation(state)&&<div>
      <button onClick={onPark}>{language==="zh"?"保留草稿并选择其他名称":"Keep draft and choose another name"}</button>
      <p>{language==="zh"?"尚未发出创建写入。保留此草稿为只读记录，新表单清空所有设置；不会重命名或覆盖资源。":"No creation write has been dispatched. Keep this draft as a read-only record and start with empty settings; no resource is renamed or overwritten."}</p>
    </div>}
  </>;
}

function ParkedCreations({models,language,onResume}) {
  if(!models?.length)return null;
  return <details className="parked-creations"><summary>{language==="zh"?"未提交的创建草稿（只读记录）":"Unsubmitted creation drafts (read-only records)"}</summary>
    <p>{language==="zh"?"保留原始 JSON 和预览证据，未发出写入。这些记录仅在会话内存中，不会自动复制到新草稿。":"Original JSON and preview evidence are retained without a write. These records are session-memory only and are not automatically copied into a new draft."}</p>
    {onResume&&<p>{language==="zh"?"可明确恢复草稿继续编辑；恢复会清除旧预览批准，必须重新预览。":"Explicitly resume a draft to edit it. Resuming clears previous preview approval and requires a fresh preview."}</p>}
    {models.map(model=>{const state=model.snapshot();return <div key={state.name}><h3>{state.name}</h3><pre className="declaration-json">{stringifyJSON({phase:state.phase,raw:state.raw,validation:state.validation,preview:state.preview,error:state.error})}</pre>{onResume&&canParkCreation(state)&&<button onClick={()=>onResume(model)}>{language==="zh"?"恢复创建草稿":"Resume creation draft"}: {state.name}</button>}</div>;})}
  </details>;
}

function CreationHistory({models,language}) {
  if(!models?.length)return null;
  return <details className="creation-history"><summary>{language==="zh"?"已接受的创建请求（会话内存）":"Accepted creation requests (session memory)"}</summary>
    <p>{language==="zh"?"历史响应不证明当前资源健康或收敛。清除会话或刷新页面会丢失这些证据，不会撤销资源。":"Historical responses do not prove current resource health or convergence. Clearing the session or reloading loses this evidence without reverting resources."}</p>
    {models.map(model=>{const state=model.snapshot();return <div key={state.name}><h3>{state.name}</h3><pre className="declaration-json">{stringifyJSON({requestId:state.requestId,responseRequestId:state.responseRequestId,returnedETag:state.returnedETag,document:state.draft,submittedPlan:state.submittedPlan,result:state.result})}</pre></div>;})}
  </details>;
}

export function QueueCreate({api,setup,prepare,language}) {
  const [form,setForm]=useState(setup.form),[model,setModel]=useState(setup.model),[error,setError]=useState(null);
  const [showBatch,setShowBatch]=useState(!!setup.batch&&setup.batchVisible!==false);
  const contract=useMemo(()=>createCreationContract(api),[api]);
  const rules=useSyncExternalStore(contract.subscribe,contract.snapshot);
  useEffect(()=>()=>contract.dispose(),[contract]);
  useEffect(()=>{if(!model)void contract.load();},[contract,model]);
  const choices=key=>rules.controls?.fields.find(field=>field.key===key)?.options??[];
  const nameInput=useRef(null),focusNext=useRef(false);
  useEffect(()=>{if(focusNext.current){if(model)document.getElementById("queue-draft")?.focus();else nameInput.current?.focus();focusNext.current=false;}},[model]);
  const t=(en,zh)=>language==="zh"?zh:en;
  function change(key,value){const next={...form,[key]:value};setup.form=next;setForm(next);setError(null);}
  function resume(previous){
    const dirty=Object.values(setup.form).some(Boolean);
    if(dirty&&!window.confirm(t("Discard the new form inputs and resume the retained draft?","丢弃当前新建表单输入并恢复保留草稿？")))return;
    if(resumeCreation(setup,previous,{discardForm:dirty})){focusNext.current=true;setModel(setup.model);setForm(setup.form);setError(null);}
  }
  const history=<><CreationHistory models={setup.completed} language={language}/><ParkedCreations models={setup.parked} language={language} onResume={model?undefined:resume}/><BatchHistory records={setup.batchHistory} language={language}/></>;
  function transition(action){if(action(setup)){contract.dispose();focusNext.current=true;setModel(null);setForm(setup.form);setError(null);}}
  if(model)return <><CreationDraft model={model} language={language} onAnother={()=>transition(startAnotherCreation)} onPark={()=>transition(parkCreation)}/>{history}</>;
  if(showBatch&&setup.batch)return <><button type="button" disabled={rules.phase==="loading"} onClick={()=>void contract.load()}>{t("Reload creation schema","重新读取创建 Schema")}</button><BatchExecution model={setup.batch} ready={rules.phase==="ready"} prepare={document=>prepare(contract.prepareImport(document))} language={language} onBack={()=>{setup.batchVisible=false;setShowBatch(false);}} onArchive={()=>{
    if(!window.confirm(t("Archive this batch as read-only evidence? Unsubmitted items will not run. Existing resources and retained names remain unchanged.","将此批次归档为只读证据？未提交项不会执行，已有资源及保留名称保持不变。")))return;
    if(archiveCreationBatch(setup,{confirmed:true}))setShowBatch(false);
  }}/>{history}</>;
  return <><section className="queue-list" aria-labelledby="create-heading">
    <h2 id="create-heading">{t("Create Queue","创建 Queue")}</h2>
    {setup.batch&&<button type="button" onClick={()=>{setup.batchVisible=true;setShowBatch(true);}}>{t("Resume retained batch","恢复保留批次")}</button>}
    <p>{t("Choose every deployment-sensitive setting explicitly. Replica count is requested configuration, not proof of cluster capacity or production qualification. One replica has no replica fault tolerance; memory storage does not survive process loss.","请明确选择部署相关设置。副本数只是请求配置，不证明集群容量或生产资格。单副本不具备副本容错，内存存储不能承受进程丢失。")}</p>
    <button type="button" disabled={rules.phase==="loading"} onClick={()=>void contract.load()}>{t("Reload creation schema","重新读取创建 Schema")}</button>
    {rules.phase==="loading"&&<p role="status">{t("Reading creation rules; entered values are retained.","正在读取创建规则，已填内容保留。")}</p>}
    {rules.phase==="error"&&<p role="alert">{t("Creation schema unavailable or changed. Inputs are retained; reload before preparing a draft.","创建 Schema 不可用或已变化。输入已保留，准备草稿前请重新读取。")}</p>}
    <form onSubmit={event=>{event.preventDefault();try{const next=prepare(contract.prepare(form));setup.model=next;setModel(next);}catch(error){setError(error.code??"invalid");}}}>
      <label htmlFor="create-name">{t("New Queue name","新 Queue 名称")}</label><input ref={nameInput} id="create-name" required value={form.name} onChange={event=>change("name",event.target.value)} />
      <label htmlFor="create-subjects">{t("Subjects (one per line)","Subject（每行一个）")}</label><textarea id="create-subjects" required value={form.subjects} onChange={event=>change("subjects",event.target.value)} />
      <label htmlFor="create-replicas">{t("Requested replicas","请求副本数")}</label><select id="create-replicas" required disabled={rules.phase!=="ready"} value={form.replicas} onChange={event=>change("replicas",event.target.value)}><option value="">{t("Select explicitly","请明确选择")}</option>{form.replicas&&!choices("replicas").includes(form.replicas)&&<option value={form.replicas}>{form.replicas}</option>}{choices("replicas").map(n=><option key={n} value={n}>{n}</option>)}</select>
      <label htmlFor="create-storage">{t("Storage type","存储类型")}</label><select id="create-storage" required disabled={rules.phase!=="ready"} value={form.storage} onChange={event=>change("storage",event.target.value)}><option value="">{t("Select explicitly","请明确选择")}</option>{form.storage&&!choices("storage").includes(form.storage)&&<option value={form.storage}>{form.storage}</option>}{choices("storage").map(value=><option key={value} value={value}>{value==="file"?"File":value==="memory"?"Memory":value}</option>)}</select>
      <label htmlFor="create-max">{t("Maximum stored messages","最大存储消息数")}</label><input id="create-max" required inputMode="numeric" value={form.maxMessages} onChange={event=>change("maxMessages",event.target.value)} />
      <QueueTemplates form={form} change={change} controls={rules.phase==="ready"?rules.controls:null} language={language}/>
      <p>{t("Additional settings can be edited in the JSON draft. The server resolves defaults and validates the full plan during preview. An existing name will never be overwritten by this creation flow.","其他设置可在 JSON 草稿中编辑，服务端预览时解析默认值并验证完整计划。此创建流程绝不覆盖同名资源。")}</p>
      {error&&<p role="alert">{error==="retained-name"?t("This name already has a retained creation request in this session. Choose a different name; the previous request will not be reused or overwritten.","此名称在本会话已有保留的创建请求，请选择其他名称。不会复用或覆盖之前的请求。"):t("Check the name, unique Subjects, explicit replicas/storage and positive int64 message limit.","请检查名称、唯一 Subject、明确的副本/存储选择及正 int64 消息数上限。")}</p>}
      {error&&form.templateId&&<p role="alert">{t("Also check the template: priority must be 1–255; a DLQ target must be a valid Queue name different from this Queue. Target existence and compatibility require server preview.","另请检查模板：优先级必须为 1–255；DLQ 目标须为有效且不同于当前 Queue 的名称。目标存在性及兼容性须经服务端预览核验。")}</p>}
      <button type="submit" disabled={rules.phase!=="ready"}>{t("Prepare creation draft","准备创建草稿")}</button>
    </form>
    <QueueImport ready={rules.phase==="ready"} language={language} onPrepare={document=>{
      if(Object.values(setup.form).some(Boolean)&&!window.confirm(t("Discard the new form inputs and prepare the imported draft?","放弃当前新建表单输入并准备导入草稿？")))return;
      const next=prepare(contract.prepareImport(document));setup.model=next;focusNext.current=true;setModel(next);
    }}/>
    <BatchImport api={api} binding={rules.phase==="ready"?rules.binding:null} language={language} retained={!!setup.batch} onStart={planning=>{
      if(setup.batch||rules.phase!=="ready"||planning.binding!==rules.binding)return;
      setup.batch=createBatchExecution(planning);setup.batchVisible=true;setShowBatch(true);
    }}/>
  </section>{history}</>;
}
