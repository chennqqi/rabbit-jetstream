import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createQueueImport} from "./queue-import.mjs";

export function QueueImport({ready,onPrepare,language}){
  const model=useMemo(()=>createQueueImport(),[]),state=useSyncExternalStore(model.subscribe,model.snapshot);
  const [confirmed,setConfirmed]=useState(false),[failure,setFailure]=useState(null);
  useEffect(()=>()=>model.clear(),[model]);
  const t=(en,zh)=>language==="zh"?zh:en;
  return <section className="queue-import" aria-label={t("Import Queue declaration","导入 Queue 声明")}>
    <h3>{t("Import a single Queue JSON file","导入单个 Queue JSON 文件")}</h3>
    <p>{t("Reads locally, up to 1 MiB UTF-8 JSON. No request is sent by file selection. This prepares a create-only draft, not a restore or overwrite. Review labels and deployment settings; DLQ dependencies are not included and must be checked during server preview.","在本地读取，最多 1 MiB UTF-8 JSON，选择文件不发送请求。仅准备创建草稿，不是恢复或覆盖。请审阅标签和部署设置；文件不包含 DLQ 依赖，须在服务端预览时检查。")}</p>
    <label>{t("Queue JSON file","Queue JSON 文件")}<input type="file" accept="application/json,.json" onChange={event=>{const file=event.target.files?.[0];setConfirmed(false);setFailure(null);model.clear();if(file)void model.read(file);event.target.value="";}}/></label>
    {state.phase==="loading"&&<><p role="status">{t("Reading local file…","正在读取本地文件…")}</p><button type="button" onClick={()=>model.clear()}>{t("Cancel file read","取消文件读取")}</button></>}
    {state.phase==="error"&&<p role="alert">{state.failure==="size"?t("Choose a nonempty JSON file no larger than 1 MiB.","请选择非空且不超过 1 MiB 的 JSON 文件。"):t("Invalid UTF-8 JSON or unsupported single-Queue document/version. No draft was changed.","UTF-8 JSON 无效，或不是受支持的单 Queue 文档／版本。未修改草稿。")}</p>}
    {state.document&&<>
      <p>{t("Loaded file: ","已读取文件：")}{state.filename} · {state.bytes} {t("bytes","字节")}</p>
      <p>Queue: {state.document.metadata.name} · {t("Labels: ","标签数：")}{Object.keys(state.document.metadata.labels??{}).length}</p>
      <details><summary>{t("Review imported document","审阅导入文档")}</summary><pre className="declaration-json">{state.raw}</pre></details>
      <label><input type="checkbox" checked={confirmed} onChange={event=>setConfirmed(event.target.checked)}/>{t("I reviewed this file and want to prepare a new Queue draft; preview and apply are separate actions.","我已审阅文件，希望准备新 Queue 草稿；预览和提交是独立操作。")}</label>
      <button type="button" disabled={!ready||!confirmed} onClick={()=>{try{setFailure(null);onPrepare(state.document);}catch(error){setFailure(error.code??"invalid");}}}>{t("Prepare imported creation draft","准备导入创建草稿")}</button>
      {!ready&&<p>{t("Reload the creation schema before preparing this file.","准备此文件前，请重新读取创建 Schema。")}</p>}
      {failure&&<p role="alert">{failure==="retained-name"?t("This name has a retained request in this session; it will not be overwritten.","本会话已保留此名称的请求，不会覆盖。"):t("Cannot prepare this draft. Check current schema and supported replicas/storage; the file is retained for review.","无法准备草稿。请检查当前 Schema 和受支持的副本／存储设置；文件仍保留供审阅。")}</p>}
    </>}
  </section>;
}
