import React,{useSyncExternalStore} from "react";
import {stringifyJSON} from "./api.mjs";
import {DeleteEvidence} from "./DeleteEvidence.jsx";
import {MutationEvidence} from "./MutationEvidence.jsx";
import {deletionLabels} from "./deletion-labels.mjs";

export function QueueDelete({model,language,blocked=false}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot),zh=language==="zh",p=state.preview,accessible=deletionLabels(language);
  const text=(en,cn)=>zh?cn:en;
  const audit=state.inspection?.audit,cursor=audit?.windows?.at(-1)?.body.nextBefore;
  return <section className="queue-list queue-delete" aria-label={accessible.deletion}>
    <h2>{text("Delete Queue","删除 Queue")}: {state.name}</h2>
    <MutationEvidence state={state} language={language}/>
    <p>{text("Deleting the Stream removes its messages and all Consumers. Counts are observations, not atomic empty/unused protection. DLQ dependents are not checked. Force cannot bypass ownership protection.","删除 Stream 会移除其中的消息和所有 Consumer。计数只是观测，不提供原子空队列/未使用保护。不检查 DLQ 依赖方。force 不能绕过归属保护。")}</p>
    {blocked&&<p role="alert">{text("An editor for this Queue is retained. Use the explicit editor handoff before reading deletion preflight. Pending or unknown operations cannot be handed off.","此 Queue 保留了编辑器。请先明确交接编辑器，再读取删除预检；进行中或结果未知的操作不能交接。")}</p>}
    {["idle","review","preview-error"].includes(state.phase)&&<button disabled={blocked} onClick={()=>void model.preview()}>{text("Read deletion preflight","读取删除预检")}</button>}
    {state.phase==="previewing"&&<p role="status">{text("Reading deletion impact…","正在读取删除影响…")}</p>}
    {state.phase==="preview-error"&&<p role="alert">{text("Deletion preflight unavailable or inconsistent. No deletion was sent. Retry reads explicitly; confirmation has been cleared.","删除预检不可用或不一致。未发送删除请求。可手动重试读取，确认已清除。")} ({state.error.status??state.error.code??state.error.kind})</p>}
    {p&&<dl className="declaration-meta">
      <dt>Stream</dt><dd>{p.stream}</dd><dt>ETag</dt><dd>{state.etag}</dd>
      <dt>{text("Observed at","观测完成时间")}</dt><dd>{p.observed_at}</dd>
      <dt>{text("Ownership","归属")}</dt><dd>{p.ownership}</dd>
      <dt>{text("Messages","消息")}</dt><dd>{p.messages===undefined?text("Unknown — Stream missing","未知 — Stream 缺失"):String(p.messages)}</dd>
      <dt>Consumers</dt><dd>{p.consumers===undefined?text("Unknown — Stream missing","未知 — Stream 缺失"):String(p.consumers)}</dd>
      <dt>{text("Default deletion blocked","默认删除被阻止")}</dt><dd>{String(p.blocked)}{p.reason?`: ${p.reason}`:""}</dd>
    </dl>}
    {state.phase==="review"&&<fieldset disabled={blocked}>
      <legend>{text("Confirm observed deletion impact","确认已观测的删除影响")}</legend>
      {!p.stream_present&&<p>{text("Stream was not found. This attempt may only remove the declaration; absence now is not a guarantee of future absence.","未找到 Stream，此次操作可能只清理声明；当前缺失不保证之后仍缺失。")}</p>}
      <label className="delete-check"><input type="checkbox" checked={state.force} onChange={e=>model.force(e.target.checked)}/><span>{text("Force deletion with messages","强制删除包含消息的队列")}</span></label>
      <label>{text("Type exact Queue name","输入完整 Queue 名称")}<input value={state.typed} autoComplete="off" spellCheck="false" onChange={e=>model.type(e.target.value)}/></label>
      <label className="delete-check"><input type="checkbox" checked={state.acknowledged} onChange={e=>model.acknowledge(e.target.checked)}/><span>{text("I reviewed this impact and understand that deletion is destructive.","我已审阅本次影响，理解删除具有破坏性。")}</span></label>
      <button className="delete-submit" disabled={!model.canSubmit()} onClick={()=>void model.submit()}>{text("Delete this Queue","删除此 Queue")}</button>
      <button onClick={()=>model.cancel()}>{text("Cancel deletion review","取消删除审阅")}</button>
    </fieldset>}
    {state.phase==="submitting"&&<p role="status">{text("Deletion request pending. Do not repeat it.","删除请求处理中，请勿重复提交。")}</p>}
    {state.phase==="accepted"&&<p role="status">{text("Server acknowledged deletion. This is not a fresh absence observation or a guarantee against recreation.","服务端已确认删除。这不是最新缺失观测，也不保证资源不会重建。")}</p>}
    {state.phase==="uncertain"&&<p role="alert">{text("Deletion outcome unknown. No retry is enabled. Missing resources or audit observations alone do not prove this request completed.","删除结果未知，禁止重试。仅凭资源缺失或审计观测，不能证明本次请求已完成。")}</p>}
    {state.requestId&&<p>Request ID: <code>{state.requestId}</code></p>}
    {["accepted","uncertain"].includes(state.phase)&&<button onClick={()=>void model.inspect()}>{text("Read deletion outcome evidence","读取删除结果证据")}</button>}
    {state.phase==="inspecting"&&<p role="status">{text("Reading evidence only…","仅在读取证据…")}</p>}
    {state.inspection&&<><p>{text("Audit evidence is a bounded window, not necessarily complete history. Reads never unlock retry.","审计证据是有界窗口，不一定包含完整历史。读取不会解锁重试。")}</p><pre className="declaration-json">{stringifyJSON(state.inspection)}</pre></>}
    {audit?.status==="available"&&<p>{text("Audit windows read","已读取审计窗口")}: {audit.windows.length}. {cursor===null?text("No older cursor in this scan; this is not proof of complete historical retention.","本次扫描没有更早游标，不代表历史保留完整。"):text("Older windows remain, even if the current window contains no matching records.","仍有更早窗口，即使当前窗口没有匹配记录。")}</p>}
    {audit?.olderError&&<p role="alert">{text("Older audit read failed. Existing evidence and cursor are retained; retry the read explicitly.","更早审计读取失败，已保留原证据与游标，可手动重试读取。")}</p>}
    {cursor!==undefined&&cursor!==null&&<button disabled={state.phase==="inspecting"} onClick={()=>void model.inspectOlderAudit()}>{text("Read older deletion audit window","读取更早删除审计窗口")}</button>}
    {!!state.inspectionHistory?.length&&<details><summary>{text("Previous deletion inspections","此前的删除检查")}: {state.inspectionHistory.length}</summary><pre className="declaration-json">{stringifyJSON(state.inspectionHistory)}</pre></details>}
    {state.requestId&&<details><summary>{text("Deletion request evidence — save before clearing session","删除请求证据 — 清除会话前请保存")}</summary><pre className="declaration-json">{stringifyJSON(state)}</pre></details>}
    {(state.preview||state.requestId)&&<DeleteEvidence state={state} language={language}/>}
  </section>;
}
