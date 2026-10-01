import React,{useMemo,useSyncExternalStore} from "react";
import {createOverview} from "./overview.mjs";
import {createNodes} from "./nodes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {PageNotes,RefreshStatus} from "./DisplayValues.jsx";
import {systemLabels} from "./system-labels.mjs";
import {MetricHistory} from "./MetricHistory.jsx";
import {overviewLabels} from "./overview-labels.mjs";
import {humanBytes} from "./format.mjs";

export function Overview({api,language,router,refreshSeconds=10}) {
  const text=overviewLabels(language),unknown=text.unknown,accessible=systemLabels(language);
  const model=useMemo(()=>createOverview(api),[api]),nodes=useMemo(()=>createNodes(api,{retainOnRefresh:true}),[api]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),monitor=useSyncExternalStore(nodes.subscribe,nodes.snapshot);
  const refresh=useMemo(()=>createRefreshLoop(async()=>{
    await Promise.all([model.load(),nodes.load()]);
    return !model.snapshot().info.failure&&!model.snapshot().queues.failure&&!nodes.snapshot().failure&&!nodes.snapshot().snapshot?.nodes.some(node=>node.status!=="available");
  }),[model,nodes]);
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>{model.clear();nodes.clear();}});
  const count=(object,key)=>consumerCounter({observed:object},key)??unknown;
  const link=(path,label)=><a href={path} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(path);}}}>{label}</a>;
  function evidence(source) {return <>
    {source.phase==="loading"&&<p role="status">{source.readAt?text.refreshing:text.loading}</p>}
    {source.failure&&<p role="alert">{text.failures[source.failure]??text.failures.fallback}</p>}
    {source.readAt&&<p>{text.readCompleted}: <time dateTime={source.readAt}>{source.readAt}</time></p>}
    <StaleEvidence readAt={source.readAt} paused={paused} failure={source.failure} clock={clock} note={text.stale}/>
  </>;}
  const issues=monitor.snapshot?.nodes.filter(node=>node.status!=="available")??[];
  return <section aria-label={accessible.overview}>
    <h2>{text.title}</h2>
    <RefreshStatus readAt={state.readAt} paused={paused} seconds={refreshSeconds} language={language}/>
    <PageNotes language={language}><p>{text.description}</p><p>{text.freshness}</p></PageNotes>
    <button onClick={()=>void refresh.refresh()} disabled={paused||state.info.phase==="loading"||state.queues.phase==="loading"||monitor.phase==="loading"}>{text.refresh}</button>
    <section aria-label={accessible.monitoring}><h3>{text.monitoringTitle}</h3>
      {evidence(monitor)}
      {monitor.snapshot&&<>
        {monitor.snapshot.total===0?<p>{text.noEndpoints}</p>:<p>{text.configuredEndpoints}: {monitor.snapshot.total} · {text.failedReads}: {issues.length}</p>}
        {issues.length>0&&<ul>{issues.map((node,index)=><li key={index}>{node.endpoint}: {node.status==="unavailable"?text.unavailable:text.partialFailure}</li>)}</ul>}
        <p>{text.coverageNote}</p>
      </>}
      {link("/admin/nodes",text.viewNodes)}
    </section>
    <section aria-label={accessible.account}><h3>{text.accountTitle}</h3>
      {evidence(state.info)}
      {state.info.value&&<>
        <dl>{[[text.fields.serviceName,state.info.value.name],[text.fields.serviceVersion,state.info.value.version],[text.fields.uptime,count(state.info.value,"uptime_seconds")],[text.fields.streams,count(state.info.value.jetstream,"streams")],[text.fields.consumers,count(state.info.value.jetstream,"consumers")],[text.fields.memory,humanBytes(state.info.value.jetstream?.memory_used)??unknown],[text.fields.storage,humanBytes(state.info.value.jetstream?.storage_used)??unknown]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{value}</dd></React.Fragment>)}</dl>
        <p>{text.accountNote}</p>
      </>}
      {link("/admin/streams",text.viewStreams)}
    </section>
    <section aria-label={accessible.queues}><h3>{text.queuesTitle}</h3>
      {evidence(state.queues)}
      {state.queues.value&&<p>{text.declarationTotal}: {state.queues.value.total}</p>}
      <p>{text.queuesNote}</p>
      {link("/admin/queues",text.viewQueues)}
    </section>
    <MetricHistory api={api} language={language}/>
  </section>;
}
