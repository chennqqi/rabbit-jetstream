import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createQueueExport} from "./queue-export.mjs";
import {queueExportLabels} from "./queue-export-labels.mjs";

export function QueueExport({api,name,document,etag,language}){
  const model=useMemo(()=>createQueueExport(api,name,document,etag),[api,name,document,etag]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),[includeLabels,setIncludeLabels]=useState(false),[download,setDownload]=useState(null);
  const text=queueExportLabels(language);
  useEffect(()=>()=>model.clear(),[model]);
  useEffect(()=>{
    if(!state.file){setDownload(null);return;}
    try {const url=URL.createObjectURL(new Blob([state.file.content],{type:"application/json"})),changeUrl=state.file.changeContent?URL.createObjectURL(new Blob([state.file.changeContent],{type:"application/json"})):null;setDownload({file:state.file,url,changeUrl});return()=>{URL.revokeObjectURL(url);if(changeUrl)URL.revokeObjectURL(changeUrl);};}
    catch {setDownload({file:state.file,error:true});}
  },[state.file]);
  const errors={denied:text.export_read_denied,disabled:text.resource_reads_are_disabled,missing:text.queue_declaration_not_found,changed:text.declaration_changed_or_cannot_be_reconstructed_refresh,invalid:text.export_evidence_is_invalid_or_differs_from,limit:text.export_exceeds_the_size_limit_no_partial,unavailable:text.export_is_unavailable_retry_explicitly};
  return <section aria-label={text.queue_declaration_export}>
    <h3>{text.export_one_queue_declaration}</h3>
    <p>{text.downloads_configuration_only_not_messages_or_a}</p>
    <p>{text.labels_are_excluded_by_default_names_and}</p>
    {!document?<p role="alert">{text.this_declaration_cannot_be_faithfully_exported}</p>:<>
      <label><input type="checkbox" checked={includeLabels} onChange={event=>{model.clear();setIncludeLabels(event.target.checked);}}/>{text.include_all_free_form_labels_may_contain}</label>
      <button type="button" disabled={state.phase==="loading"} onClick={()=>void model.prepare(includeLabels)}>{text.prepare_declaration_download}</button>
      {state.phase==="loading"&&<><p role="status">{text.reading_and_verifying_export}</p><button type="button" onClick={()=>model.clear()}>{text.cancel_export}</button></>}
      {state.failure&&<p role="alert">{errors[state.failure]}</p>}
      {state.file&&<><p>{text.source_etag}{state.file.etag} · {text.omitted_labels}{state.file.omitted}</p>
        <p>{text.prepared_from_a_saved_declaration_read_at}<time dateTime={state.file.readAt}>{state.file.readAt}</time></p>
        {download?.file===state.file&&(download.error?<p role="alert">{text.browser_could_not_prepare_the_local_file}</p>:<><a href={download.url} download={state.file.name}>{text.download_queue_json}</a>{download.changeUrl&&<><p>{text.the_versioned_change_package_includes_all_labels}</p><a href={download.changeUrl} download={state.file.changeName}>{text.download_editable_change_package}</a></>}</>)}
      </>}
    </>}
  </section>;
}
