import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createSummaryRefresh} from "./summary-refresh.mjs";
import {createAuditList} from "./audit-list.mjs";
import {readAuditQuery,auditURL} from "./audit-query.mjs";
import {streamDetailURL,queueTabURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";
import {SummaryMetrics} from "./SummaryMetrics.jsx";
import {ExactDuration,ReadTime} from "./DisplayValues.jsx";
import {summaryLabels} from "./summary-labels.mjs";
import {queuePanelsLabels} from "./queue-panels-labels.mjs";

export function QueueSummary({api,plan,language,router,route,refreshSeconds=10}) {
  const zh=language==="zh",name=plan.stream.name,accessible=summaryLabels(language),text=queuePanelsLabels(language);
  const view=useMemo(()=>createSummaryRefresh(api,plan),[api,plan]),{stream,consumer,refresh}=view;
  const state=useSyncExternalStore(stream.subscribe,stream.snapshot),consumerState=useSyncExternalStore(consumer.subscribe,consumer.snapshot);
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  const busy=paused||state.phase==="loading"||consumerState.phase==="loading";
  return <><div className="queue-summary-grid"><section className="summary-evidence"><header className="observation-heading"><h3>{text.observedStream}: {name}</h3>
    <button disabled={busy} onClick={()=>void refresh.refresh("stream")}>{text.refreshObservation}</button>
    </header><details className="reading-help"><summary>{text.guide}</summary><p>{text.guideText}</p></details>
    {state.phase==="loading"&&<p role="status">{text.loading}</p>}
    <p>{paused?text.paused:refreshSeconds===0?text.manual:`${text.autoPrefix} ${refreshSeconds} ${text.autoSuffix}`}</p>
    {state.failure&&<p role="alert">{state.failure==="missing"?text.streamMissing:text.streamFailure}</p>}
    {state.resource&&(state.phase==="loading"||state.failure)&&<p role="status">{text.retained}</p>}
    <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale}/>
    <SummaryMetrics scope={view.scope} consumerState={consumerState} streamState={state} language={language} router={router} disabled={busy} paused={paused} clock={clock} onRefresh={()=>void refresh.refresh("consumer")}/>
    {state.resource&&<><ReplicaEvidence stream={state.resource} desired={plan.stream.replicas} language={language}/><dl>{[["bytes",text.storedBytes],["consumers",text.observedConsumers]].map(([field,label])=><React.Fragment key={field}><dt>{label}</dt><dd>{consumerCounter({observed:state.resource},field)??text.unknown}</dd></React.Fragment>)}</dl><ReadTime value={state.readAt} language={language}/></>}
    <p><a href={streamDetailURL(name)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(streamDetailURL(name));}}}>{text.viewStream}</a></p>
  </section><section className="summary-config" aria-label={text.configuration}><h3>{text.configuration}</h3><p>{text.declaredNote}</p><dl>{[["Stream",name],[text.storage,plan.stream.storage],[text.replicas,plan.stream.replicas],[text.maxAge,plan.stream.maxAgeNanos,"duration"],[text.ackWait,plan.consumer?.ackWaitNanos,"duration"],[text.maxDeliveries,plan.consumer?.maxDeliver]].map(([label,value,kind])=><div key={label}><dt>{label}</dt><dd>{kind==="duration"?<ExactDuration value={value} language={language}/>:value===undefined||value===null?text.unknown:String(value)}</dd></div>)}</dl><p>{text.ageNote}</p></section></div>
    <section aria-label={accessible.diagnostics}><h3>{text.investigate}</h3><p>{text.investigateText}</p><a href={queueTabURL(plan.queue,"consumers",route?.consumerQuery)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(event.currentTarget.getAttribute("href"));}}}>{text.viewConsumers}</a></section></>;
}

export function QueueEvents({api,name,language,router,route}) {
  const text=queuePanelsLabels(language),model=useMemo(()=>createAuditList(api),[api]);
  const query=useMemo(()=>readAuditQuery(new URLSearchParams({resource:name}).toString()),[name]);
  const before=route.eventBefore??null,state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load({...query,before});return()=>model.clear();},[model,query,before]);
  return <section><h3>{text.auditTitle}</h3>
    <p>{text.auditText}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load({...query,before})}>{text.refreshEvents}</button>
    {state.phase==="loading"&&<p role="status">{text.loadingEvents}</p>}
    {state.failure&&<p role="alert">{text.auditFailure}</p>}
    {state.page&&<><p>{text.matches}: {state.page.items.length} · {state.readAt}</p>
      {state.page.items.length===0&&<p>{text.noMatches}</p>}
      {state.page.items.map(event=><details key={String(event.sequence)}><summary>{event.time} · {event.action} · {event.phase} · {event.id}</summary><pre className="declaration-json">{stringifyJSON(event)}</pre></details>)}
      <button disabled={state.page.nextBefore===null} onClick={()=>router.navigate(queueTabURL(name,"events",route.consumerQuery,String(state.page.nextBefore)))}>{text.olderEvents}</button></>}
    <p><a href={auditURL(query)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(auditURL(query));}}}>{text.openAudit}</a></p>
  </section>;
}
