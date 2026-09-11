import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {createListRefresh} from "./list-refresh.mjs";
import {queueDeployment,queueObservation} from "./queue-list.mjs";
import {queueListURL, queueDetailURL, streamListURL, streamDetailURL} from "./routes.mjs";

const copy = {
  en: {title: "Queues", search: "Queue name contains", apply: "Search", refresh: "Refresh", asc: "Name ascending", desc: "Name descending", sort: "Sort", size: "Page size", previous: "Previous", next: "Next", loading: "Loading Queues…", empty: "No matching Queues.", total: "Filtered total", name: "Queue", revision: "Plan revision", read: "Read completed", note: "Each page joins one declaration enumeration with one Stream enumeration. Counts and Stream consistency are observations, not Consumer-configuration, convergence or health proof.", errors: {"credentials-rejected": "Credentials rejected. Clear the session and verify again; expiry is not assumed.", "role-denied": "Your role cannot read these resources.", "invalid-query": "The server rejected this query. Search is limited to 256 normalized UTF-8 bytes.", "auth-disabled": "Resource authentication is not configured.", "invalid-response": "Invalid Queue page response; no partial results are shown.", unavailable: "Queue data is unavailable. Retry explicitly; this is not an empty result."}},
  zh: {title: "队列", search: "Queue 名称包含", apply: "搜索", refresh: "刷新", asc: "名称升序", desc: "名称降序", sort: "排序", size: "每页条数", previous: "上一页", next: "下一页", loading: "正在加载 Queue…", empty: "没有匹配的 Queue。", total: "筛选后总数", name: "Queue", revision: "Plan 版本", read: "读取完成时间", note: "每页由一次声明枚举和一次 Stream 枚举连接。计数及 Stream 一致性属于观测，不证明 Consumer 配置、收敛或健康。", errors: {"credentials-rejected": "凭据被拒绝，请清除会话并重新验证，不能直接认定为过期。", "role-denied": "当前角色无权读取这些资源。", "invalid-query": "服务端拒绝了查询。搜索上限为规范化后的 256 个 UTF-8 字节。", "auth-disabled": "资源认证尚未配置。", "invalid-response": "Queue 分页响应无效，不展示部分结果。", unavailable: "Queue 数据不可用，请手动重试。这不代表空结果。"}},
};

