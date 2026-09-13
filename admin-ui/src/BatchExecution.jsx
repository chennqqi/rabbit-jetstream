import React,{useSyncExternalStore} from "react";
import {QueueEditor} from "./QueueEditor.jsx";
import {batchPhaseLabel} from "./batch-phase.mjs";
import {batchExecutionLabels} from "./batch-execution-labels.mjs";

export function BatchExecution({model,prepare,ready,language,onBack,onArchive}){
  const state=useSyncExternalStore(model.subscribe,model.snapshot),selected=model.selectedModel();
  const text=batchExecutionLabels(language);
  const reasons={"planning-blocked":text.blocked_by_original_plan_correct_files_and,"request-pending":text.a_request_is_active_or_its_write,"prerequisite-not-accepted":text.an_in_batch_prerequisite_has_not_been,"dependency-changed":text.the_draft_dependency_differs_from_this_plan,"not-selected":text.select_this_item_before_acting,"not-prepared":text.prepare_this_item_first,"retained-name":text.this_name_already_has_a_retained_creation,"preparation-failed":text.preparation_failed_reload_creation_rules_and_check};
  return <>
    <section className="queue-import" aria-label={text.batch_execution}>
      <h2>{text.review_and_apply_one_queue_at_a}</h2>
      <p>{text.each_item_reuses_create_only_preview_and}</p>
      <button type="button" disabled={state.items.some(item=>item.blocked==="request-pending")} onClick={onBack}>{text.keep_batch_and_return_to_creation}</button>
      <button type="button" disabled={!state.canArchive} onClick={onArchive}>{text.archive_batch_and_start_another}</button>
      <p>{text.archiving_stops_this_batch_locally_and_keeps}</p>
      {state.failure&&<p role="alert">{reasons[state.failure]}</p>}
      <ol>{state.items.map(item=><li key={item.index}>
        <strong>{item.queue||text.unknown_queue}</strong> — {item.filename}<p>{batchPhaseLabel(item.phase,language)}</p>
        {item.blocked&&<p>{reasons[item.blocked]}</p>}
        {item.problems.some(problem=>problem.code==="external_dependency_unverified")&&<p>{text.external_dependency_unresolved_in_the_plan_prepare}</p>}
        {item.phase==="not-prepared"?<button type="button" disabled={!ready||!!item.blocked} onClick={()=>model.prepare(item.index,prepare)}>{text.prepare_item}: {item.queue}</button>:<button type="button" disabled={state.selected!==item.index&&state.items.some(value=>value.blocked==="request-pending")} onClick={()=>model.select(item.index)}>{text.review_item}: {item.queue}</button>}
      </li>)}</ol>
    </section>
    {selected&&<QueueEditor key={state.selected} model={selected} language={language}/>}
  </>;
}
