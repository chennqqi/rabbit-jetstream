import React,{useMemo,useSyncExternalStore} from "react";
import {createConnectionDetail} from "./connection-detail.mjs";
import {nodeConnectionsURL} from "./routes.mjs";
import {connectionLabels} from "./connection-labels.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {ConnectionSubscriptions} from "./ConnectionSubscriptions.jsx";
import {completionRefreshLabel} from "./completion-refresh-label.mjs";
import {connectionDetailFormatters} from "./connection-detail-formatters.mjs";
import {connectionDetailLabels} from "./connection-detail-labels.mjs";

export function ConnectionDetail({api,route,router,language,refreshSeconds=10}){
  const format=connectionDetailFormatters(language);
  const text=connectionDetailLabels(language),{id,cid,query}=route,accessible=connectionLabels(language);
  const view=useMemo(()=>createConnectionDetail(api,id,cid),[api,id,cid]),state=useSyncExternalStore(view.subscribe,view.snapshot),{detail}=state;
  const back=nodeConnectionsURL(id,query);
  const {paused:hidden,clock:now}=useRefreshLoop({loop:view.refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  const errors={missing:text.this_open_connection_was_not_found_on,node_missing:text.node_not_found_in_complete_configured_endpoint,ambiguous:text.node_identity_is_ambiguous_connection_evidence_cleared,denied:text.connection_read_denied,disabled:text.resource_reads_disabled,query:text.connection_query_rejected,invalid:text.invalid_connection_response_old_data_cleared,unavailable:text.connection_observation_unavailable_absence_is_not_established};
  return <section className="connection-detail" aria-label={accessible.detail}>
    <ResourceBreadcrumb trail={[{label:text.nodes,href:"/admin/nodes"},{label:id,href:`/admin/nodes/${encodeURIComponent(id)}`},{label:format.connection(cid),href:back}]} router={router} language={language}/>
    <a href={back} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(back);}}}>{text.back_to_connections}</a>
    <h2>{text.connection}: {cid}</h2><p>{text.node}: {id}</p>
    <p>{text.identity_is_node_id_plus_cid_this}</p>
    <p>{hidden?text.automatic_refresh_paused_while_hidden:refreshSeconds===0?text.automatic_refresh_disabled_manual_only:completionRefreshLabel(language,refreshSeconds)}</p>
    <button disabled={hidden||state.phase==="loading"} onClick={()=>void view.refresh.refresh()}>{text.refresh_connection}</button>
    {state.phase==="loading"&&<p role="status">{text.reading_connection}</p>}{state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {detail&&<>{(state.phase==="loading"||state.failure)&&<p role="status">{text.historical_connection_evidence_from_the_last_successful}</p>}
      <StaleEvidence readAt={state.readAt} paused={hidden} failure={state.failure} clock={now} note={text.stale_observation_freshness_is_not_health}/>
      <p>{text.server_observation}: <time dateTime={detail.observed_at}>{detail.observed_at}</time> · {text.management_read}: <time dateTime={detail.read_at}>{detail.read_at}</time></p>
      <p>{text.message_and_byte_counters_are_cumulative_for}</p>
      <dl>{["pending_bytes","in_msgs","out_msgs","in_bytes","out_bytes","subscriptions"].map(key=><React.Fragment key={key}><dt>{format.counters[key]}</dt><dd>{detail.item[key]===undefined?text.unknown:String(detail.item[key])}</dd></React.Fragment>)}</dl>
      <ConnectionSubscriptions api={api} id={id} cid={cid} language={language}/>
    </>}
  </section>;
}
