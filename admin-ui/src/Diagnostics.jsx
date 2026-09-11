import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createDiagnostics} from "./diagnostics.mjs";
export function Diagnostics({api,language}){
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,model=useMemo(()=>createDiagnostics(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),job=state.job;
  useEffect(()=>()=>model.clear(),[model]);
  useEffect(()=>{if(job?.state!=="collecting")return;const timer=setTimeout(()=>void model.refresh(),1000);return()=>clearTimeout(timer);},[job,model]);
  return <section aria-label={t("Diagnostics","诊断包")}><h2>{t("Diagnostics","诊断包")}</h2>
    <p>{t("Creates a temporary metadata-only ZIP in this management process. It excludes messages, subscriptions, logs, environment, credentials and full Queue/Stream inventories. It is not a backup or atomic snapshot.","在当前管理进程内创建临时、仅元数据的 ZIP。不包含消息、订阅、日志、环境变量、凭据及完整 Queue/Stream 清单；它不是备份或原子快照。")}</p>
    <p>{t("One collection runs at a time, at most two jobs are retained, and each expires ten minutes after creation. Restart loses all jobs. Creation is explicit and is never retried automatically.","同一时间只采集一个任务，最多保留两个任务，每项自创建起十分钟过期。进程重启会丢失全部任务；创建必须显式触发且绝不自动重试。")}</p>
    <button type="button" disabled={state.phase==="creating"||job?.state==="collecting"} onClick={()=>void model.create()}>{state.phase==="creating"?t("Requesting…","正在请求…"):t("Create metadata bundle","创建元数据诊断包")}</button>
    {state.error&&<p role="alert">{t("The operation failed or its response was invalid. Review audit evidence before retrying a create request.","操作失败或响应无效。重新发起创建请求前请先检查审计证据。")}</p>}
    {job&&<section aria-label={t("Current diagnostic job","当前诊断任务")}><h3>{t("Current job","当前任务")}</h3><dl><dt>ID</dt><dd><code>{job.id}</code></dd><dt>{t("State","状态")}</dt><dd>{job.state}</dd><dt>{t("Created","创建时间")}</dt><dd><time dateTime={job.createdAt}>{job.createdAt}</time></dd><dt>{t("Expires","过期时间")}</dt><dd><time dateTime={job.expiresAt}>{job.expiresAt}</time></dd></dl>
      {job.state==="collecting"&&<><p role="status">{t("Collecting bounded metadata…","正在采集有界元数据…")}</p><button type="button" onClick={()=>void model.cancel()}>{t("Cancel collection","取消采集")}</button></>}
      {job.state==="partial"&&<p role="alert">{t("Partial bundle: one or more sources were omitted. Inspect the manifest before use.","部分完成：一个或多个来源已省略，使用前请检查清单。")}</p>}
      {job.manifest&&<table><caption>{t("Bundle manifest","诊断包清单")}</caption><thead><tr><th>{t("Source","来源")}</th><th>{t("File","文件")}</th><th>{t("Bytes","字节")}</th><th>{t("Result","结果")}</th></tr></thead><tbody>{job.manifest.entries.map(entry=><tr key={entry.source}><th scope="row">{entry.source}</th><td>{entry.file??"—"}</td><td>{entry.size??"—"}</td><td>{entry.error??t("Included","已包含")}</td></tr>)}</tbody></table>}
      {["ready","partial"].includes(job.state)&&<button type="button" onClick={()=>void model.download()}>{t("Prepare download","准备下载")}</button>}
      {state.download&&<p><a href={state.download.url} download={state.download.filename}>{t("Save diagnostic ZIP","保存诊断 ZIP")}</a> · {state.download.size} {t("bytes","字节")}. {t("Server completion does not prove the browser saved the file.","服务端写出完成不证明浏览器已保存文件。")}</p>}
      {["failed","cancelled","expired"].includes(job.state)&&<p role="alert">{t("No archive is available for this terminal state.","该终态没有可下载归档。")}</p>}
    </section>}
  </section>;
}
