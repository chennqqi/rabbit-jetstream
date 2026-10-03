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
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {CopyValue} from "./copy-value.jsx";
import {queueDetailURL} from "./routes.mjs";
import {queueEditorLabels} from "./queue-editor-labels.mjs";

export function QueueEditor({model,language,router}) {
  const editorRoot=useRef(null);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load().then(()=>model.loadForm?.());},[model]);
  const text=queueEditorLabels(language);
  const editable=["editing","review","blocked","preview-error","conflict","denied"].includes(state.phase);
  const diagnostic=queueDiagnostic(state);
  const capabilityFailure=capabilityCheckPreventedDispatch(state);
  return <section ref={editorRoot} className="queue-list" aria-labelledby="editor-heading">
    {!state.create&&router&&state.name&&<ResourceBreadcrumb trail={[{label:text.all_queues,href:"/admin/queues"},{label:state.name,href:queueDetailURL(state.name)},{label:text.edit}]} router={router} language={language}/>}
    <h2 id="editor-heading">{state.create?text.create_queue_draft:text.edit_queue_draft}: {state.name}</h2>
    {state.create&&<p>{text.create_only_if_none_match_no_existing}</p>}
    <p>{text.review_a_current_preview_and_explicitly_confirm}</p>
    {state.phase==="loading" && <p role="status">{text.loading_canonical_document}</p>}
    {["load-error","uneditable"].includes(state.phase)&&<>
      <button onClick={()=>void model.retryLoad()}>{text.retry_canonical_read}</button>
      <p>{text.no_editable_base_has_been_loaded_retry}</p>
      {state.error?.status&&<p>HTTP {state.error.status}</p>}
    </>}
    {state.etag && <p>{text.original_etag}: {state.etag}</p>}
    {state.draft && <>
      <QueueFields model={model} state={state} language={language} editable={editable}/>
      <label htmlFor="queue-draft">{text.queue_document_json}</label>
      <textarea id="queue-draft" className="queue-draft" value={state.raw} onChange={event=>model.edit(event.target.value)} disabled={!editable} spellCheck="false" />
      <p>{text.queue_identity_is_immutable_integers_are_decoded}</p>
      <button disabled={!editable || !!state.validation} onClick={()=>void model.preview()}>{text.preview_changes}</button>
    </>}
    {state.validation && <p role="alert">{text.invalid_json_or_changed_queue_identity_previous}</p>}
    <MutationEvidence state={state} language={language}/>
    {state.phase==="previewing" && <p role="status">{text.reading_current_topology_for_preview}</p>}
    {state.phase==="reading-conflict" && <p role="status">{text.reading_latest_declaration_for_comparison}</p>}
    {state.phase==="conflict" && !state.create && <button onClick={()=>void model.readConflict()}>{text.read_latest_for_comparison}</button>}
    {state.phase==="conflict"&&state.create&&<p role="alert">{text.creation_conflicts_with_current_resource_state_inspect}</p>}
    {state.phase==="conflict" && state.comparison && <section aria-labelledby="conflict-heading">
      <h3 id="conflict-heading">{text.resolve_declaration_conflict}</h3>
      <p>{text.compare_all_three_documents_the_merge_field}</p>
      <h4>{text.original_base}</h4><pre className="declaration-json">{stringifyJSON(state.base)}</pre>
      <h4>{text.local_draft}</h4><pre className="declaration-json">{state.raw}</pre>
      <h4>{text.latest_declaration}</h4><p>ETag: {state.comparison.etag}</p><pre className="declaration-json">{stringifyJSON(state.comparison.document)}</pre>
      <label htmlFor="merged-draft">{text.reviewed_merged_document_json}</label>
      <textarea id="merged-draft" className="queue-draft" value={state.mergeRaw} onChange={event=>model.editMerge(event.target.value)} spellCheck="false" />
      {state.mergeValidation && <p role="alert">{text.invalid_merged_json_or_changed_queue_identity}</p>}
      <label><input type="checkbox" checked={state.mergeConfirmed} onChange={event=>model.confirmMerge(event.target.checked)} disabled={!!state.mergeValidation} />{text.i_reviewed_all_three_versions_and_approve}</label>
      <button disabled={!state.mergeConfirmed||!!state.mergeValidation} onClick={()=>model.rebase()}>{text.use_merged_draft_and_new_base}</button>
      <p>{text.this_changes_only_the_local_draft_base}</p>
    </section>}
    {state.error && !capabilityFailure && <p role="alert">{state.phase==="conflict" ? text.original_declaration_revision_has_changed_draft_and : state.phase==="uneditable" ? text.this_declaration_cannot_be_safely_reconstructed_for : state.phase==="denied" ? text.operation_denied_check_current_authorization : text.operation_failed_review_the_outcome_state_below}</p>}
    {state.error?.code==="invalid_preview"&&<p role="alert">{text.preview_response_is_incomplete_or_inconsistent_it}</p>}
    {capabilityFailure&&<p role="alert">{text.capabilities_changed_or_could_not_be_verified}</p>}
    {diagnostic&&<section aria-labelledby="queue-diagnostic-heading">
      <h3 id="queue-diagnostic-heading">{text.server_preview_validation}</h3>
      {diagnostic.dependencyCycle&&<p>{text.dlq_dependency_cycle_detected_inspect_the_referenced}</p>}
      <p>{text.the_server_rejected_this_preview_document_correct}</p>
      <pre className="declaration-json">{diagnostic.message}</pre>
      {diagnostic.fields?.length>0&&<ul aria-label={text.server_field_diagnostics}>
        {diagnostic.fields.map((issue,index)=><li key={index}><code>{issue.path}</code> — {issue.message} (<code>{issue.code}</code>)
          <button type="button" onClick={()=>focusDiagnostic(editorRoot.current,issue.path)}>{text.locate_field_or_json}: {issue.path}</button>
        </li>)}
      </ul>}
      {diagnostic.truncated&&<p role="status">{text.diagnostic_display_truncated_at_16_384_characters}</p>}
      {diagnostic.requestId&&<p>{text.preview_response_request_id}: <CopyValue value={diagnostic.requestId} language={language}/></p>}
    </section>}
    {state.preview && <>
      <h3>{text.advisory_preview_not_applied}</h3>
      <p role="status">{state.preview.result.blocked ? text.blocked : state.preview.result.status==="noop" ? text.no_changes_reported : text.changes_proposed}</p>
      <DeclarationReview state={state} language={language}/>
      <PreviewOperations preview={state.preview} language={language}/>
      <details><summary>{text.raw_preview_result}</summary><pre className="declaration-json">{stringifyJSON(state.preview.result)}</pre></details>
      <details><summary>{text.generated_plan_for_this_preview}</summary><pre className="declaration-json">{stringifyJSON(state.preview.plan)}</pre></details>
      {state.phase==="review" && state.preview.result.status!=="noop" && previewOperations(state.preview.result)!==null && declarationReview(state).status!=="invalid" && <>
        <label><input type="checkbox" checked={state.applyConfirmed} onChange={event=>model.confirmApply(event.target.checked)} />{text.i_reviewed_this_preview_and_authorize_applying}</label>
        <button disabled={!state.applyConfirmed} onClick={()=>void model.apply()}>{text.apply_reviewed_draft}</button>
      </>}
    </>}
    {state.phase==="submitting" && <p role="status">{text.submitting_once_do_not_retry_or_close}</p>}
    {state.phase==="accepted" && <p role="status">{text.apply_accepted_this_is_not_proof_of}</p>}
    {["accepted","reading-next"].includes(state.phase)&&!state.create&&<>
      <button disabled={state.phase==="reading-next"} onClick={()=>void model.editNext()}>{text.read_latest_declaration_to_edit_again}</button>
      <p>{text.starts_a_new_draft_from_a_fresh}</p>
      {state.phase==="reading-next"&&<p role="status">{text.reading_the_next_edit_base}</p>}
      {state.nextEditError&&<p role="alert">{text.latest_edit_base_unavailable_previous_accepted_evidence}</p>}
    </>}
    {!!state.acceptedOperations?.length&&<details>
      <summary>{text.previous_accepted_requests_session_memory}</summary>
      <p>{text.accepted_responses_are_not_proof_of_current}</p>
      <pre className="declaration-json">{stringifyJSON(state.acceptedOperations)}</pre>
    </details>}
    {state.requestId && <p>{text.request_id_not_an_idempotency_key}: <CopyValue value={state.requestId} language={language}/></p>}
    {state.draft&&<QueueEvidence state={state} language={language}/>}
    {["uncertain","inspecting"].includes(state.phase) && <>
      {state.error?.status && <p>{text.last_http_status_not_proof_of_the}: {state.error.status}</p>}
      <p role="alert">{text.write_outcome_unknown_the_server_may_have}</p>
      <button disabled={state.phase==="inspecting"} onClick={()=>void model.inspect()}>{text.inspect_current_state_read_only}</button>
      {state.inspection && <section aria-labelledby="inspection-heading"><h3 id="inspection-heading">{text.current_evidence_not_outcome_attribution}</h3>
        <p>{text.consumer_evidence_is_the_first_page_only}</p>
        {Object.entries(state.inspection).map(([source,evidence])=><div key={source}><h4>{source}</h4><p>{evidence.status} · {evidence.readAt}</p><pre className="declaration-json">{stringifyJSON({status:evidence.status,etag:evidence.etag,...(evidence.windows?{windows:evidence.windows}:{body:evidence.body})})}</pre></div>)}
        {state.inspection.audit?.windows && <>
          {state.inspection.audit.olderError && <p role="alert">{text.older_audit_evidence_could_not_be_read}</p>}
          {state.inspection.audit.windows.at(-1).body.nextBefore !== null ? <button disabled={state.phase==="inspecting"} onClick={()=>void model.inspectOlderAudit()}>{text.read_older_audit_window}</button> : <p>{text.reached_the_observed_retained_lower_boundary_this}</p>}
        </>}
      </section>}
    </>}
  </section>;
}
