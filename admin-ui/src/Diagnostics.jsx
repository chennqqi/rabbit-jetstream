import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createDiagnostics} from "./diagnostics.mjs";
import {diagnosticsLabels} from "./diagnostics-labels.mjs";
export function Diagnostics({api,language}){
  const zh=language==="zh",text=diagnosticsLabels(language),model=useMemo(()=>createDiagnostics(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),job=state.job;
  useEffect(()=>()=>model.clear(),[model]);
  useEffect(()=>{if(job?.state!=="collecting")return;const timer=setTimeout(()=>void model.refresh(),1000);return()=>clearTimeout(timer);},[job,model]);
  return <section aria-label={text.diagnostics}><h2>{text.diagnostics}</h2>
    <p>{text.creates_a_temporary_metadata_only_zip_in}</p>
    <p>{text.one_collection_runs_at_a_time_at}</p>
    <button type="button" disabled={state.phase==="creating"||job?.state==="collecting"} onClick={()=>void model.create()}>{state.phase==="creating"?text.requesting:text.create_metadata_bundle}</button>
    {state.error&&<p role="alert">{text.the_operation_failed_or_its_response_was}</p>}
    {job&&<section aria-label={text.current_diagnostic_job}><h3>{text.current_job}</h3><dl><dt>ID</dt><dd><code>{job.id}</code></dd><dt>{text.state}</dt><dd>{job.state}</dd><dt>{text.created}</dt><dd><time dateTime={job.createdAt}>{job.createdAt}</time></dd><dt>{text.expires}</dt><dd><time dateTime={job.expiresAt}>{job.expiresAt}</time></dd></dl>
      {job.state==="collecting"&&<><p role="status">{text.collecting_bounded_metadata}</p><button type="button" onClick={()=>void model.cancel()}>{text.cancel_collection}</button></>}
      {job.state==="partial"&&<p role="alert">{text.partial_bundle_one_or_more_sources_were}</p>}
      {job.manifest&&<table><caption>{text.bundle_manifest}</caption><thead><tr><th>{text.source}</th><th>{text.file}</th><th>{text.bytes}</th><th>{text.result}</th></tr></thead><tbody>{job.manifest.entries.map(entry=><tr key={entry.source}><th scope="row">{entry.source}</th><td>{entry.file??"—"}</td><td>{entry.size??"—"}</td><td>{entry.error??text.included}</td></tr>)}</tbody></table>}
      {["ready","partial"].includes(job.state)&&<button type="button" onClick={()=>void model.download()}>{text.prepare_download}</button>}
      {state.download&&<p><a href={state.download.url} download={state.download.filename}>{text.save_diagnostic_zip}</a> · {state.download.size} {text.bytes_2}. {text.server_completion_does_not_prove_the_browser}</p>}
      {["failed","cancelled","expired"].includes(job.state)&&<p role="alert">{text.no_archive_is_available_for_this_terminal}</p>}
    </section>}
  </section>;
}
