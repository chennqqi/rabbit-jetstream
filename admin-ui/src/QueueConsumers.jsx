import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {consumerCounter} from "./queue-consumers.mjs";
import {createConsumerCollectionRefresh} from "./consumer-collection-refresh.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {consumerRefreshLabel} from "./consumer-refresh-label.mjs";
import {consumerDetailURL, queueConsumersURL, readConsumerQuery} from "./routes.mjs";
import {queueConsumersLabels} from "./queue-consumers-labels.mjs";

export function QueueConsumers({api, queue, language, declarationETag, route, router, refreshSeconds=10}) {
  const query=route.consumerQuery??readConsumerQuery();
  const view=useMemo(()=>createConsumerCollectionRefresh(api,"queue",queue,query),[api,queue,query.q,query.mode,query.order,query.offset,query.limit]);
  const {model,refresh}=view;
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const [search, setSearch] = useState(route.consumerQuery?.q ?? "");
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  useEffect(()=>setSearch(query.q),[query.q]);
  function changeQuery(changes) {
    const reset = ["q","mode","order","limit"].some(key => Object.hasOwn(changes,key));
    const query = {...(route.consumerQuery ?? readConsumerQuery()), ...changes, ...(reset ? {offset:0} : {})};
    if (!router.navigate(queueConsumersURL(queue,query))) void refresh.refresh();
  }
  const text=queueConsumersLabels(language);
  const unknown = text.unknown;
  const errors = {denied: text.read_denied_check_credentials_and_role, changed: text.declaration_changed_during_observation_refresh_explicitly, missing: text.queue_declaration_not_found, query: text.query_rejected_search_limit_is_256_normalized, invalid: text.invalid_collection_response_no_partial_results_shown, unavailable: text.observation_unavailable_not_an_empty_collection};
  const page = state.page;
  const names = {present: text.present, missing: text.missing, matching: text.matching, different: text.different_owner, unmarked: text.unmarked, unknown};
  return <section aria-labelledby="consumers-heading" className="queue-list">
    <h2 id="consumers-heading">{text.consumers}</h2>
    <p>{text.declared_and_observed_consumers_in_the_queue}</p>
    <p>{paused?text.tab_hidden_automatic_refresh_paused:refreshSeconds===0?text.automatic_refresh_disabled_manual_refresh_only:consumerRefreshLabel(language,refreshSeconds)}</p>
    <form className="list-controls filter-toolbar" onSubmit={event => { event.preventDefault(); changeQuery({q: search, offset: 0}); }}>
      <label htmlFor="consumer-search">{text.consumer_name_or_filter_subject}</label>
      <input id="consumer-search" value={search} onChange={event => setSearch(event.target.value)} />
      <button type="submit">{text.filter_consumers}</button>
      <label htmlFor="consumer-mode">{text.delivery_mode}</label>
      <select id="consumer-mode" value={state.query.mode} onChange={event => changeQuery({mode: event.target.value})}><option value="">{text.all_modes}</option><option value="pull">Pull</option><option value="push">Push</option></select>
      <label htmlFor="consumer-order">{text.consumer_order}</label>
      <select id="consumer-order" value={state.query.order} onChange={event => changeQuery({order: event.target.value})}><option value="asc">{text.name_ascending}</option><option value="desc">{text.name_descending}</option></select>
      <button type="button" disabled={paused||state.phase === "loading"} onClick={() => void refresh.refresh()}>{text.refresh_consumers}</button>
    </form>
    {state.phase === "loading" && <p role="status">{page?text.refreshing_retaining_last_consumer_page:text.loading_consumers}</p>}
    {state.failure && <p role="alert">{state.failure==="disabled"?text.resource_read_api_disabled_prior_data_cleared:errors[state.failure]}</p>}
    {page && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{text.rows_totals_and_declaration_revision_below_are}</p>}
      <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale_collection_observation_the_30_second_freshness}/>
      {(!declarationETag || page.declaration_revision !== declarationETag) && <p role="status">{text.this_collection_and_the_declaration_above_are}</p>}
      <dl><dt>Stream</dt><dd>{page.stream} · {names[page.stream_status]} · {names[page.stream_ownership]}</dd>
        <dt>{text.collection_declaration_etag}</dt><dd>{page.declaration_revision}</dd>
        <dt>{text.collection_read_completed}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd></dl>
      <p role="status">{text.filtered_total}: {page.total}</p>
      {page.items.length === 0 ? <p>{page.total > 0 ? text.this_page_is_empty_after_collection_changes : state.query.q || state.query.mode ? text.no_matching_consumers : text.no_consumers_in_this_collection}</p> :
        <div className="table-scroll" tabIndex="0" role="region" aria-label={text.consumer_collection_scroll_horizontally_for_all_columns}><table className="consumer-table">
          <caption className="visually-hidden">{text.queue_consumer_collection}</caption>
          <thead><tr>{["Consumer", text.declared, text.observed, text.ownership, text.mode, text.filter_subjects, text.pending, text.ack_pending].map(label => <th scope="col" key={label}>{label}</th>)}</tr></thead>
          <tbody>{page.items.map(row => {
            const subjects = row.observed ? (row.observed.filter_subjects?.length ? row.observed.filter_subjects : row.observed.filter_subject ? [row.observed.filter_subject] : []) : row.expected?.filterSubjects;
            return <tr key={`${row.stream}/${row.name}`}><th scope="row"><a href={consumerDetailURL(row.stream,row.name)} onClick={event => { if (event.button===0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {event.preventDefault();router.navigate(consumerDetailURL(row.stream,row.name));} }}>{row.name}</a></th><td>{row.expected ? text.yes : text.no_external}</td><td>{names[row.status]}</td><td>{names[row.ownership]}</td><td>{row.observed?.mode ?? row.expected?.mode ?? unknown}</td><td>{subjects?.length ? subjects.join(", ") : text.no_filter}</td><td>{consumerCounter(row,"pending") ?? unknown}</td><td>{consumerCounter(row,"ack_pending") ?? unknown}</td></tr>;
          })}</tbody>
        </table></div>}
      <nav className="pagination" aria-label={text.consumer_pages}>
        <button disabled={page.offset === 0} onClick={() => changeQuery({offset: page.items.length ? Math.max(0,page.offset-page.limit) : Math.max(0,Math.floor((page.total-1)/page.limit)*page.limit)})}>{text.previous_consumers}</button>
        <span>{page.items.length ? `${page.offset+1}–${page.offset+page.items.length}` : "0"} / {page.total}</span>
        <button disabled={page.offset+page.items.length >= page.total} onClick={() => changeQuery({offset: page.offset+page.limit})}>{text.next_consumers}</button>
      </nav>
    </>}
  </section>;
}
