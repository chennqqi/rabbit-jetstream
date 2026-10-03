import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {createStreamRefresh} from "./stream-refresh.mjs";
import {createConsumerCollectionRefresh} from "./consumer-collection-refresh.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {consumerDetailURL, streamDetailURL} from "./routes.mjs";
import {stringifyJSON} from "./api.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {RouteLink} from "./route-link.jsx";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";
import {humanBytes} from "./format.mjs";
import {PageNotes,RefreshStatus} from "./DisplayValues.jsx";
import {streamLabels} from "./stream-labels.mjs";

export function StreamDetail({api, name, language, route, router, refreshSeconds=10}) {
  const accessible=streamLabels(language), unknown=accessible.unknown;
  const observation = useMemo(() => createStreamRefresh(api, name), [api, name]);
  const {model,refresh:stateRefresh}=observation;
  const query=route.query;
  const collection=useMemo(()=>createConsumerCollectionRefresh(api,"stream",name,query),[api,name,query.q,query.mode,query.order,query.offset,query.limit]);
  const {model:list,refresh}=collection;
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const consumers = useSyncExternalStore(list.subscribe, list.snapshot);
  const [search, setSearch] = useState(route.query.q);
  useEffect(()=>setSearch(query.q),[query.q]);
  useRefreshLoop({loop:stateRefresh,seconds:refreshSeconds,cleanup:()=>observation.clear()});
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>collection.clear()});
  const resource = state.resource, page = consumers.resource;
  const failure = value => accessible.failures[value] ?? accessible.failures.fallback;
  const counter = (row, field) => field === "bytes" ? (humanBytes(Number(consumerCounter({observed: row}, field))) ?? consumerCounter({observed: row}, field) ?? unknown) : consumerCounter({observed: row}, field) ?? unknown;
  function changePage(offset) {
    changeQuery({offset});
  }
  function changeQuery(changes) {
    const reset = ["q", "mode", "order", "limit"].some(key => Object.hasOwn(changes, key));
    const query = {...route.query, ...changes, ...(reset ? {offset: 0} : {})};
    const path = `${streamDetailURL(name)}?${new URLSearchParams(query)}`;
    if (!router.navigate(path)) void refresh.refresh();
  }
  return <section aria-label={accessible.detail}>
    <ResourceBreadcrumb trail={[{label:accessible.streamList,href:"/admin/streams"},{label:name}]} router={router} language={language}/>
    <h2>Stream: {name}</h2>
    <button disabled={paused||state.phase === "loading"} onClick={() => void stateRefresh.refresh()}>{accessible.refreshStream}</button>
    <RefreshStatus readAt={state.readAt} paused={paused} seconds={refreshSeconds} language={language}/>
    {state.phase === "loading" && <p role="status">{accessible.loadingStream}</p>}
    {state.failure && <p role="alert">{failure(state.failure)}</p>}
    {resource && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{accessible.retainedStream}</p>}
      <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={accessible.staleStream}/>
      <h3>{accessible.observedConfiguration}</h3>
      <dl>{[[accessible.subjects, resource.subjects?.join(", ") ?? unknown], [accessible.storage, resource.storage ?? unknown], [accessible.configuredReplicas, resource.replicas ?? unknown], [accessible.retention, resource.retention ?? unknown], [accessible.discard, resource.discard ?? unknown]].map(([label, value]) => <React.Fragment key={label}><dt>{label}</dt><dd>{String(value)}</dd></React.Fragment>)}</dl>
      <h3>{accessible.observedState}</h3>
      <PageNotes language={language}><p>{accessible.stateNote}</p></PageNotes>
      <dl>{[[accessible.storedMessages, "messages"], [accessible.bytes, "bytes"], [accessible.consumerCount, "consumers"], [accessible.firstSequence, "first_sequence"], [accessible.lastSequence, "last_sequence"]].map(([label, field]) => <React.Fragment key={field}><dt>{label}</dt><dd>{counter(resource, field)}</dd></React.Fragment>)}</dl>
      <time dateTime={state.readAt}>{state.readAt}</time>
      <ReplicaEvidence stream={resource} language={language}/>
      <details><summary>{accessible.fullResponse}</summary><pre className="declaration-json">{stringifyJSON(resource)}</pre></details>
    </>}
    <h3>{accessible.observedConsumers}</h3>
    <p>{paused?accessible.collectionPaused:refreshSeconds===0?accessible.collectionManual:`${accessible.collectionAutoPrefix} ${refreshSeconds} ${accessible.collectionAutoSuffix}`}</p>
    <p>{accessible.collectionDescription}</p>
    <form className="list-controls" onSubmit={event => {event.preventDefault(); changeQuery({q: search});}}>
      <label htmlFor="stream-consumer-search">{accessible.consumerSearch}</label>
      <input id="stream-consumer-search" value={search} onChange={event => setSearch(event.target.value)} />
      <button type="submit">{accessible.filterConsumers}</button>
      <label htmlFor="stream-consumer-mode">{accessible.consumerMode}</label>
      <select id="stream-consumer-mode" value={route.query.mode} onChange={event => changeQuery({mode: event.target.value})}><option value="">{accessible.allModes}</option><option value="pull">Pull</option><option value="push">Push</option></select>
      <label htmlFor="stream-consumer-order">{accessible.consumerSort}</label>
      <select id="stream-consumer-order" value={route.query.order} onChange={event => changeQuery({order: event.target.value})}><option value="asc">{accessible.nameAscending}</option><option value="desc">{accessible.nameDescending}</option></select>
      <label htmlFor="stream-consumer-limit">{accessible.consumerPageSize}</label>
      <select id="stream-consumer-limit" value={route.query.limit} onChange={event => changeQuery({limit: Number(event.target.value)})}>{[...new Set([1, 25, 50, 100, 200, route.query.limit])].sort((a,b) => a-b).map(limit => <option key={limit}>{limit}</option>)}</select>
    </form>
    <button disabled={paused||consumers.phase === "loading"} onClick={() => void refresh.refresh()}>{accessible.refreshConsumers}</button>
    {consumers.phase === "loading" && <p role="status">{page?accessible.refreshingConsumers:accessible.loadingConsumers}</p>}
    {consumers.failure && <p role="alert">{failure(consumers.failure)}</p>}
    {page && <>
      {(consumers.phase==="loading"||consumers.failure)&&<p role="status">{accessible.retainedConsumers}</p>}
      <StaleEvidence readAt={consumers.readAt} paused={paused} failure={consumers.failure} clock={clock} note={accessible.staleConsumers}/>
      <p>{accessible.filteredTotal}: {page.total} · <time dateTime={consumers.readAt}>{consumers.readAt}</time></p>
      {page.items.length ? <div className="table-scroll" tabIndex="0" role="region" aria-label={accessible.consumers}><table><caption className="visually-hidden">{accessible.consumerCaption}</caption><thead><tr><th>Consumer</th><th>{accessible.mode}</th><th>{accessible.pending}</th><th>{accessible.ackPending}</th></tr></thead><tbody>{page.items.map(row => <tr key={row.name}><th scope="row"><RouteLink href={consumerDetailURL(name, row.name)} router={router}>{row.name}</RouteLink></th><td>{row.mode}</td><td>{counter(row, "pending")}</td><td>{counter(row, "ack_pending")}</td></tr>)}</tbody></table></div> : <p>{page.total === 0 ? accessible.noMatches : accessible.emptyPage}</p>}
      <nav aria-label={accessible.pagination}><button disabled={page.offset === 0} onClick={() => changePage(page.items.length ? Math.max(0, page.offset - page.limit) : Math.max(0, Math.floor((page.total - 1) / page.limit) * page.limit))}>{accessible.previous}</button><span>{page.items.length ? `${page.offset + 1}–${page.offset + page.items.length}` : "0"} / {page.total}</span><button disabled={page.offset + page.items.length >= page.total} onClick={() => changePage(page.offset + page.limit)}>{accessible.next}</button></nav>
    </>}
  </section>;
}
