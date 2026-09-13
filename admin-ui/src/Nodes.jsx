import React,{useMemo,useSyncExternalStore} from "react";
import {nodeSourceAvailable,nodeJSMetric} from "./nodes.mjs";
import {createNodeRefresh} from "./node-refresh.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {nodeDetailURL,nodeConnectionsURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {nodeLabels} from "./node-labels.mjs";

export function Nodes({api,language,route,router,refreshSeconds=10}){
  const text=nodeLabels(language),view=useMemo(()=>createNodeRefresh(api),[api,route.kind,route.id]),{model,refresh}=view;
  const state=useSyncExternalStore(model.subscribe,model.snapshot),{paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  const matches=state.snapshot?.nodes.filter(node=>node.id===route.id)??[],selected=matches.length===1?matches[0]:null;
  let connectionsURL=null;try{if(selected)connectionsURL=nodeConnectionsURL(route.id);}catch{}
  const status=node=>text.statuses[node.status],count=(node,key)=>nodeSourceAvailable(node,"varz")?consumerCounter({observed:node},key)??text.unknown:text.unknown;
  const follow=(event,url)=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(url);}};
  return <section aria-label={text.view}>
    <h2>{route.kind==="node"?`Node: ${route.id}`:text.title}</h2><p>{text.description}</p>
    <p>{paused?text.paused:refreshSeconds===0?text.manual:`${text.automaticPrefix} ${refreshSeconds} ${text.automaticSuffix}`}</p>
    <button disabled={paused||state.phase==="loading"} onClick={()=>void refresh.refresh()}>{text.refresh}</button>
    {state.phase==="loading"&&<p role="status">{state.snapshot?text.refreshing:text.loading}</p>}{state.failure&&<p role="alert">{text.failures[state.failure]??text.failures.fallback}</p>}
    {state.snapshot&&<>{(state.phase==="loading"||state.failure)&&<p role="status">{text.retained}</p>}<p>{text.readCompleted}: <time dateTime={state.readAt}>{state.readAt}</time></p><StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale}/>
      {route.kind==="nodes"?<>{state.snapshot.total===0?<p>{text.noEndpoints}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={text.collection}><table><caption className="visually-hidden">{text.endpointList}</caption><thead><tr><th>Node ID</th><th>{text.endpoint}</th><th>{text.readStatus}</th></tr></thead><tbody>{state.snapshot.nodes.map((node,index)=><tr key={`${node.endpoint}-${index}`}><th scope="row">{node.id?<a href={nodeDetailURL(node.id)} onClick={event=>follow(event,nodeDetailURL(node.id))}>{node.id}</a>:text.unknown}</th><td>{node.endpoint}</td><td>{status(node)}{node.errors?.map((error,i)=><p key={i}>{error}</p>)}</td></tr>)}</tbody></table></div>}</>:!selected?<p role="alert">{text.notUnique}</p>:<>
        <p>{status(selected)}</p>{connectionsURL&&<p><a href={connectionsURL} onClick={event=>follow(event,connectionsURL)}>{text.viewConnections}</a></p>}
        <dl>{[[text.fields.name,selected.name??text.unknown],[text.fields.endpoint,selected.endpoint],[text.fields.version,selected.version??text.unknown],[text.fields.goVersion,selected.go_version??text.unknown],[text.fields.uptime,selected.uptime??text.unknown],[text.fields.clusterName,selected.cluster_name??text.unknown],[text.fields.memoryBytes,count(selected,"memory_bytes")],[text.fields.cpuCores,count(selected,"cores")],[text.fields.connections,count(selected,"connections")],[text.fields.subscriptions,count(selected,"subscriptions")],[text.fields.slowConsumers,count(selected,"slow_consumers")],[text.fields.inMessages,count(selected,"in_messages")],[text.fields.outMessages,count(selected,"out_messages")],["CPU %",nodeSourceAvailable(selected,"varz")&&typeof selected.cpu_percent==="number"&&Number.isFinite(selected.cpu_percent)&&selected.cpu_percent>=0?String(selected.cpu_percent):text.unknown],[text.fields.serverObservation,nodeSourceAvailable(selected,"varz")&&selected.observed_at&&!selected.observed_at.startsWith("0001-")?selected.observed_at:text.unknown]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{String(value)}</dd></React.Fragment>)}</dl>
        <p>{text.cumulativeNote}</p><h3>{text.connectedPeers}</h3><p>{nodeSourceAvailable(selected,"routez")&&Array.isArray(selected.connected_peers)?(selected.connected_peers.length?selected.connected_peers.join(", "):text.noPeers):text.unknown}</p>
        <h3>{text.sourceObservations}</h3><dl>{["varz","routez","jsz"].map(source=><React.Fragment key={source}><dt>{source}</dt><dd>{nodeSourceAvailable(selected,source)?text.readSucceeded:text.sourceUnavailable} · {selected.sources?.[source]?.read_at??text.unknown}</dd></React.Fragment>)}</dl>
        <h3>JetStream</h3><p>{text.enabled}: {nodeSourceAvailable(selected,"varz")&&typeof selected.jetstream?.enabled==="boolean"?String(selected.jetstream.enabled):text.unknown}</p><p>{text.metricNote}</p>
        <dl aria-label={text.metrics}>{Object.entries(text.jsFields).map(([key,label])=><React.Fragment key={key}><dt>{label}</dt><dd>{nodeJSMetric(selected,key)??text.unknown}</dd></React.Fragment>)}</dl>
        {nodeSourceAvailable(selected,"jsz")&&<details><summary>{text.rawJetStream}</summary><pre className="declaration-json">{stringifyJSON(selected.jetstream)}</pre></details>}{selected.errors?.map((error,index)=><p role="alert" key={index}>{error}</p>)}</>}
    </>}
  </section>;
}
