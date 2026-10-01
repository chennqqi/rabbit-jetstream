import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createDLQDiagnostics,dlqTarget} from "./dlq-diagnostics.mjs";
import {queueDetailURL,streamDetailURL} from "./routes.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {DLQEvidence} from "./DLQEvidence.jsx";
import {dLQDiagnosticsLabels} from "./dlqdiagnostics-labels.mjs";

export function DLQDiagnostics({api,plan,language,router,declarationETag,declarationReadAt}){
  const text=dLQDiagnosticsLabels(language);
  let target;try{target=dlqTarget(plan);}catch{return <section><h3>{text.dlq_diagnosis}</h3><p role="alert">{text.invalid_dlq_plan_no_related_resource_reads}</p></section>;}
  if(!target)return <section><h3>{text.dlq_diagnosis}</h3><p>{text.no_dead_letter_target_is_configured_in}</p></section>;
  return <ConfiguredDLQ api={api} plan={plan} language={language} router={router} target={target} declarationETag={declarationETag} declarationReadAt={declarationReadAt}/>;
}
function ConfiguredDLQ({api,plan,language,router,target,declarationETag,declarationReadAt}){
  const text=dLQDiagnosticsLabels(language),model=useMemo(()=>createDLQDiagnostics(api,plan),[api,plan]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const labels={unobserved:text.not_observed,loading:text.reading,available:text.observed_in_this_read,missing:text.not_found_in_this_read,denied:text.access_denied,invalid:text.incompatible_response,unavailable:text.read_unavailable};
  const link=(path,label)=><a href={path} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(path);}}}>{label}</a>;
  const status=source=><><p role={["missing","denied","invalid","unavailable"].includes(source.phase)?"alert":"status"}>{labels[source.phase]}</p>{source.readAt&&<ReadTime value={source.readAt} language={language}/>}</>;
  const controller=state.controller.value;
  return <section aria-label={text.dlq_diagnosis}><h3>{text.dlq_diagnosis}</h3>
    <p>{text.declared_target}: {link(queueDetailURL(target.queue),target.queue)} · <code>{target.mechanism}</code></p>
    <p>{text.read_only_one_target_hop_declaration_target}</p>
    <button disabled={Object.values(state).some(value=>value.phase==="loading")} onClick={()=>void model.load()}>{text.refresh_dlq_evidence}</button>
    <section aria-label={text.dlq_target_declaration}><h4>{text.target_declaration}</h4>{status(state.target)}{state.target.value&&<p>{text.declaration_etag}: <code>{state.target.etag??text.unreported}</code></p>}</section>
    <section aria-label={text.dlq_target_stream}><h4>{text.target_stream}</h4>{link(streamDetailURL(target.stream),target.stream)}{status(state.stream)}</section>
    <section aria-label={text.dlq_controller_evidence}><h4>{text.controller_process_all_queues}</h4>{status(state.controller)}{controller&&<dl>
      {[[text.instance,controller.instanceId],[text.enabled,String(controller.enabled)],[text.reports_leader,String(controller.leader)],[text.last_run,controller.lastRun],[text.last_successful_run,controller.lastSuccess],[text.error_reported,String(controller.errorReported)],[text.reported_processed,controller.dlqProcessed],[text.reported_moved,controller.dlqMoved],[text.reported_failed,controller.dlqFailed],[text.reported_ignored,controller.dlqIgnored]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{value===undefined?text.unreported:String(value)}</dd></React.Fragment>)}
    </dl>}<p>{text.counters_belong_to_this_controller_process_across}</p></section>
    <p>{text.these_are_processing_attempts_not_unique_messages}</p>
    <p>{text.ignored_counts_advisory_attempts_with_invalid_json}</p>
    <DLQEvidence plan={plan} state={state} declarationETag={declarationETag} declarationReadAt={declarationReadAt} language={language}/>
  </section>;
}
