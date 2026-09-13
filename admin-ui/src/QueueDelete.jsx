import React,{useSyncExternalStore} from "react";
import {stringifyJSON} from "./api.mjs";
import {DeleteEvidence} from "./DeleteEvidence.jsx";
import {MutationEvidence} from "./MutationEvidence.jsx";
import {deletionLabels} from "./deletion-labels.mjs";
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {CopyValue} from "./copy-value.jsx";
import {queueDetailURL} from "./routes.mjs";
import {queueDeleteLabels} from "./queue-delete-labels.mjs";

export function QueueDelete({model,language,router,blocked=false}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot),p=state.preview,accessible=deletionLabels(language);
  const labels=queueDeleteLabels(language);
  const audit=state.inspection?.audit,cursor=audit?.windows?.at(-1)?.body.nextBefore;
  return <section className="queue-list queue-delete" aria-label={accessible.deletion}>
    <ResourceBreadcrumb trail={[{label:labels.all_queues,href:"/admin/queues"},{label:state.name,href:queueDetailURL(state.name)},{label:labels.delete}]} router={router} language={language}/>
    <h2>{labels.delete_queue}: {state.name}</h2>
    <MutationEvidence state={state} language={language}/>
    <p>{labels.deleting_the_stream_removes_its_messages_and}</p>
    {blocked&&<p role="alert">{labels.an_editor_for_this_queue_is_retained}</p>}
    {["idle","review","preview-error"].includes(state.phase)&&<button disabled={blocked} onClick={()=>void model.preview()}>{labels.read_deletion_preflight}</button>}
    {state.phase==="previewing"&&<p role="status">{labels.reading_deletion_impact}</p>}
    {state.phase==="preview-error"&&<p role="alert">{labels.deletion_preflight_unavailable_or_inconsistent_no_deletion} ({state.error.status??state.error.code??state.error.kind})</p>}
    {p&&<dl className="declaration-meta">
      <dt>Stream</dt><dd>{p.stream}</dd><dt>ETag</dt><dd>{state.etag}</dd>
      <dt>{labels.observed_at}</dt><dd>{p.observed_at}</dd>
      <dt>{labels.ownership}</dt><dd>{p.ownership}</dd>
      <dt>{labels.messages}</dt><dd>{p.messages===undefined?labels.unknown_stream_missing:String(p.messages)}</dd>
      <dt>Consumers</dt><dd>{p.consumers===undefined?labels.unknown_stream_missing:String(p.consumers)}</dd>
      <dt>{labels.default_deletion_blocked}</dt><dd>{String(p.blocked)}{p.reason?`: ${p.reason}`:""}</dd>
    </dl>}
    {state.phase==="review"&&<fieldset disabled={blocked}>
      <legend>{labels.confirm_observed_deletion_impact}</legend>
      {!p.stream_present&&<p>{labels.stream_was_not_found_this_attempt_may}</p>}
      <label className="delete-check"><input type="checkbox" checked={state.force} onChange={e=>model.force(e.target.checked)}/><span>{labels.force_deletion_with_messages}</span></label>
      <label>{labels.type_exact_queue_name}<input value={state.typed} autoComplete="off" spellCheck="false" onChange={e=>model.type(e.target.value)}/></label>
      <label className="delete-check"><input type="checkbox" checked={state.acknowledged} onChange={e=>model.acknowledge(e.target.checked)}/><span>{labels.i_reviewed_this_impact_and_understand_that}</span></label>
      <button className="delete-submit" disabled={!model.canSubmit()} onClick={()=>void model.submit()}>{labels.delete_this_queue}</button>
      <button onClick={()=>model.cancel()}>{labels.cancel_deletion_review}</button>
    </fieldset>}
    {state.phase==="submitting"&&<p role="status">{labels.deletion_request_pending_do_not_repeat_it}</p>}
    {state.phase==="accepted"&&<p role="status">{labels.server_acknowledged_deletion_this_is_not_a}</p>}
    {state.phase==="uncertain"&&<p role="alert">{labels.deletion_outcome_unknown_no_retry_is_enabled}</p>}
    {state.requestId&&<p>Request ID: <CopyValue value={state.requestId} language={language}/></p>}
    {["accepted","uncertain"].includes(state.phase)&&<button onClick={()=>void model.inspect()}>{labels.read_deletion_outcome_evidence}</button>}
    {state.phase==="inspecting"&&<p role="status">{labels.reading_evidence_only}</p>}
    {state.inspection&&<><p>{labels.audit_evidence_is_a_bounded_window_not}</p><pre className="declaration-json">{stringifyJSON(state.inspection)}</pre></>}
    {audit?.status==="available"&&<p>{labels.audit_windows_read}: {audit.windows.length}. {cursor===null?labels.no_older_cursor_in_this_scan_this:labels.older_windows_remain_even_if_the_current}</p>}
    {audit?.olderError&&<p role="alert">{labels.older_audit_read_failed_existing_evidence_and}</p>}
    {cursor!==undefined&&cursor!==null&&<button disabled={state.phase==="inspecting"} onClick={()=>void model.inspectOlderAudit()}>{labels.read_older_deletion_audit_window}</button>}
    {!!state.inspectionHistory?.length&&<details><summary>{labels.previous_deletion_inspections}: {state.inspectionHistory.length}</summary><pre className="declaration-json">{stringifyJSON(state.inspectionHistory)}</pre></details>}
    {state.requestId&&<details><summary>{labels.deletion_request_evidence_save_before_clearing_session}</summary><pre className="declaration-json">{stringifyJSON(state)}</pre></details>}
    {(state.preview||state.requestId)&&<DeleteEvidence state={state} language={language}/>}
  </section>;
}
