import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createBatchImport} from "./batch-import.mjs";
import {batchImportLabels} from "./batch-import-labels.mjs";

export function BatchImport({api,binding,language,onStart,retained=false}){
  const model=useMemo(()=>createBatchImport(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>()=>model.clear(),[model]);useEffect(()=>model.invalidatePlan(),[model,binding]);
  const text=batchImportLabels(language),result=state.binding===binding?state.result:null;
  const [reviewed,setReviewed]=useState(null);
  const problems={invalid_declaration:text.invalid_declaration,duplicate_queue:text.duplicate_queue_name,external_dependency_unverified:text.external_dependency_not_validated,dependency_cycle:text.dependency_cycle_member,blocked_dependency:text.blocked_by_prerequisite};
  const failures={bounds:text.select_1_100_nonempty_files_at_most,files:text.some_files_are_not_valid_utf_8,denied:text.planning_denied_operator_permission_is_required,changed:text.capabilities_changed_reload_the_creation_schema_and,invalid:text.invalid_planning_evidence_no_order_is_shown,unavailable:text.planning_unavailable_retry_explicitly_this_is_not};
  return <section className="queue-import" aria-label={text.batch_import_planning}>
    <h3>{text.plan_multiple_queue_files}</h3>
    <p>{text.select_files_locally_then_explicitly_send_their}</p>
    <label>{text.queue_json_files}<input type="file" multiple accept="application/json,.json" onChange={event=>{void model.read(event.target.files);event.target.value="";}}/></label>
    {state.files.length>0&&<ol>{state.files.map((file,index)=><li key={index}>{file.name} — {file.status==="loaded"?text.json_read_semantics_unchecked:file.status==="invalid"?text.invalid_file:text.not_read}</li>)}</ol>}
    <button type="button" disabled={!binding||!state.files.length||state.files.some(file=>file.status!=="loaded")||state.phase==="planning"} onClick={()=>void model.plan(binding)}>{text.plan_import_only}</button>
    {state.phase==="reading"||state.phase==="planning"?<p role="status">{text.working}</p>:null}
    {state.files.length>0&&<button type="button" onClick={()=>model.clear()}>{text.clear_batch_files_and_plan}</button>}
    {state.failure&&<p role="alert">{failures[state.failure]}</p>}
    {result&&<>
      <p role="status">{result.ready?text.internally_ordered_only_destination_preview_and_explicit:text.some_items_are_blocked_internal_order_excludes}</p>
      <ol>{result.items.map(item=><li key={item.index}><strong>{state.files[item.index].name}</strong> — Queue: {item.queue||text.unknown}
        {item.problems.length?<ul>{item.problems.map((problem,index)=><li key={index}>{problems[problem.code]} {problem.dependency??""}</li>)}</ul>:<p>{text.no_internal_planning_problem_not_an_apply}</p>}
      </li>)}</ol>
      <h4>{text.dependency_first_order}</h4><ol>{result.order.map(index=><li key={index}>{result.items[index].queue} — {state.files[index].name}</li>)}</ol>
      <h4>{text.destination_preview_order_not_approval}</h4><ol>{(result.review_order??result.order).map(index=><li key={index}>{result.items[index].queue} — {state.files[index].name}</li>)}</ol>
      <h4>{text.external_declaration_observations}</h4>{result.external.length?<ul>{result.external.map(item=><li key={item.queue}>{item.queue} — {({present:text.declaration_present,missing:text.declaration_missing,unavailable:text.unavailable,unrepresentable:text.cannot_reconstruct})[item.status]} {item.etag??""}</li>)}</ul>:<p>{text.no_external_references_in_this_plan}</p>}
      {onStart&&<>
        <label><input type="checkbox" checked={reviewed===result} onChange={event=>setReviewed(event.target.checked?result:null)}/>{text.i_reviewed_the_files_and_blocking_problems}</label>
        <button type="button" disabled={retained||reviewed!==result||!(result.review_order??result.order).length} onClick={()=>onStart(state)}>{text.prepare_per_item_review}</button>
        {retained&&<p>{text.a_batch_is_already_retained_in_this}</p>}
      </>}
    </>}
  </section>;
}
