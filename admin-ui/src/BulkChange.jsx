import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createBulkChange} from "./bulk-change.mjs";
import {QueueEditor} from "./QueueEditor.jsx";
import {bulkChangeLabels} from "./bulk-change-labels.mjs";

export function BulkChange({api,language,prepare}){
  const model=useMemo(()=>createBulkChange(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),[selected,setSelected]=useState(null),[prepareFailure,setPrepareFailure]=useState(null),text=bulkChangeLabels(language);
  useEffect(()=>()=>model.clear(),[model]);
  const failures={bounds:text.choose_1_100_nonempty_packages_at_most,files:text.every_file_must_be_a_valid_rjs,denied:text.bulk_change_planning_was_denied,changed:text.console_capabilities_changed_reload_before_planning_again,invalid:text.server_returned_inconsistent_itemized_preview_evidence,unavailable:text.bulk_change_planning_is_unavailable_retry_explicitly};
  async function open(item){setPrepareFailure(null);try{setSelected(await prepare(item.document,item.etag));}catch(error){setPrepareFailure(error.code??"unavailable");}}
  return <section aria-label={text.bulk_queue_changes}><h2>{text.plan_bounded_queue_updates}</h2>
    <p>{text.upload_versioned_change_packages_containing_a_queue}</p>
    <label htmlFor="bulk-change-files">{text.queue_change_packages}</label><input id="bulk-change-files" type="file" accept="application/json,.json" multiple onChange={event=>void model.read(event.target.files)}/>
    <button type="button" disabled={state.phase==="planning"||!state.packages.length||state.packages.some(item=>item.status!=="loaded")} onClick={()=>void model.plan()}>{text.preview_every_target}</button><button type="button" onClick={()=>{setSelected(null);model.clear();}}>{text.clear_local_batch}</button>
    {state.phase==="planning"&&<p role="status">{text.reading_itemized_server_previews}</p>}{state.failure&&<p role="alert">{failures[state.failure]}</p>}
    {!!state.packages.length&&<ol>{state.packages.map((item,index)=><li key={index}>{item.filename} — {item.document?.metadata?.name??text.invalid_package} — {item.status}</li>)}</ol>}
    {state.result&&<section aria-label={text.bulk_preview_results}><p role="status">{state.result.ready?text.every_item_has_an_unblocked_preview_each : text.some_items_are_not_ready_successful_siblings}</p><ol>{state.result.items.map(item=><li key={item.index}><strong>{item.document.metadata.name}</strong> — {item.status}{item.code&&<> — {item.code}</>}{item.status==="ready"&&<button type="button" onClick={()=>void open(item)}>{text.prepare_independent_review}: {item.document.metadata.name}</button>}</li>)}</ol></section>}
    {prepareFailure&&<p role="alert">{prepareFailure==="changed"?text.the_target_changed_since_batch_preview_re : text.this_target_cannot_be_prepared_because_another}</p>}
    {selected&&<QueueEditor model={selected} language={language}/>}</section>;
}
