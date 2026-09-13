import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {createGlobalConsumers} from "./global-consumers.mjs";
import {globalConsumerLabels} from "./global-consumer-labels.mjs";
import {consumerDetailURL, globalConsumersURL} from "./routes.mjs";
import {ReadOnlyCollectionTable,OffsetPagination} from "./collection-table.jsx";

export function GlobalConsumers({api, language, route, router, canRefresh}) {
  const text = globalConsumerLabels(language);
  const model = useMemo(() => createGlobalConsumers(api), [api]);
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const [draft, setDraft] = useState({q: route.query.q, queue: route.query.queue, stream: route.query.stream});
  const [collecting, setCollecting] = useState(false);
  const [collectionFailure, setCollectionFailure] = useState(null);

  useEffect(() => {
    setDraft({q: route.query.q, queue: route.query.queue, stream: route.query.stream});
    void model.load(route.query);
    return () => model.clear();
  }, [model, route.query.q, route.query.queue, route.query.stream, route.query.mode, route.query.state, route.query.order, route.query.offset, route.query.limit, route.query.generation]);

  const navigate = changes => {
    const resets = ["q", "queue", "stream", "mode", "state", "order", "limit"].some(key => Object.hasOwn(changes, key));
    router.navigate(globalConsumersURL({...route.query, ...changes, ...(resets ? {offset: 0, generation: ""} : {})}));
  };
  const collect = async () => {
    setCollecting(true);
    setCollectionFailure(null);
    const result = await model.refreshCollection();
    setCollecting(false);
    if (!result.ok) {
      setCollectionFailure(result.kind);
      return;
    }
    void model.load({...route.query, offset: 0, generation: ""});
  };
  const counter = value => value === null ? text.unknown : String(value);
  const failureFor = kind => text.failures[kind] ?? text.failures.fallback;
  const page = state.page;
  const columns = [{key:"consumer",label:"Consumer"},{key:"stream",label:"Stream"},{key:"queue",label:"Queue"},{key:"mode",label:text.mode},{key:"state",label:text.state},{key:"ownership",label:text.ownership},{key:"pending",label:text.pending},{key:"ack",label:text.ackPending}];

  return <section aria-labelledby="global-consumers-heading">
    <h2 id="global-consumers-heading">{text.title}</h2>
    <p>{text.description}</p>
    <form className="list-controls consumer-list-controls" autoComplete="off" onSubmit={event => { event.preventDefault(); navigate(draft); }}>
      <label htmlFor="consumer-search">{text.contains}</label>
      <input id="consumer-search" name="consumer-search" type="search" value={draft.q} onChange={event => setDraft({...draft, q: event.target.value})}/>
      <label htmlFor="consumer-queue">Queue</label>
      <input id="consumer-queue" name="consumer-queue" type="search" value={draft.queue} onChange={event => setDraft({...draft, queue: event.target.value})}/>
      <label htmlFor="consumer-stream">Stream</label>
      <input id="consumer-stream" name="consumer-stream" type="search" value={draft.stream} onChange={event => setDraft({...draft, stream: event.target.value})}/>
      <button>{text.filter}</button>
      <label htmlFor="consumer-mode">{text.mode}</label>
      <select id="consumer-mode" value={route.query.mode} onChange={event => navigate({mode: event.target.value})}>
        <option value="">{text.all}</option><option value="pull">Pull</option><option value="push">Push</option>
      </select>
      <label htmlFor="consumer-state">{text.state}</label>
      <select id="consumer-state" value={route.query.state} onChange={event => navigate({state: event.target.value})}>
        <option value="">{text.all}</option><option value="present">Present</option><option value="missing">Missing</option><option value="mismatched">Mismatched</option>
      </select>
      <label htmlFor="consumer-order">{text.identityOrder}</label>
      <select id="consumer-order" value={route.query.order} onChange={event => navigate({order: event.target.value})}>
        <option value="asc">{text.ascending}</option><option value="desc">{text.descending}</option>
      </select>
      <label htmlFor="consumer-limit">{text.pageSize}</label>
      <select id="consumer-limit" value={route.query.limit} onChange={event => navigate({limit: Number(event.target.value)})}>
        {[25, 50, 100, 200].map(value => <option key={value}>{value}</option>)}
      </select>
    </form>
    {canRefresh
      ? <button disabled={collecting} onClick={() => void collect()}>{collecting ? text.collectingAction : text.collectNewGeneration}</button>
      : <p>{text.auditorCollection}</p>}
    {collectionFailure && <p role="alert">{failureFor(collectionFailure)}</p>}
    {state.phase === "loading" && <p role="status">{text.loading}</p>}
    {state.failure && <p role="alert">{failureFor(state.failure.kind)}</p>}
    {state.failure?.kind === "generation-changed" && <button onClick={() => router.navigate(globalConsumersURL({...route.query, offset: 0, generation: ""}))}>{text.resetFirst}</button>}
    {page && <>
      <p role="status">{text.filteredTotal}: {page.total} · {text.generation}: <code>{page.generation_id}</code> · <time dateTime={page.completed_at}>{page.completed_at}</time> · {page.state}</p>
      {page.state === "stale" && <p role="alert">{text.stale}</p>}
      {page.items.length
        ? <ReadOnlyCollectionTable label={text.list} caption={text.caption} columns={columns} className="consumer-table">{page.items.map(row => <tr key={`${row.stream}\0${row.name}`}>
              <th scope="row"><a href={consumerDetailURL(row.stream, row.name)} onClick={event => {
                if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
                  event.preventDefault();
                  router.navigate(event.currentTarget.getAttribute("href"));
                }
              }}>{row.name}</a></th>
              <td>{row.stream}</td><td>{row.queue || "—"}</td><td>{row.mode}</td><td>{row.status}</td><td>{row.ownership}</td><td>{counter(row.pending)}</td><td>{counter(row.ack_pending)}</td>
            </tr>)}</ReadOnlyCollectionTable>
        : <p>{page.total === 0 ? text.noMatches : text.emptyPage}</p>}
      <OffsetPagination label={text.pagination} offset={page.offset} limit={page.limit} returned={page.items.length} total={page.total} previousLabel={text.previous} nextLabel={text.next} onPrevious={() => navigate({offset: Math.max(0, page.offset - page.limit), generation: page.generation_id})} onNext={() => navigate({offset: page.offset + page.limit, generation: page.generation_id})}/>
    </>}
  </section>;
}
