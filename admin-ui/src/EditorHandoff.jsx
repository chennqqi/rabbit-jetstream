import React,{useSyncExternalStore} from "react";
import {canArchiveEditor} from "./editor-handoff.mjs";
import {QueueEvidence} from "./QueueEvidence.jsx";
import {stringifyJSON} from "./api.mjs";
import {deletionLabels} from "./deletion-labels.mjs";

function HandoffEntry({model,language}){
  const state=useSyncExternalStore(model.subscribe,model.snapshot),zh=language==="zh";
  return <div><p>{zh?"编辑器状态":"Editor state"}: {state.phase}</p>
    {!canArchiveEditor(state)&&<p role="alert">{zh?"操作进行中或结果未知，不能交接。请返回编辑器处理并保存证据。":"Operation pending or outcome unknown; handoff is blocked. Return to the editor to inspect and save evidence."}</p>}
    <QueueEvidence state={state} language={language}/></div>;
}
export function EditorHandoff({entries,language,onArchive}){
  const zh=language==="zh",accessible=deletionLabels(language);
  useSyncExternalStore(listener=>{const stops=entries.map(({model})=>model.subscribe(listener));return()=>stops.forEach(stop=>stop());},()=>entries.map(({model})=>model.snapshot().phase).join(","));
  const blocked=entries.some(({model})=>!canArchiveEditor(model.snapshot()));
  if(!entries.length)return null;
  return <section className="queue-list" aria-label={accessible.handoff}>
    <h3>{zh?"结束此 Queue 的编辑审阅":"End this Queue's editor review"}</h3>
    <p>{zh?"明确结束编辑并保留只读证据后，可重新读取删除预检。未提交的修改不会应用，不会清除登录或其他 Queue 草稿；此操作本身不会删除资源。":"Explicitly end editing and retain read-only evidence before reading a fresh deletion preflight. Unsubmitted changes are not applied. Login and other Queue drafts are unchanged; this action does not delete resources."}</p>
    {entries.map(({key,model})=><HandoffEntry key={key} model={model} language={language}/>)}
    <button disabled={blocked} onClick={()=>{if(window.confirm(zh?"结束此 Queue 的编辑，保留证据为只读记录？不会提交草稿或删除资源。":"End this Queue's editing and retain evidence as read-only records? This does not apply drafts or delete resources."))onArchive();}}>{zh?"归档此 Queue 编辑并允许重新预检":"Archive this Queue editor for fresh preflight"}</button>
  </section>;
}
export function ArchivedEditors({records,language}){
  if(!records?.length)return null;
  return <details className="queue-list"><summary>{language==="zh"?"已归档编辑证据":"Archived editor evidence"}: {records.length}</summary>
    {records.map((record,index)=><section key={index}><p>{record.key} · {record.archivedAt}</p><pre className="declaration-json">{stringifyJSON({phase:record.state.phase,raw:record.state.raw,mergeRaw:record.state.mergeRaw,requestId:record.state.requestId})}</pre><QueueEvidence state={record.state} language={language}/></section>)}
  </details>;
}
