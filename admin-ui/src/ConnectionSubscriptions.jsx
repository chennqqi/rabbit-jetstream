import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createConnectionSubscriptions,subscriptionPage} from "./connection-subscriptions.mjs";
import {connectionSubscriptionsLabels} from "./connection-subscriptions-labels.mjs";

export function ConnectionSubscriptions({api,id,cid,language}){
  const text=connectionSubscriptionsLabels(language),model=useMemo(()=>createConnectionSubscriptions(api,id,cid),[api,id,cid]),state=useSyncExternalStore(model.subscribe,model.snapshot),[query,setQuery]=useState(""),[offset,setOffset]=useState(0),limit=50;
  useEffect(()=>()=>model.clear(),[model]);
  let page=null;try{if(state.observation)page=subscriptionPage(state.observation,query,offset,limit);}catch{}
  useEffect(()=>{if(page&&offset>=page.total&&offset!==0)setOffset(Math.max(0,Math.floor(Math.max(0,page.total-1)/limit)*limit));},[page?.total,offset]);
  const errors={denied:text.subscription_read_denied,disabled:text.resource_reads_disabled,limit:text.more_than_1_000_subscriptions_were_reported,missing:text.the_open_connection_disappeared_before_subscription_observation,node_missing:text.node_not_found_in_complete_configured_endpoint,ambiguous:text.node_identity_is_ambiguous,query:text.subscription_request_rejected,invalid:text.invalid_subscription_response_data_cleared,unavailable:text.subscription_observation_unavailable_no_empty_result_is};
  return <section className="connection-subscriptions" aria-label={text.connection_subscriptions}>
    <h3>{text.subscriptions}</h3>
    <p>{text.load_explicitly_subjects_queue_groups_and_exact}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{state.observation?text.refresh_subscriptions:text.load_subscriptions}</button>
    {state.phase==="loading"&&<p role="status">{text.reading_a_bounded_subscription_observation}</p>}{state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {state.observation&&page&&<><p>{text.observed}: <time dateTime={state.observation.observed_at}>{state.observation.observed_at}</time> · {text.management_read}: <time dateTime={state.observation.read_at}>{state.observation.read_at}</time></p>
      {state.failure==="unavailable"&&<p role="status">{text.showing_the_last_successful_historical_observation}</p>}
      <label>{text.filter_this_complete_observation}<input value={query} maxLength="256" onChange={event=>{setQuery(event.target.value);setOffset(0);}} /></label>
      {page.items.length?<div className="table-scroll" role="region" tabIndex="0" aria-label={text.subscription_rows}><table><thead><tr><th>SID</th><th>Subject</th><th>{text.queue_group}</th><th>{text.delivered_messages}</th><th>{text.auto_unsubscribe_maximum}</th></tr></thead><tbody>{page.items.map(row=><tr key={row.sid}><th scope="row">{row.sid}</th><td>{row.subject}</td><td>{row.queue??text.none}</td><td>{String(row.messages)}</td><td>{row.maximum===undefined?text.unreported:String(row.maximum)}</td></tr>)}</tbody></table></div>:<p>{page.total===0&&query?text.no_subscriptions_match_this_local_filter : text.no_subscriptions_were_reported_in_this_observation}</p>}
      <nav aria-label={text.subscription_pages}><button disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-limit))}>{text.previous}</button><span>{page.items.length?`${offset+1}–${offset+page.items.length}`:"0"} / {page.total}</span><button disabled={offset+limit>=page.total} onClick={()=>setOffset(offset+limit)}>{text.next}</button></nav></>}
  </section>;
}
