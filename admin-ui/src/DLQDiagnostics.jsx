import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createDLQDiagnostics,dlqTarget} from "./dlq-diagnostics.mjs";
import {queueDetailURL,streamDetailURL} from "./routes.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {DLQEvidence} from "./DLQEvidence.jsx";

export function DLQDiagnostics({api,plan,language,router,declarationETag,declarationReadAt}){
  const t=(en,zh)=>language==="zh"?zh:en;
  let target;try{target=dlqTarget(plan);}catch{return <section><h3>{t("DLQ diagnosis","DLQ 诊断")}</h3><p role="alert">{t("Invalid DLQ plan; no related-resource reads were sent.","DLQ Plan 无效，未发送关联资源读取。")}</p></section>;}
  if(!target)return <section><h3>{t("DLQ diagnosis","DLQ 诊断")}</h3><p>{t("No dead-letter target is configured in this declaration.","此声明未配置死信目标。")}</p></section>;
  return <ConfiguredDLQ api={api} plan={plan} language={language} router={router} target={target} declarationETag={declarationETag} declarationReadAt={declarationReadAt}/>;
}
function ConfiguredDLQ({api,plan,language,router,target,declarationETag,declarationReadAt}){
  const t=(en,zh)=>language==="zh"?zh:en,model=useMemo(()=>createDLQDiagnostics(api,plan),[api,plan]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const labels={unobserved:t("Not observed","未观测"),loading:t("Reading…","正在读取…"),available:t("Observed in this read","本次读取已观测"),missing:t("Not found in this read","本次读取不存在"),denied:t("Access denied","访问被拒绝"),invalid:t("Incompatible response","响应不兼容"),unavailable:t("Read unavailable","读取不可用")};
  const link=(path,label)=><a href={path} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(path);}}}>{label}</a>;
  const status=source=><><p role={["missing","denied","invalid","unavailable"].includes(source.phase)?"alert":"status"}>{labels[source.phase]}</p>{source.readAt&&<ReadTime value={source.readAt} language={language}/>}</>;
  const controller=state.controller.value;
  return <section aria-label={t("DLQ diagnosis","DLQ 诊断")}><h3>{t("DLQ diagnosis","DLQ 诊断")}</h3>
    <p>{t("Declared target","声明目标")}: {link(queueDetailURL(target.queue),target.queue)} · <code>{target.mechanism}</code></p>
    <p>{t("Read-only, one target hop. Declaration, target and controller reads are not an atomic snapshot. Existence does not prove ownership, readiness or successful transfer.","只读且仅检查一层目标。声明、目标及控制器读取不是原子快照；存在不证明所有权、就绪或转移成功。")}</p>
    <button disabled={Object.values(state).some(value=>value.phase==="loading")} onClick={()=>void model.load()}>{t("Refresh DLQ evidence","刷新 DLQ 证据")}</button>
    <section aria-label={t("DLQ target declaration","DLQ 目标声明")}><h4>{t("Target declaration","目标声明")}</h4>{status(state.target)}{state.target.value&&<p>{t("Declaration ETag","声明 ETag")}: <code>{state.target.etag??t("Unreported","未报告")}</code></p>}</section>
    <section aria-label={t("DLQ target Stream","DLQ 目标 Stream")}><h4>{t("Target Stream","目标 Stream")}</h4>{link(streamDetailURL(target.stream),target.stream)}{status(state.stream)}</section>
    <section aria-label={t("DLQ controller evidence","DLQ 控制器证据")}><h4>{t("Controller process — all Queues","控制器进程 — 所有 Queue")}</h4>{status(state.controller)}{controller&&<dl>
      {[[t("Instance","实例"),controller.instanceId],[t("Enabled","已启用"),String(controller.enabled)],[t("Reports leader","报告为 Leader"),String(controller.leader)],[t("Last run","上次运行"),controller.lastRun],[t("Last successful run","上次成功运行"),controller.lastSuccess],[t("Error reported","报告存在错误"),String(controller.errorReported)],[t("Reported processed","报告已处理"),controller.dlqProcessed],[t("Reported moved","报告已转移"),controller.dlqMoved],[t("Reported failed","报告失败"),controller.dlqFailed],[t("Reported ignored","报告已忽略"),controller.dlqIgnored]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{value===undefined?t("Unreported","未报告"):String(value)}</dd></React.Fragment>)}
    </dl>}<p>{t("Counters belong to this controller process across all Queues, can reset, and are not a complete transfer ledger. Leader/enabled flags are not health proof. Per-Queue transfer history is unavailable; no success is inferred from zero failures.","计数属于此控制器进程的所有 Queue，可能重置，不是完整转移台账。Leader/启用标志不是健康证明。尚无逐 Queue 转移历史，不按失败数为零推断成功。")}</p></section>
    <p>{t("These are processing attempts, not unique messages. Redelivered advisories can be counted again; moved can include an already-absent source message. A completed run can contain failed transfer attempts. Last successful run does not prove every transfer succeeded.","这些是处理尝试计数，不是去重消息数。重新投递的事件可能重复计数；已转移计数可包含源消息已不存在的情况。已完成的运行中也可能存在失败的转移尝试。上次成功运行不证明每次转移均成功。")}</p>
    <p>{t("Ignored counts advisory attempts with invalid JSON or no matching declared source. It does not prove message loss or completed acknowledgment. Missing counters remain unreported, not zero.","已忽略计数包括事件 JSON 无效或无法匹配声明来源的处理尝试，不证明消息丢失或确认已完成。缺失计数保持未报告，不补零。")}</p>
    <DLQEvidence plan={plan} state={state} declarationETag={declarationETag} declarationReadAt={declarationReadAt} language={language}/>
  </section>;
}
