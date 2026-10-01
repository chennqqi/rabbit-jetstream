import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createConnections,connectionCounters} from "./connections.mjs";
import {nodeConnectionsURL,nodeDetailURL,nodeConnectionURL} from "./routes.mjs";
import {connectionLabels} from "./connection-labels.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {completionRefreshLabel} from "./completion-refresh-label.mjs";
import {connectionsLabels} from "./connections-labels.mjs";
export function Connections({api,route,router,language,refreshSeconds=10}){
  const zh=language==="zh",text=connectionsLabels(language),{id,query}=route,accessible=connectionLabels(language);
  const view=useMemo(()=>createConnections(api,id,query),[api,id,query.offset,query.limit,query.cid]);
  const state=useSyncExternalStore(view.subscribe,view.snapshot),{page}=state;
  const [cid,setCID]=useState(query.cid??""),[cidError,setCIDError]=useState(false),[identityKind,setIdentityKind]=useState("user"),[identityValue,setIdentityValue]=useState(""),[identityError,setIdentityError]=useState(false);
  const {paused:hidden,clock:now}=useRefreshLoop({loop:view.refresh,seconds:refreshSeconds,cleanup:()=>view.clear()});
  useEffect(()=>{setCID(query.cid??"");setCIDError(false);},[query.cid]);
  const navigate=(event,url)=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(url);}};
  const change=changes=>{const next={...query,...changes};if(next.cid===undefined)delete next.cid;router.navigate(nodeConnectionsURL(id,next));};
  const searchCID=event=>{event.preventDefault();const value=cid.trim();let valid=value===""||/^[0-9]+$/.test(value);if(valid&&value!==""){const parsed=BigInt(value);valid=parsed>=1n&&parsed<=18446744073709551615n;}if(!valid){setCIDError(true);return;}setCIDError(false);change({offset:0,cid:value||undefined});};
  const searchIdentity=event=>{event.preventDefault();const value=identityValue;if(!value||value.length>256||/[\u0000-\u001f\u007f]/.test(value)){setIdentityError(true);return;}setIdentityError(false);void view.search(identityKind,value,0,query.limit);};
  const errors={missing:text.node_not_found_in_complete_configured_endpoint,ambiguous:text.multiple_endpoints_report_this_node_id_no,denied:text.connection_read_denied,disabled:text.resource_reads_disabled,query:text.connection_query_rejected,limit:text.connection_search_exceeds_the_1_000_connection,invalid:text.invalid_connection_response_old_data_cleared,unavailable:text.connection_observation_unavailable_this_is_not_an};
  return <section className="node-connections" aria-label={accessible.view}><a href={nodeDetailURL(id)} onClick={e=>navigate(e,nodeDetailURL(id))}>{text.back_to_node}</a><h2>{text.node_connections}: {id}</h2>
    <p>{text.open_connections_on_this_exact_node_sorted}</p>
    <p>{hidden?text.automatic_refresh_paused_while_hidden:refreshSeconds===0?text.automatic_refresh_disabled_manual_only:completionRefreshLabel(language,refreshSeconds)}</p>
    <form className="connection-controls" onSubmit={searchCID}><label htmlFor="connection-cid-search">{text.exact_cid}</label><input id="connection-cid-search" inputMode="numeric" pattern="[0-9]+" maxLength="20" aria-invalid={cidError||undefined} aria-describedby={cidError?"connection-cid-error":undefined} value={cid} onChange={event=>{setCID(event.target.value);setCIDError(false);}}/><button>{text.search}</button><button type="button" disabled={hidden||state.phase==="loading"} onClick={()=>void view.refresh.refresh()}>{text.refresh_connections}</button>
    <div><label htmlFor="connection-page-size">{text.connections_per_page}</label><select id="connection-page-size" value={query.limit} onChange={e=>change({limit:Number(e.target.value),offset:0})}>{[...new Set([25,50,100,200,query.limit])].sort((a,b)=>a-b).map(n=><option key={n}>{n}</option>)}</select></div></form>
    {cidError&&<p id="connection-cid-error" role="alert">{text.cid_must_be_a_decimal_integer_from}</p>}
    <form className="connection-controls" onSubmit={searchIdentity}>
      <label htmlFor="connection-identity-kind">{text.exact_identity_field}</label><select id="connection-identity-kind" value={identityKind} onChange={e=>setIdentityKind(e.target.value)}><option value="name">{text.client_name}</option><option value="user">{text.authorized_user}</option><option value="account">{text.account}</option><option value="mqtt_client">{text.mqtt_client_id}</option></select>
      <label htmlFor="connection-identity-value">{text.exact_value}</label><input id="connection-identity-value" type="password" autoComplete="off" maxLength="256" value={identityValue} aria-invalid={identityError||undefined} aria-describedby={identityError?"connection-identity-error":"connection-identity-note"} onChange={e=>{setIdentityValue(e.target.value);setIdentityError(false);}}/><button>{text.search_identity}</button>{state.identity&&<button type="button" onClick={()=>void view.clearSearch()}>{text.clear_identity_search}</button>}
    </form>
    <p id="connection-identity-note">{text.the_exact_value_stays_in_this_page}</p>
    {identityError&&<p id="connection-identity-error" role="alert">{text.enter_1_256_characters_without_control_characters}</p>}
    {state.phase==="loading"&&<p role="status">{text.reading_connections}</p>}{state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {page&&<>{(state.phase==="loading"||state.failure)&&<p role="status">{text.historical_page_from_the_last_successful_read}</p>}
      <StaleEvidence readAt={state.readAt} paused={hidden} failure={state.failure} clock={now} note={text.stale_observation_freshness_is_not_health}/>
      <p>{text.reported_node_total}: {page.total} · {text.returned_rows}: {page.items.length}</p>
      <p>{text.server_observation}: <time dateTime={page.observed_at} title={page.observed_at}>{page.observed_at}</time> · {text.management_read}: <time dateTime={page.read_at} title={page.read_at}>{page.read_at}</time></p>
      <p>{text.pages_are_live_independent_reads_counters_are}</p>
      {page.items.length?<div className="table-scroll connection-rows" role="region" tabIndex="0" aria-label={accessible.rows}><table><caption className="visually-hidden">{text.connection_rows}</caption><thead><tr>{["CID",text.pending_bytes,text.received_messages,text.sent_messages,text.received_bytes,text.sent_bytes,text.subscriptions].map(v=><th key={v}>{v}</th>)}</tr></thead><tbody>{page.items.map(row=><tr key={String(row.cid)}><th scope="row"><a href={nodeConnectionURL(id,row.cid,query)} onClick={event=>navigate(event,nodeConnectionURL(id,row.cid,query))}>{String(row.cid)}</a></th>{connectionCounters.map(key=><td key={key}>{row[key]===undefined?text.unknown:String(row[key])}</td>)}</tr>)}</tbody></table></div>:<p>{page.total===0?text.no_open_connections_reported:text.this_page_is_empty_return_to_an}</p>}
      {state.identity?<nav className="connection-pagination" aria-label={accessible.pagination}><button disabled={page.offset===0} onClick={()=>void view.searchPage(Math.max(0,page.offset-page.limit))}>{text.previous}</button><span>{page.items.length?`${page.offset+1}–${page.offset+page.items.length}`:"0"} / {page.total}</span><button disabled={page.items.length===0||page.offset+page.limit>=page.total} onClick={()=>void view.searchPage(page.offset+page.limit)}>{text.next}</button></nav>:!query.cid&&<nav className="connection-pagination" aria-label={accessible.pagination}><button disabled={query.offset===0} onClick={()=>change({offset:Math.max(0,Math.min(query.offset-query.limit,Math.floor(Math.max(0,page.total-1)/query.limit)*query.limit))})}>{text.previous}</button><span>{page.items.length?`${query.offset+1}–${query.offset+page.items.length}`:"0"} / {page.total}</span><button disabled={page.items.length===0||query.offset+query.limit>=page.total||query.offset+query.limit>1000000} onClick={()=>change({offset:query.offset+query.limit})}>{text.next}</button></nav>}
    </>}
  </section>;
}
