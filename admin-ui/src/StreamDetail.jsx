import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {createStreamRefresh} from "./stream-refresh.mjs";
import {createConsumerCollectionRefresh} from "./consumer-collection-refresh.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {consumerDetailURL, streamDetailURL} from "./routes.mjs";
import {stringifyJSON} from "./api.mjs";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";
import {streamLabels} from "./stream-labels.mjs";

export function StreamDetail({api, name, language, route, router, refreshSeconds=10}) {
  const zh = language === "zh", unknown = zh ? "未知" : "Unknown", accessible=streamLabels(language);
  const observation = useMemo(() => createStreamRefresh(api, name), [api, name]);
  const {model,refresh:stateRefresh}=observation;
  const query=route.query;
  const collection=useMemo(()=>createConsumerCollectionRefresh(api,"stream",name,query),[api,name,query.q,query.mode,query.order,query.offset,query.limit]);
  const {model:list,refresh}=collection;
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const consumers = useSyncExternalStore(list.subscribe, list.snapshot);
  const [search, setSearch] = useState(route.query.q);
  useEffect(()=>{stateRefresh.setInterval(refreshSeconds*1000);},[stateRefresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>stateRefresh.visibility(document.hidden);
    document.addEventListener("visibilitychange",visibility);stateRefresh.start(document.hidden);
    return()=>{observation.clear();document.removeEventListener("visibilitychange",visibility);};
  },[observation,stateRefresh]);
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0),clock=Date.now();
  useEffect(()=>setSearch(query.q),[query.q]);
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{collection.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[collection,refresh]);
  const resource = state.resource, page = consumers.resource;
  const failure = value => ({disabled: zh?"资源读取接口已禁用，已清除此前数据。":"Resource read API disabled; prior data cleared.","invalid-query": zh ? "查询被拒绝。搜索上限为规范化后的 256 个 UTF-8 字节。" : "Query rejected. Search is limited to 256 normalized UTF-8 bytes.", missing: zh ? "资源不存在。" : "Resource not found.", denied: zh ? "凭据被拒绝或无读取权限。" : "Credentials rejected or read access denied.", invalid: zh ? "响应无效，不展示部分数据。" : "Invalid response; no partial data shown.", unavailable: zh ? "数据不可用，不代表空结果。" : "Data unavailable; this is not an empty result."}[value]);
  const counter = (row, field) => consumerCounter({observed: row}, field) ?? unknown;
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
    <h2>Stream: {name}</h2>
    <button disabled={paused||state.phase === "loading"} onClick={() => void stateRefresh.refresh()}>{zh ? "刷新 Stream" : "Refresh Stream"}</button>
    <p>{paused?(zh?"标签页隐藏，Stream 自动刷新已暂停。":"Tab hidden; automatic Stream refresh paused."):refreshSeconds===0?(zh?"Stream 自动刷新已关闭，仅手动刷新。":"Automatic Stream refresh disabled; manual refresh only."):(zh?`Stream 自动刷新：每次读取完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。与 Consumer 集合独立调度。`:`Automatic Stream refresh: ${refreshSeconds} seconds after each read; failure backoff up to 60 seconds. Scheduled independently from the Consumer collection.`)}</p>
    {state.phase === "loading" && <p role="status">{zh ? "正在读取 Stream…" : "Loading Stream…"}</p>}
    {state.failure && <p role="alert">{failure(state.failure)}</p>}
    {resource && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{zh?"以下 Stream 配置、指标、副本及完整响应来自上次成功读取，不代表当前观测。":"Stream configuration, metrics, replicas and full response below are from the last successful read, not current observations."}</p>}
      {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{zh?"旧 Stream 观测；30 秒新鲜度阈值不是健康证明。":"Stale Stream observation; the 30-second freshness threshold is not health evidence."}</p>}
      <h3>{zh ? "观测配置" : "Observed configuration"}</h3>
      <dl>{[[accessible.subjects, resource.subjects?.join(", ") ?? unknown], [zh ? "存储类型" : "Storage", resource.storage ?? unknown], [zh ? "配置副本数" : "Configured replicas", resource.replicas ?? unknown], [accessible.retention, resource.retention ?? unknown], [accessible.discard, resource.discard ?? unknown]].map(([label, value]) => <React.Fragment key={label}><dt>{label}</dt><dd>{String(value)}</dd></React.Fragment>)}</dl>
      <h3>{zh ? "观测状态" : "Observed state"}</h3>
      <p>{zh ? "配置副本数不代表在线副本数；存储消息数不是 Consumer 待投递数。不同读取不是同一时刻的原子快照。" : "Configured replicas are not an online replica count. Stored messages are not Consumer pending messages. Separate reads are not an atomic snapshot."}</p>
      <dl>{[[zh ? "存储消息数" : "Stored messages", "messages"], [accessible.bytes, "bytes"], [accessible.consumerCount, "consumers"], [accessible.firstSequence, "first_sequence"], [accessible.lastSequence, "last_sequence"]].map(([label, field]) => <React.Fragment key={field}><dt>{label}</dt><dd>{counter(resource, field)}</dd></React.Fragment>)}</dl>
      <time dateTime={state.readAt}>{state.readAt}</time>
      <ReplicaEvidence stream={resource} language={language}/>
      <details><summary>{zh ? "完整 Stream 响应" : "Full Stream response"}</summary><pre className="declaration-json">{stringifyJSON(resource)}</pre></details>
    </>}
    <h3>{zh ? "观测到的 Consumer 列表" : "Observed Consumers"}</h3>
    <p>{paused?(zh?"标签页隐藏，集合自动刷新已暂停。":"Tab hidden; automatic collection refresh paused."):refreshSeconds===0?(zh?"集合自动刷新已关闭，仅手动刷新。":"Automatic collection refresh disabled; manual refresh only."):(zh?`集合自动刷新：每次读取完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。`:`Automatic collection refresh: ${refreshSeconds} seconds after each read; failure backoff up to 60 seconds.`)}</p>
    <p>{zh ? "展示实际存在的 Consumer，包括外部创建的资源；不含缺失的声明项。搜索、模式筛选和排序均在服务端分页前执行。" : "Actual Consumers, including externally created resources, not missing declarations. Search, mode filtering and sorting run on the server before pagination."}</p>
    <form className="list-controls" onSubmit={event => {event.preventDefault(); changeQuery({q: search});}}>
      <label htmlFor="stream-consumer-search">{zh ? "Consumer 名称或过滤 Subject" : "Stream Consumer name or filter Subject"}</label>
      <input id="stream-consumer-search" value={search} onChange={event => setSearch(event.target.value)} />
      <button type="submit">{zh ? "筛选 Stream Consumers" : "Filter Stream Consumers"}</button>
      <label htmlFor="stream-consumer-mode">{zh ? "Consumer 模式" : "Stream Consumer mode"}</label>
      <select id="stream-consumer-mode" value={route.query.mode} onChange={event => changeQuery({mode: event.target.value})}><option value="">{zh ? "全部模式" : "All modes"}</option><option value="pull">Pull</option><option value="push">Push</option></select>
      <label htmlFor="stream-consumer-order">{zh ? "Consumer 排序" : "Stream Consumer sort"}</label>
      <select id="stream-consumer-order" value={route.query.order} onChange={event => changeQuery({order: event.target.value})}><option value="asc">{zh ? "名称升序" : "Name ascending"}</option><option value="desc">{zh ? "名称降序" : "Name descending"}</option></select>
      <label htmlFor="stream-consumer-limit">{zh ? "Consumer 每页条数" : "Stream Consumer page size"}</label>
      <select id="stream-consumer-limit" value={route.query.limit} onChange={event => changeQuery({limit: Number(event.target.value)})}>{[...new Set([1, 25, 50, 100, 200, route.query.limit])].sort((a,b) => a-b).map(limit => <option key={limit}>{limit}</option>)}</select>
    </form>
    <button disabled={paused||consumers.phase === "loading"} onClick={() => void refresh.refresh()}>{zh ? "刷新 Stream Consumers" : "Refresh Stream Consumers"}</button>
    {consumers.phase === "loading" && <p role="status">{page?(zh?"正在刷新，保留上次 Consumer 页…":"Refreshing; retaining last Consumer page…"):(zh ? "正在读取 Consumer…" : "Loading Consumers…")}</p>}
    {consumers.failure && <p role="alert">{failure(consumers.failure)}</p>}
    {page && <>
      {(consumers.phase==="loading"||consumers.failure)&&<p role="status">{zh?"以下行和总数来自此查询上次成功读取，不代表当前观测。":"Rows and totals below are from the last successful read of this query, not current observations."}</p>}
      {consumers.readAt&&(paused||consumers.failure||clock-Date.parse(consumers.readAt)>30000||clock<Date.parse(consumers.readAt))&&<p role="status">{zh?"旧集合观测；30 秒新鲜度阈值不是健康证明。":"Stale collection observation; the 30-second freshness threshold is not health evidence."}</p>}
      <p>{zh ? "筛选后总数" : "Filtered total"}: {page.total} · <time dateTime={consumers.readAt}>{consumers.readAt}</time></p>
      {page.items.length ? <div className="table-scroll" tabIndex="0" role="region" aria-label={accessible.consumers}><table><thead><tr><th>Consumer</th><th>{accessible.mode}</th><th>{accessible.pending}</th><th>{accessible.ackPending}</th></tr></thead><tbody>{page.items.map(row => <tr key={row.name}><th scope="row"><a href={consumerDetailURL(name, row.name)} onClick={event => {if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {event.preventDefault(); router.navigate(consumerDetailURL(name, row.name));}}}>{row.name}</a></th><td>{row.mode}</td><td>{counter(row, "pending")}</td><td>{counter(row, "ack_pending")}</td></tr>)}</tbody></table></div> : <p>{page.total === 0 ? (zh ? "没有匹配的 Consumer。" : "No matching Consumers.") : (zh ? "当前页已无数据，请返回上一页。" : "This page is now empty. Return to the previous page.")}</p>}
      <nav aria-label={accessible.pagination}><button disabled={page.offset === 0} onClick={() => changePage(page.items.length ? Math.max(0, page.offset - page.limit) : Math.max(0, Math.floor((page.total - 1) / page.limit) * page.limit))}>{zh ? "上一页" : "Previous"}</button><span>{page.items.length ? `${page.offset + 1}–${page.offset + page.items.length}` : "0"} / {page.total}</span><button disabled={page.offset + page.items.length >= page.total} onClick={() => changePage(page.offset + page.limit)}>{zh ? "下一页" : "Next"}</button></nav>
    </>}
  </section>;
}
