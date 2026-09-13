import React, {useMemo,useSyncExternalStore} from "react";
import {createConsumerRefresh} from "./consumer-refresh.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {ConsumerDiagnosis} from "./ConsumerDiagnosis.jsx";
import {createNodeRefresh} from "./node-refresh.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {streamDetailURL} from "./routes.mjs";
import {consumerDetailFormatters} from "./consumer-detail-formatters.mjs";
import {consumerDetailLabels} from "./consumer-detail-labels.mjs";

export function ConsumerDetail({api,stream,name,language,router,refreshSeconds=10}) {
  const format=consumerDetailFormatters(language);
  const view=useMemo(()=>createConsumerRefresh(api,stream,name),[api,stream,name]);
  const {model,refresh}=view;
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  const nodeView=useMemo(()=>createNodeRefresh(api),[api]);
  const nodeState=useSyncExternalStore(nodeView.model.subscribe,nodeView.model.snapshot);
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  useRefreshLoop({loop:nodeView.refresh,seconds:refreshSeconds,cleanup:()=>nodeView.clear()});
  const text=consumerDetailLabels(language), unknown=text.unknown;
  const errors={missing:text.consumer_not_found_at_this_observation_it,denied:text.read_denied_check_credentials_and_permissions,invalid:text.invalid_consumer_response_no_partial_data_shown,unavailable:text.consumer_observation_unavailable_not_a_missing_or};
  const resource=state.resource;
  return <section className="queue-list" aria-labelledby="consumer-heading">
    <ResourceBreadcrumb trail={[{label:text.stream_list,href:"/admin/streams"},{label:stream,href:streamDetailURL(stream)},{label:name}]} router={router} language={language}/>
    <h2 id="consumer-heading">Consumer: {name}</h2><p>Stream: {stream}</p>
    <p>{text.exact_observed_consumer_independent_of_list_pagination}</p>
    <p>{paused?text.tab_hidden_automatic_refresh_paused:refreshSeconds===0?text.automatic_refresh_disabled_manual_refresh_only:format.refresh(refreshSeconds)}</p>
    <button disabled={paused||state.phase==="loading"||nodeState.phase==="loading"} onClick={()=>void Promise.all([refresh.refresh(),nodeView.refresh.refresh()])}>{text.refresh_consumer}</button>
    {state.phase==="loading" && <p role="status">{resource?text.refreshing_retaining_last_consumer_observation:text.loading_consumer}</p>}
    {state.failure && <p role="alert">{state.failure==="disabled"?text.resource_read_api_disabled_prior_data_cleared:errors[state.failure]}</p>}
    {resource && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{text.configuration_and_counters_below_are_from_the}</p>}
      <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale_observation_paused_refresh_failed_or_older}/>
      <dl><dt>{text.read_completed}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd>
        <dt>{text.mode}</dt><dd>{resource.mode}</dd>
        <dt>Durable</dt><dd>{resource.durable || text.not_configured}</dd>
        <dt>{text.filter_subjects}</dt><dd>{(resource.filter_subjects?.length ? resource.filter_subjects : resource.filter_subject ? [resource.filter_subject] : []).join(", ") || text.no_filter}</dd>
        {["pending","ack_pending","redelivered","waiting"].map(key=><React.Fragment key={key}><dt>{format.counters[key]}</dt><dd>{consumerCounter({observed:resource},key)??unknown}</dd></React.Fragment>)}
        <dt>{text.ack_policy}</dt><dd>{resource.ack_policy??unknown}</dd>
        <dt>{text.ack_wait_nanoseconds}</dt><dd>{consumerCounter({observed:resource},"ack_wait_nanos")??unknown}</dd>
      </dl>
      <details><summary>{text.raw_observation_exact_integers}</summary><pre className="declaration-json">{stringifyJSON(resource)}</pre></details>
      <ConsumerDiagnosis state={state} nodeState={nodeState} language={language} now={clock} paused={paused}/>
    </>}
  </section>;
}