export function QueueList({api, language, route, router, resource = "queues", refreshSeconds = 10}) {
  const streams = resource === "streams";
  const listURL = streams ? streamListURL : queueListURL;
  const detailURL = streams ? streamDetailURL : queueDetailURL;
  const list = useMemo(() => createListRefresh(api, resource, route.query), [api, resource, route.query.q, route.query.order, route.query.offset, route.query.limit]);
  const {model,refresh}=list;
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const [search, setSearch] = useState(route.query.q);
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0);
  const clock=Date.now();
  const text = streams ? {...copy[language],
    title: "Streams", name: "Stream", revision: language === "zh" ? "存储消息数" : "Stored messages",
    search: language === "zh" ? "Stream 名称包含" : "Stream name contains",
    loading: language === "zh" ? "正在加载 Stream…" : "Loading Streams…",
    empty: language === "zh" ? "没有匹配的 Stream。" : "No matching Streams.",
    note: language === "zh" ? "展示观测到的 Stream（包括外部创建的资源）。存储消息数不是某个 Consumer 的待投递数，也不代表健康状态。" : "Observed Streams, including externally created resources. Stored messages are not a Consumer pending count or a health assessment.",
    errors: {...copy[language].errors, "invalid-response": language === "zh" ? "Stream 分页响应无效。" : "Invalid Stream page response.", unavailable: language === "zh" ? "Stream 数据不可用；不代表空结果。" : "Stream data unavailable; this is not an empty result."},
  } : copy[language];
  useEffect(()=>setSearch(route.query.q),[route.query.q]);
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{list.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[list,refresh]);
  function changeQuery(changes) {
    const reset = ["q", "order", "limit"].some(key => Object.hasOwn(changes, key));
    const query = {...route.query, ...changes, ...(reset ? {offset: 0} : {})};
    if (!router.navigate(listURL(query))) void refresh.refresh();
  }
  const busy = state.phase === "loading";
  const declaredStorage=language==="zh"?"声明存储":"Declared storage";
  const declaredReplicas=language==="zh"?"请求副本数":"Requested replicas";
  const observedState=language==="zh"?"观测状态":"Observed state",storedMessages=language==="zh"?"存储消息数":"Stored messages",consumerCount=language==="zh"?"Consumer 数":"Consumers";
  const unknown=language==="zh"?"未知":"Unknown";
  const observationLabel=value=>value===null?unknown:{present:language==="zh"?"Stream 一致；未检查 Consumer 配置":"Stream consistent; Consumer configuration not checked",missing:language==="zh"?"Stream 缺失":"Stream missing",degraded:language==="zh"?"Stream 降级或配置不一致":"Stream degraded or configuration mismatch",unavailable:language==="zh"?"观测不可用":"Observation unavailable"}[value.state];
  return <section aria-labelledby="queue-list-heading" className="queue-list">
    <h2 id="queue-list-heading">{text.title}</h2><p>{text.note}</p>
    <p>{paused?(language==="zh"?"标签页隐藏，自动刷新已暂停。":"Tab hidden; automatic refresh paused."):refreshSeconds===0?(language==="zh"?"自动刷新已关闭，仅手动刷新。":"Automatic refresh disabled; manual refresh only."):(language==="zh"?`自动刷新：每次读取完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。`:`Automatic refresh: ${refreshSeconds} seconds after each read; failure backoff up to 60 seconds.`)}</p>
    <form className="list-controls" onSubmit={event => { event.preventDefault(); changeQuery({q: search, offset: 0}); }}>
      <label htmlFor="queue-search">{text.search}</label>
      <input id="queue-search" value={search} onChange={event => setSearch(event.target.value)} />
      <button type="submit">{text.apply}</button>
      <label htmlFor="queue-sort">{text.sort}</label>
      <select id="queue-sort" value={state.query.order} onChange={event => changeQuery({order: event.target.value})}><option value="asc">{text.asc}</option><option value="desc">{text.desc}</option></select>
      <label htmlFor="queue-size">{text.size}</label>
      <select id="queue-size" value={state.query.limit} onChange={event => changeQuery({limit: Number(event.target.value)})}>{[...new Set([25, 50, 100, 200, route.query.limit])].sort((a,b) => a-b).map(size => <option key={size}>{size}</option>)}</select>
      <button type="button" onClick={() => void refresh.refresh()} disabled={busy||paused}>{text.refresh}</button>
    </form>
    {busy && <p role="status">{state.page?(language==="zh"?"正在刷新，保留上次成功结果…":"Refreshing; retaining last successful results…"):text.loading}</p>}
    {state.failure && <p role="alert">{text.errors[state.failure.kind]}</p>}
    {state.page && <>
      {(busy||state.failure)&&<p role="status">{language==="zh"?"以下行、总数及分页来自此查询上次成功读取，不代表当前结果。详情链接将重新读取资源。":"Rows, totals and pagination below are from the last successful read of this query, not current results. Detail links read the resource again."}</p>}
      <p role="status">{text.total}: {state.page.total} · {text.read}: <time dateTime={state.readAt}>{state.readAt}</time></p>
      {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{language==="zh"?"旧观测数据：已暂停、刷新失败或超过 30 秒；此阈值不表示 Broker 健康状态。":"Stale observation: paused, refresh failed or older than 30 seconds; this threshold is not broker health evidence."}</p>}
      {state.page.items.length === 0 ? <p>{state.page.total === 0 ? text.empty : language === "zh" ? "当前页已无数据，请返回上一页。" : "This page is now empty. Return to the previous page."}</p> : <div className="table-scroll" tabIndex="0" role="region" aria-label={text.title}><table>
        <thead><tr><th scope="col">{text.name}</th>{!streams&&<><th scope="col">{observedState}</th><th scope="col">{storedMessages}</th><th scope="col">{consumerCount}</th><th scope="col">{declaredStorage}</th><th scope="col">{declaredReplicas}</th></>}<th scope="col">{text.revision}</th></tr></thead>
        <tbody>{state.page.items.map(item => {const name = streams ? item.name : item.queue,deployment=streams?null:queueDeployment(item),observation=streams?null:queueObservation(item); return <tr key={name}><th scope="row"><a href={detailURL(name)} onClick={event => { if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) { event.preventDefault(); router.navigate(detailURL(name)); } }}>{name}</a></th>{!streams&&<><td>{observationLabel(observation)}</td><td>{observation?.messages===undefined?unknown:String(observation.messages)}</td><td>{observation?.consumers===undefined?unknown:String(observation.consumers)}</td><td>{deployment?.storage??unknown}</td><td>{deployment?.replicas??unknown}</td></>}<td>{streams ? ((typeof item.messages === "bigint" && item.messages >= 0n || Number.isSafeInteger(item.messages) && item.messages >= 0) ? String(item.messages) : unknown) : item.revision}</td></tr>;})}</tbody>
      </table></div>}
      <nav aria-label={text.title} className="pagination">
        <button disabled={state.page.offset === 0} onClick={() => changeQuery({offset: state.page.items.length === 0 ? Math.max(0, Math.floor((state.page.total - 1) / state.page.limit) * state.page.limit) : Math.max(0, state.page.offset - state.page.limit)})}>{text.previous}</button>
        <span>{state.page.items.length ? `${state.page.offset + 1}–${state.page.offset + state.page.items.length}` : "0"} / {state.page.total}</span>
        <button disabled={state.page.offset + state.page.items.length >= state.page.total} onClick={() => changeQuery({offset: state.page.offset + state.page.limit})}>{text.next}</button>
      </nav>
    </>}
  </section>;
}
