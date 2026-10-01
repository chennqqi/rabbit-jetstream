import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createListRefresh} from "./list-refresh.mjs";
import {useRefreshLoop} from "./use-refresh-loop.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {queueDeployment,queueObservation} from "./queue-list.mjs";
import {queueListURL,queueDetailURL,streamListURL,streamDetailURL} from "./routes.mjs";
import {queueListLabels} from "./queue-list-labels.mjs";
import {ReadOnlyCollectionTable,OffsetPagination} from "./collection-table.jsx";
import {StatusBadge,observationTone} from "./status-badge.jsx";
import {PageNotes,RefreshStatus} from "./DisplayValues.jsx";
import {CopyValue} from "./copy-value.jsx";

export function QueueList({api,language,route,router,resource="queues",refreshSeconds=10,permissions=[]}){
  const streams=resource==="streams",text=queueListLabels(language,resource);
  const listURL=streams?streamListURL:queueListURL,detailURL=streams?streamDetailURL:queueDetailURL;
  const list=useMemo(()=>createListRefresh(api,resource,route.query),[api,resource,route.query.q,route.query.order,route.query.offset,route.query.limit]);
  const {model,refresh}=list,state=useSyncExternalStore(model.subscribe,model.snapshot);
  const [search,setSearch]=useState(route.query.q);
  const {paused,clock}=useRefreshLoop({loop:refresh,seconds:refreshSeconds,cleanup:()=>list.clear()});
  useEffect(()=>setSearch(route.query.q),[route.query.q]);
  function changeQuery(changes){
    const reset=["q","order","limit"].some(key=>Object.hasOwn(changes,key));
    const query={...route.query,...changes,...(reset?{offset:0}:{})};
    if(!router.navigate(listURL(query)))void refresh.refresh();
  }
  const busy=state.phase==="loading";
  const observationCell=value=>{
    if(value===null)return text.unknown;
    const label=text.observationsShort[value.state]??text.observations[value.state]??text.unknown;
    const footnote=value.state==="present"?text.observationFootnote:null;
    return <><StatusBadge tone={observationTone(value.state)}>{label}</StatusBadge>{footnote&&<small className="badge-footnote">{footnote}</small>}</>;
  };
  const columns=[{key:"name",label:text.name},...(!streams?[{key:"state",label:text.observedState},{key:"messages",label:text.storedMessages},{key:"consumers",label:text.consumerCount},{key:"storage",label:text.declaredStorage},{key:"replicas",label:text.declaredReplicas}]:[]),{key:"revision",label:text.revision}];
  return <section aria-labelledby="queue-list-heading" className="queue-list">
    <h2 id="queue-list-heading">{text.title}</h2>{!streams&&permissions.includes("queue:apply")&&<a className="primary-action list-create" href="/admin/queues/new" onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate("/admin/queues/new");}}}>{text.createQueue}</a>}
    <RefreshStatus readAt={state.readAt} paused={paused} seconds={refreshSeconds} language={language}/>
    <PageNotes language={language}><p>{text.note}</p></PageNotes>
    <form className="list-controls queue-list-controls" onSubmit={event=>{event.preventDefault();changeQuery({q:search,offset:0});}}>
      <label htmlFor="queue-search">{text.search}</label>
      <input id="queue-search" name={`${resource}-search`} type="search" autoComplete="off" value={search} onChange={event=>setSearch(event.target.value)}/>
      <button type="submit">{text.apply}</button>
      <label htmlFor="queue-sort">{text.sort}</label>
      <select id="queue-sort" value={state.query.order} onChange={event=>changeQuery({order:event.target.value})}><option value="asc">{text.asc}</option><option value="desc">{text.desc}</option></select>
      <label htmlFor="queue-size">{text.size}</label>
      <select id="queue-size" value={state.query.limit} onChange={event=>changeQuery({limit:Number(event.target.value)})}>{[...new Set([25,50,100,200,route.query.limit])].sort((a,b)=>a-b).map(size=><option key={size}>{size}</option>)}</select>
      <button type="button" onClick={()=>void refresh.refresh()} disabled={busy||paused}>{text.refresh}</button>
    </form>
    {busy&&<p role="status">{state.page?text.refreshing:text.loading}</p>}
    {state.failure&&<p role="alert">{text.errors[state.failure.kind]??text.errors.unavailable}</p>}
    {state.page&&<>
      {(busy||state.failure)&&<p role="status">{text.retained}</p>}
      <p role="status">{text.total}: {state.page.total} · {text.read}: <time dateTime={state.readAt}>{state.readAt}</time></p>
      <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale}/>
      {state.page.items.length===0?<p>{state.page.total===0?text.empty:text.emptyPage}</p>:<ReadOnlyCollectionTable label={text.title} caption={text.caption} columns={columns}>{state.page.items.map(item=>{
          const name=streams?item.name:item.queue,deployment=streams?null:queueDeployment(item),observation=streams?null:queueObservation(item);
          return <tr key={name}><th scope="row"><a href={detailURL(name)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(detailURL(name));}}}>{name}</a></th>{!streams&&<><td>{observationCell(observation)}</td><td>{observation?.messages===undefined?text.unknown:String(observation.messages)}</td><td>{observation?.consumers===undefined?text.unknown:String(observation.consumers)}</td><td>{deployment?.storage??text.unknown}</td><td>{deployment?.replicas??text.unknown}</td></>}<td>{streams?((typeof item.messages==="bigint"&&item.messages>=0n||Number.isSafeInteger(item.messages)&&item.messages>=0)?String(item.messages):text.unknown):(item.revision?<CopyValue value={item.revision} language={language} short/>:text.unknown)}</td></tr>;
        })}</ReadOnlyCollectionTable>}
      <OffsetPagination label={text.title} offset={state.page.offset} limit={state.page.limit} returned={state.page.items.length} total={state.page.total} previousLabel={text.previous} nextLabel={text.next} onPrevious={()=>changeQuery({offset:state.page.items.length===0?Math.max(0,Math.floor((state.page.total-1)/state.page.limit)*state.page.limit):Math.max(0,state.page.offset-state.page.limit)})} onNext={()=>changeQuery({offset:state.page.offset+state.page.limit})}/>
    </>}
  </section>;
}
