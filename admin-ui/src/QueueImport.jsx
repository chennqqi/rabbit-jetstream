import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createQueueImport} from "./queue-import.mjs";
import {queueImportLabels} from "./queue-import-labels.mjs";

export function QueueImport({ready,onPrepare,language}){
  const model=useMemo(()=>createQueueImport(),[]),state=useSyncExternalStore(model.subscribe,model.snapshot);
  const [confirmed,setConfirmed]=useState(false),[failure,setFailure]=useState(null);
  useEffect(()=>()=>model.clear(),[model]);
  const text=queueImportLabels(language);
  return <section className="queue-import" aria-label={text.import_queue_declaration}>
    <h3>{text.import_a_single_queue_json_file}</h3>
    <p>{text.reads_locally_up_to_1_mib_utf}</p>
    <label>{text.queue_json_file}<input type="file" accept="application/json,.json" onChange={event=>{const file=event.target.files?.[0];setConfirmed(false);setFailure(null);model.clear();if(file)void model.read(file);event.target.value="";}}/></label>
    {state.phase==="loading"&&<><p role="status">{text.reading_local_file}</p><button type="button" onClick={()=>model.clear()}>{text.cancel_file_read}</button></>}
    {state.phase==="error"&&<p role="alert">{state.failure==="size"?text.choose_a_nonempty_json_file_no_larger:text.invalid_utf_8_json_or_unsupported_single}</p>}
    {state.document&&<>
      <p>{text.loaded_file}{state.filename} · {state.bytes} {text.bytes}</p>
      <p>Queue: {state.document.metadata.name} · {text.labels}{Object.keys(state.document.metadata.labels??{}).length}</p>
      <details><summary>{text.review_imported_document}</summary><pre className="declaration-json">{state.raw}</pre></details>
      <label><input type="checkbox" checked={confirmed} onChange={event=>setConfirmed(event.target.checked)}/>{text.i_reviewed_this_file_and_want_to}</label>
      <button type="button" disabled={!ready||!confirmed} onClick={async()=>{setFailure(null);try{await onPrepare(state.document);}catch(error){setFailure(error.code??"invalid");}}}>{text.prepare_imported_creation_draft}</button>
      {!ready&&<p>{text.reload_the_creation_schema_before_preparing_this}</p>}
      {failure&&<p role="alert">{failure==="retained-name"?text.this_name_has_a_retained_request_in:text.cannot_prepare_this_draft_check_current_schema}</p>}
    </>}
  </section>;
}
