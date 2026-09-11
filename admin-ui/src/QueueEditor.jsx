import React,{useEffect,useRef,useSyncExternalStore} from "react";
import {stringifyJSON} from "./api.mjs";
import {QueueEvidence} from "./QueueEvidence.jsx";
import {QueueFields} from "./QueueFields.jsx";
import {queueDiagnostic} from "./queue-diagnostic.mjs";
import {focusDiagnostic} from "./diagnostic-focus.mjs";
import {MutationEvidence} from "./MutationEvidence.jsx";
import {PreviewOperations} from "./PreviewOperations.jsx";
import {previewOperations} from "./preview-operations.mjs";
import {DeclarationReview} from "./DeclarationReview.jsx";
import {declarationReview} from "./declaration-review.mjs";
import {capabilityCheckPreventedDispatch} from "./mutation-capabilities.mjs";

export function QueueEditor({model,language}) {
  const editorRoot=useRef(null);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load().then(()=>model.loadForm?.());},[model]);
  const t=(en,zh)=>language==="zh"?zh:en;
  const editable=["editing","review","blocked","preview-error","conflict","denied"].includes(state.phase);
  const diagnostic=queueDiagnostic(state);
  const capabilityFailure=capabilityCheckPreventedDispatch(state);
  return <section ref={editorRoot} className="queue-list" aria-labelledby="editor-heading">
    <h2 id="editor-heading">{state.create?t("Create Queue draft","创建 Queue 草稿"):t("Edit Queue draft","编辑 Queue 草稿")}: {state.name}</h2>
    {state.create&&<p>{t("Create only: If-None-Match: *. No existing declaration will be replaced.","仅创建：If-None-Match: *。不会替换已有声明。")}</p>}
    <p>{t("Review a current preview and explicitly confirm before applying. Drafts and unresolved request evidence survive in-app navigation in memory; reloading loses them. No automatic write retries.","提交前须审阅当前预览并明确确认。草稿和未决请求证据在站内导航时保留于内存，刷新会丢失。不自动重试写入。")}</p>
    {state.phase==="loading" && <p role="status">{t("Loading canonical document…","正在读取规范文档…")}</p>}
    {["load-error","uneditable"].includes(state.phase)&&<>
      <button onClick={()=>void model.retryLoad()}>{t("Retry canonical read","重试读取规范文档")}</button>
      <p>{t("No editable base has been loaded. Retry reads the declaration only; it cannot create, repair or apply a Queue. Unsupported declarations remain read-only until the server can reconstruct a valid document.","尚未读取到可编辑基线。重试仅读取声明，不会创建、修复或提交 Queue。服务端无法重建有效文档的声明仍保持只读。")}</p>
      {state.error?.status&&<p>HTTP {state.error.status}</p>}
    </>}
    {state.etag && <p>{t("Original ETag","原始 ETag")}: {state.etag}</p>}
    {state.draft && <>
      <QueueFields model={model} state={state} language={language} editable={editable}/>
      <label htmlFor="queue-draft">{t("Queue document (JSON)","Queue 文档（JSON）")}</label>
      <textarea id="queue-draft" className="queue-draft" value={state.raw} onChange={event=>model.edit(event.target.value)} disabled={!editable} spellCheck="false" />
      <p>{t("Queue identity is immutable. Integers are decoded without precision loss; duration and size strings retain their units.","Queue 身份不可变。整数无损解析，时长和大小字符串保留单位。")}</p>
      <button disabled={!editable || !!state.validation} onClick={()=>void model.preview()}>{t("Preview changes","预览变更")}</button>
    </>}
    {state.validation && <p role="alert">{t("Invalid JSON or changed Queue identity. Previous preview is invalidated.","JSON 无效或 Queue 身份被修改，之前的预览已失效。")}</p>}
    <MutationEvidence state={state} language={language}/>
    {state.phase==="previewing" && <p role="status">{t("Reading current topology for preview…","正在读取当前拓扑以预览…")}</p>}
    {state.phase==="reading-conflict" && <p role="status">{t("Reading latest declaration for comparison…","正在读取最新声明以比较…")}</p>}
    {state.phase==="conflict" && !state.create && <button onClick={()=>void model.readConflict()}>{t("Read latest for comparison","读取最新声明进行比较")}</button>}
    {state.phase==="conflict"&&state.create&&<p role="alert">{t("Creation conflicts with current resource state. Inspect the existing Queue separately; this flow cannot convert to an overwrite.","创建与当前资源状态冲突，请单独检查现有 Queue。此流程不能转换为覆盖操作。")}</p>}
    {state.phase==="conflict" && state.comparison && <section aria-labelledby="conflict-heading">
      <h3 id="conflict-heading">{t("Resolve declaration conflict","解决声明冲突")}</h3>
      <p>{t("Compare all three documents. The merge field starts with your local draft, not an automatic merge; preserve or explicitly replace remote changes. Reading does not update your original ETag.","请比较三个文档。合并框以本地草稿开始，并非自动合并；请保留或明确替换远端变化。读取不会更新原始 ETag。")}</p>
      <h4>{t("Original base","原始基线")}</h4><pre className="declaration-json">{stringifyJSON(state.base)}</pre>
      <h4>{t("Local draft","本地草稿")}</h4><pre className="declaration-json">{state.raw}</pre>
      <h4>{t("Latest declaration","最新声明")}</h4><p>ETag: {state.comparison.etag}</p><pre className="declaration-json">{stringifyJSON(state.comparison.document)}</pre>
      <label htmlFor="merged-draft">{t("Reviewed merged document (JSON)","已审阅的合并文档（JSON）")}</label>
      <textarea id="merged-draft" className="queue-draft" value={state.mergeRaw} onChange={event=>model.editMerge(event.target.value)} spellCheck="false" />
      {state.mergeValidation && <p role="alert">{t("Invalid merged JSON or changed Queue identity.","合并 JSON 无效或 Queue 身份被修改。")}</p>}
      <label><input type="checkbox" checked={state.mergeConfirmed} onChange={event=>model.confirmMerge(event.target.checked)} disabled={!!state.mergeValidation} />{t("I reviewed all three versions and approve this merged draft.","我已比较三个版本并确认此合并草稿。")}</label>
      <button disabled={!state.mergeConfirmed||!!state.mergeValidation} onClick={()=>model.rebase()}>{t("Use merged draft and new base","采用合并草稿与新基线")}</button>
      <p>{t("This changes only the local draft/base. A new preview is required; no apply is performed.","仅更新本地草稿/基线，必须重新预览，不会执行提交。")}</p>
    </section>}
    {state.error && !capabilityFailure && <p role="alert">{state.phase==="conflict" ? t("Original declaration revision has changed. Draft and original ETag are retained; no automatic rebase or write.","原声明版本已变化。保留草稿和原始 ETag，不自动变基或写入。") : state.phase==="uneditable" ? t("This declaration cannot be safely reconstructed for editing.","此声明无法安全重建为可编辑文档。") : state.phase==="denied" ? t("Operation denied. Check current authorization.","操作被拒绝，请检查当前授权。") : t("Operation failed. Review the outcome state below; no automatic retry is performed.","操作失败。请查看下方结果状态，不会自动重试。")}</p>}
    {state.error?.code==="invalid_preview"&&<p role="alert">{t("Preview response is incomplete or inconsistent. It cannot authorize submission. Draft and original ETag are retained; retry preview explicitly.","预览响应不完整或自相矛盾，不能授权提交。草稿和原始 ETag 已保留，请手动重新预览。")}</p>}
    {capabilityFailure&&<p role="alert">{t("Capabilities changed or could not be verified. No write request was sent. Draft and original ETag are retained; preview and confirm again.","能力已变化或无法验证，未发送写请求。草稿和原始 ETag 保留，请重新预览并确认。")}</p>}
    {diagnostic&&<section aria-labelledby="queue-diagnostic-heading">
      <h3 id="queue-diagnostic-heading">{t("Server preview validation","服务端预览校验")}</h3>
      {diagnostic.dependencyCycle&&<p>{t("DLQ dependency cycle detected. Inspect the referenced Queue declarations, correct the dependency chain, then preview again. This can be an existing downstream cycle, not only a reference back to this Queue. Do not treat it as a network failure or retry the unchanged draft. This read-only preview did not apply changes.","检测到 DLQ 依赖循环。请检查引用 Queue 的声明，修正依赖链后重新预览。循环可能已存在于下游，不一定直接返回当前 Queue。不要将其当作网络故障，也不要重复提交未修改的草稿。本次只读预览未应用变更。")}</p>}
      <p>{t("The server rejected this preview document. Correct the draft and preview again; no apply was performed by this preview. Diagnostics are shown as returned by the server, not translated or mapped to guessed fields.","服务端拒绝了本次预览文档。请修正草稿后重新预览；此预览未执行提交。以下诊断按服务端原文显示，不翻译或猜测对应字段。")}</p>
      <pre className="declaration-json">{diagnostic.message}</pre>
      {diagnostic.fields?.length>0&&<ul aria-label={t("Server field diagnostics","服务端字段诊断")}>
        {diagnostic.fields.map((issue,index)=><li key={index}><code>{issue.path}</code> — {issue.message} (<code>{issue.code}</code>)
          <button type="button" onClick={()=>focusDiagnostic(editorRoot.current,issue.path)}>{t("Locate field or JSON","定位字段或 JSON")}: {issue.path}</button>
        </li>)}
      </ul>}
      {diagnostic.truncated&&<p role="status">{t("Diagnostic display truncated at 16,384 characters.","诊断显示已截断至 16,384 个字符。")}</p>}
      {diagnostic.requestId&&<p>{t("Preview response request ID","预览响应请求标识")}: <code>{diagnostic.requestId}</code></p>}
    </section>}
    {state.preview && <>
      <h3>{t("Advisory preview — not applied","建议性预览——尚未应用")}</h3>
      <p role="status">{state.preview.result.blocked ? t("Blocked","已阻止") : state.preview.result.status==="noop" ? t("No changes reported","未报告变更") : t("Changes proposed","已提出变更")}</p>
      <DeclarationReview state={state} language={language}/>
      <PreviewOperations preview={state.preview} language={language}/>
      <details><summary>{t("Raw preview result","原始预览结果")}</summary><pre className="declaration-json">{stringifyJSON(state.preview.result)}</pre></details>
      <details><summary>{t("Generated plan for this preview","本次预览的生成计划")}</summary><pre className="declaration-json">{stringifyJSON(state.preview.plan)}</pre></details>
      {state.phase==="review" && state.preview.result.status!=="noop" && previewOperations(state.preview.result)!==null && declarationReview(state).status!=="invalid" && <>
        <label><input type="checkbox" checked={state.applyConfirmed} onChange={event=>model.confirmApply(event.target.checked)} />{t("I reviewed this preview and authorize applying this draft.","我已审阅本次预览，确认提交此草稿。")}</label>
        <button disabled={!state.applyConfirmed} onClick={()=>void model.apply()}>{t("Apply reviewed draft","提交已审阅草稿")}</button>
      </>}
    </>}
    {state.phase==="submitting" && <p role="status">{t("Submitting once. Do not retry or close this page.","正在提交一次请求。请勿重试或关闭页面。")}</p>}
    {state.phase==="accepted" && <p role="status">{t("Apply accepted. This is not proof of observed convergence. Inspect the Queue and Consumers before concluding health.","提交已接受，不代表观测已收敛。请检查 Queue 和 Consumer 后再判断健康。")}</p>}
    {["accepted","reading-next"].includes(state.phase)&&!state.create&&<>
      <button disabled={state.phase==="reading-next"} onClick={()=>void model.editNext()}>{t("Read latest declaration to edit again","读取最新声明并继续编辑")}</button>
      <p>{t("Starts a new draft from a fresh canonical read and ETag, not the previous submission. A new preview and confirmation are required. Prior accepted request evidence stays in session memory.","使用重新读取的规范文档和 ETag 开始新草稿，而非沿用上次提交。必须重新预览和确认。之前已接受的请求证据保留在会话内存中。")}</p>
      {state.phase==="reading-next"&&<p role="status">{t("Reading the next edit base…","正在读取下一次编辑基线…")}</p>}
      {state.nextEditError&&<p role="alert">{t("Latest edit base unavailable. Previous accepted evidence is retained; no new draft or write was started. Retry the read explicitly.","最新编辑基线不可用。已保留之前的接受证据，未开始新草稿或写入。请手动重试读取。")}</p>}
    </>}
    {!!state.acceptedOperations?.length&&<details>
      <summary>{t("Previous accepted requests (session memory)","之前已接受的请求（会话内存）")}</summary>
      <p>{t("Accepted responses are not proof of current convergence. Request IDs are correlation identifiers, not idempotency keys.","接受响应不证明当前已收敛。请求标识用于关联，不是幂等键。")}</p>
      <pre className="declaration-json">{stringifyJSON(state.acceptedOperations)}</pre>
    </details>}
    {state.requestId && <p>{t("Request ID (not an idempotency key)","请求标识（不是幂等键）")}: <code>{state.requestId}</code></p>}
    {state.draft&&<QueueEvidence state={state} language={language}/>}
    {["uncertain","inspecting"].includes(state.phase) && <>
      {state.error?.status && <p>{t("Last HTTP status (not proof of the original attempt's outcome)","最后 HTTP 状态（不能证明原始请求结果）")}: {state.error.status}</p>}
      <p role="alert">{t("Write outcome unknown. The server may have applied changes. Further writes in this editor are blocked; do not replay the request. Read current evidence without assuming it was caused by this request.","写入结果未知，服务端可能已应用变更。此编辑器禁止后续写入，请勿重放请求。可只读检查当前证据，但不能认定是本次请求造成。")}</p>
      <button disabled={state.phase==="inspecting"} onClick={()=>void model.inspect()}>{t("Inspect current state (read-only)","只读检查当前状态")}</button>
      {state.inspection && <section aria-labelledby="inspection-heading"><h3 id="inspection-heading">{t("Current evidence — not outcome attribution","当前证据——不代表结果归因")}</h3>
        <p>{t("Consumer evidence is the first page only (up to 200). Audit evidence checks at most 256 retained sequence positions; nextBefore means older evidence remains unread. Empty evidence does not prove no execution. Reads do not unlock writes or replace the original ETag; explicit outcome resolution remains unavailable.","Consumer 证据仅为第一页（最多 200 条）。审计证据最多检查 256 个保留序号，nextBefore 表示更旧证据尚未读取。空证据不能证明未执行。读取不解锁写入或替换原始 ETag，明确结果判定仍未提供。")}</p>
        {Object.entries(state.inspection).map(([source,evidence])=><div key={source}><h4>{source}</h4><p>{evidence.status} · {evidence.readAt}</p><pre className="declaration-json">{stringifyJSON({status:evidence.status,etag:evidence.etag,...(evidence.windows?{windows:evidence.windows}:{body:evidence.body})})}</pre></div>)}
        {state.inspection.audit?.windows && <>
          {state.inspection.audit.olderError && <p role="alert">{t("Older audit evidence could not be read. Previous windows and cursor are retained; retry explicitly.","无法读取更旧的审计证据。之前的窗口与游标仍保留，请手动重试。")}</p>}
          {state.inspection.audit.windows.at(-1).body.nextBefore !== null ? <button disabled={state.phase==="inspecting"} onClick={()=>void model.inspectOlderAudit()}>{t("Read older audit window","读取更旧审计窗口")}</button> : <p>{t("Reached the observed retained lower boundary. This does not prove the absence of expired, future or unrecorded events.","已到达观测到的保留下界，不证明过期、未来或未记录事件不存在。")}</p>}
        </>}
      </section>}
    </>}
  </section>;
}
