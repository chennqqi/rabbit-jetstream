import React,{useSyncExternalStore} from "react";
import {QueueEditor} from "./QueueEditor.jsx";
import {batchPhaseLabel} from "./batch-phase.mjs";

export function BatchExecution({model,prepare,ready,language,onBack,onArchive}){
  const state=useSyncExternalStore(model.subscribe,model.snapshot),selected=model.selectedModel();
  const t=(en,zh)=>language==="zh"?zh:en;
  const reasons={"planning-blocked":t("Blocked by original plan; correct files and plan separately.","原规划阻塞，请修正文件后重新规划。"),"request-pending":t("A request is active or its write outcome is unknown. Resolve it before switching items.","请求进行中或写入结果未知，请先处理再切换项。"),"prerequisite-not-accepted":t("An in-batch prerequisite has not been accepted. Acceptance still does not prove live health.","批次内前置项尚未接受；接受也不证明实时健康。"),"dependency-changed":t("The draft dependency differs from this plan. Restore it or plan a new batch.","草稿依赖与本规划不同，请恢复依赖或重新规划批次。"),"not-selected":t("Select this item before acting.","操作前请选择此项。"),"not-prepared":t("Prepare this item first.","请先准备此项。"),"retained-name":t("This name already has a retained creation/deletion request. It was not overwritten.","此名称已有保留的创建／删除请求，未覆盖。"),"preparation-failed":t("Preparation failed. Reload creation rules and check imported settings.","准备失败，请重新读取创建规则并检查导入设置。")};
  return <>
    <section className="queue-import" aria-label={t("Batch execution","批次执行")}>
      <h2>{t("Review and apply one Queue at a time","逐项审阅并提交 Queue")}</h2>
      <p>{t("Each item reuses create-only preview and separate apply confirmation. No automatic writes, retries or rollback. Files and outcomes stay in session memory across in-app navigation; reload loses them. External prerequisites require current server validation, regardless of their planning observation. Invalid/duplicate/cyclic items remain blocked. Accepted prerequisites still require live checks in each dependent's server preview.","每项复用仅创建预览及独立提交确认，不自动写入、重试或回滚。文件与结果在站内导航时保留于会话内存，刷新会丢失。不论规划观测如何，外部前置依赖均须当前服务端验证。无效／重复／循环项仍阻塞。即使前置项已接受，依赖项的服务端预览仍须检查实时状态。")}</p>
      <button type="button" disabled={state.items.some(item=>item.blocked==="request-pending")} onClick={onBack}>{t("Keep batch and return to creation","保留批次并返回创建页")}</button>
      <button type="button" disabled={!state.canArchive} onClick={onArchive}>{t("Archive batch and start another","归档批次并开始新批次")}</button>
      <p>{t("Archiving stops this batch locally and keeps a read-only snapshot. It does not cancel or undo resources, release retained Queue names, or authorize retries. Active or unknown requests must be resolved first.","归档仅在本地结束此批次并保留只读快照，不取消或撤销资源、不释放保留的 Queue 名称，也不授权重试。进行中或结果未知的请求须先处理。")}</p>
      {state.failure&&<p role="alert">{reasons[state.failure]}</p>}
      <ol>{state.items.map(item=><li key={item.index}>
        <strong>{item.queue||t("Unknown Queue","未知 Queue")}</strong> — {item.filename}<p>{batchPhaseLabel(item.phase,language)}</p>
        {item.blocked&&<p>{reasons[item.blocked]}</p>}
        {item.problems.some(problem=>problem.code==="external_dependency_unverified")&&<p>{t("External dependency unresolved in the plan; prepare only for server preview, not immediate apply.","规划中的外部依赖未解决，仅可准备服务端预览，不能直接提交。")}</p>}
        {item.phase==="not-prepared"?<button type="button" disabled={!ready||!!item.blocked} onClick={()=>model.prepare(item.index,prepare)}>{t("Prepare item","准备此项")}: {item.queue}</button>:<button type="button" disabled={state.selected!==item.index&&state.items.some(value=>value.blocked==="request-pending")} onClick={()=>model.select(item.index)}>{t("Review item","审阅此项")}: {item.queue}</button>}
      </li>)}</ol>
    </section>
    {selected&&<QueueEditor key={state.selected} model={selected} language={language}/>}
  </>;
}
