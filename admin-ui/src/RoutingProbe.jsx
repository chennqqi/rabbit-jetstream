import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createRoutingProbe} from "./routing-probe.mjs";
import {routingLabels} from "./routing-labels.mjs";
import {routingProbeLabels} from "./routing-probe-labels.mjs";

export function RoutingProbe({api,name,revision,etag,language}){
  const model=useMemo(()=>createRoutingProbe(api,name,revision,etag),[api,name,revision,etag]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),[mode,setMode]=useState("subject"),[input,setInput]=useState({subject:"",exchange:"",type:"direct",routingKey:""});
  useEffect(()=>()=>model.clear(),[model]);
  const text=routingProbeLabels(language),labels=routingLabels(language);
  function edit(key,value){model.clear();setInput(old=>({...old,[key]:value,...(key==="type"&&value==="fanout"?{routingKey:""}:{})}));}
  const errors={denied:text.read_denied_check_credentials_and_permissions,disabled:text.resource_reads_are_disabled,missing:text.queue_declaration_not_found,changed:text.declaration_changed_or_cannot_be_reconstructed_refresh,query:text.invalid_or_unsupported_probe_input_priority_queues,limit:text.routing_evidence_exceeds_the_response_limit_no,invalid:text.invalid_response_no_routing_conclusion_is_available,unavailable:text.read_unavailable_retry_explicitly_this_does_not};
  const result=state.result,list=items=>items.length?<ul>{items.map((value,i)=><li key={i}><code>{value}</code></li>)}</ul>:text.none;
  return <section className="routing-probe" aria-label={text.routing_probe}>
    <h3>{text.test_routing_without_publishing}</h3>
    <p>{text.checks_the_saved_declaration_not_editor_drafts}</p>
    <form onSubmit={event=>{event.preventDefault();void model.run(mode==="subject"?{subject:input.subject}:{exchange:input.exchange,type:input.type,routingKey:input.routingKey});}}>
      <label>{text.probe_mode}<select aria-label={text.probe_mode} value={mode} onChange={event=>{model.clear();setMode(event.target.value);}}><option value="subject">{text.literal_subject}</option><option value="exchange">{text.exchange_target}</option></select></label>
      {mode==="subject"?<label>Subject<input required value={input.subject} onChange={event=>edit("subject",event.target.value)} spellCheck="false"/></label>:<>
        <label>{labels.exchange}<input required value={input.exchange} onChange={event=>edit("exchange",event.target.value)} spellCheck="false"/></label>
        <label>{text.exchange_type}<select aria-label={text.exchange_type} value={input.type} onChange={event=>edit("type",event.target.value)}>{["direct","topic","fanout"].map(type=><option key={type}>{type}</option>)}</select></label>
        <label>{text.routing_key}<input required={input.type==="direct"} disabled={input.type==="fanout"} value={input.routingKey} onChange={event=>edit("routingKey",event.target.value)} spellCheck="false"/></label>
      </>}
      <button disabled={state.phase==="loading"}>{text.check_routing}</button>
      {state.phase==="loading"&&<button type="button" onClick={()=>model.clear()}>{text.cancel}</button>}
    </form>
    {state.phase==="loading"&&<p role="status">{text.checking_saved_declaration}</p>}
    {state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {result&&<>
      <p role="status">{result.matchedStreamSubjects.length?text.generated_stream_subject_matched_this_is_not:text.no_generated_stream_subject_matched_binding_matches}</p>
      <dl><dt>Queue</dt><dd>{result.queue}</dd><dt>{text.declaration_revision}</dt><dd>{result.revision}</dd><dt>{text.read_completed_local_time}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd><dt>{text.resolved_subject}</dt><dd>{result.subject}</dd><dt>Stream</dt><dd>{result.stream}</dd><dt>{text.generated_stream_subjects}</dt><dd>{list(result.streamSubjects)}</dd><dt>{text.matched_stream_subjects}</dt><dd>{list(result.matchedStreamSubjects)}</dd></dl>
      {result.bindings.length?<div className="table-scroll" role="region" tabIndex={0} aria-label={text.binding_results_scroll_horizontally}><table><caption>{text.binding_translation_and_matches}</caption><thead><tr>{[labels.exchange,labels.type,labels.keys,labels.generatedSubjects,labels.matchedSubjects].map(label=><th key={label} scope="col">{label}</th>)}</tr></thead><tbody>{result.bindings.map((row,i)=><tr key={i}><td>{row.exchange}</td><td>{row.type}</td><td>{list(row.keys)}</td><td>{list(row.subjects)}</td><td>{list(row.matchedSubjects)}</td></tr>)}</tbody></table></div>:<p>{text.no_declared_exchange_bindings}</p>}
    </>}
  </section>;
}
