import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {consumerCounter} from "./queue-consumers.mjs";
import {createConsumerCollectionRefresh} from "./consumer-collection-refresh.mjs";
import {consumerDetailURL, queueConsumersURL, readConsumerQuery} from "./routes.mjs";

export function QueueConsumers({api, queue, language, declarationETag, route, router, refreshSeconds=10}) {
  const query=route.consumerQuery??readConsumerQuery();
  const view=useMemo(()=>createConsumerCollectionRefresh(api,"queue",queue,query),[api,queue,query.q,query.mode,query.order,query.offset,query.limit]);
  const {model,refresh}=view;
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const [search, setSearch] = useState(route.consumerQuery?.q ?? "");
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0),clock=Date.now();
  useEffect(()=>setSearch(query.q),[query.q]);
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{view.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[view,refresh]);
  function changeQuery(changes) {
    const reset = ["q","mode","order","limit"].some(key => Object.hasOwn(changes,key));
    const query = {...(route.consumerQuery ?? readConsumerQuery()), ...changes, ...(reset ? {offset:0} : {})};
    if (!router.navigate(queueConsumersURL(queue,query))) void refresh.refresh();
  }
  const zh = language === "zh", t = (en, cn) => zh ? cn : en;
  const unknown = t("Unknown", "未知");
  const errors = {denied: t("Read denied; check credentials and role.", "读取被拒绝，请检查凭据与角色。"), changed: t("Declaration changed during observation. Refresh explicitly.", "观测期间声明发生变化，请手动刷新。"), missing: t("Queue declaration not found.", "Queue 声明不存在。"), query: t("Query rejected; search limit is 256 normalized UTF-8 bytes.", "查询被拒绝，搜索上限为规范化后的 256 个 UTF-8 字节。"), invalid: t("Invalid collection response; no partial results shown.", "集合响应无效，不展示部分结果。"), unavailable: t("Observation unavailable; not an empty collection.", "观测不可用，不代表集合为空。")};
  const page = state.page;
  const names = {present: t("Present", "存在"), missing: t("Missing", "缺失"), matching: t("Matching", "匹配"), different: t("Different owner", "其他所有者"), unmarked: t("Unmarked", "无标记"), unknown};
  return <section aria-labelledby="consumers-heading" className="queue-list">
    <h2 id="consumers-heading">{t("Consumers", "消费者")}</h2>
    <p>{t("Declared and observed Consumers in the Queue's Stream. Filters apply before pagination. Counters are per Consumer, not Queue totals.", "Queue 对应 Stream 中的声明与观测 Consumer。先筛选再分页，计数属于单个 Consumer，不是 Queue 汇总。")}</p>
    <p>{paused?t("Tab hidden; automatic refresh paused.","标签页隐藏，自动刷新已暂停。"):refreshSeconds===0?t("Automatic refresh disabled; manual refresh only.","自动刷新已关闭，仅手动刷新。"):t(`Automatic collection refresh: ${refreshSeconds} seconds after each read; failure backoff up to 60 seconds.`,`集合自动刷新：每次读取完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。`)}</p>
    <form className="list-controls" onSubmit={event => { event.preventDefault(); changeQuery({q: search, offset: 0}); }}>
      <label htmlFor="consumer-search">{t("Consumer name or filter Subject", "Consumer 名称或过滤 Subject")}</label>
      <input id="consumer-search" value={search} onChange={event => setSearch(event.target.value)} />
      <button type="submit">{t("Filter Consumers", "筛选消费者")}</button>
      <label htmlFor="consumer-mode">{t("Delivery mode", "投递模式")}</label>
      <select id="consumer-mode" value={state.query.mode} onChange={event => changeQuery({mode: event.target.value})}><option value="">{t("All modes", "所有模式")}</option><option value="pull">Pull</option><option value="push">Push</option></select>
      <label htmlFor="consumer-order">{t("Consumer order", "消费者排序")}</label>
      <select id="consumer-order" value={state.query.order} onChange={event => changeQuery({order: event.target.value})}><option value="asc">{t("Name ascending", "名称升序")}</option><option value="desc">{t("Name descending", "名称降序")}</option></select>
      <button type="button" disabled={paused||state.phase === "loading"} onClick={() => void refresh.refresh()}>{t("Refresh Consumers", "刷新消费者")}</button>
    </form>
    {state.phase === "loading" && <p role="status">{page?t("Refreshing; retaining last Consumer page…","正在刷新，保留上次 Consumer 页…"):t("Loading Consumers…", "正在读取消费者…")}</p>}
    {state.failure && <p role="alert">{state.failure==="disabled"?t("Resource read API disabled; prior data cleared.","资源读取接口已禁用，已清除此前数据。"):errors[state.failure]}</p>}
    {page && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{t("Rows, totals and declaration revision below are from the last successful read of this query, not current observations.","以下行、总数及声明版本来自此查询上次成功读取，不代表当前观测。")}</p>}
      {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{t("Stale collection observation; the 30-second freshness threshold is not health evidence.","旧集合观测；30 秒新鲜度阈值不是健康证明。")}</p>}
      {(!declarationETag || page.declaration_revision !== declarationETag) && <p role="status">{t("This collection and the declaration above are not verified at the same revision. Do not treat them as one snapshot; reload the declaration before comparing.", "集合与上方声明未确认处于同一版本，不能视作同一快照；比较前请重新加载声明。")}</p>}
      <dl><dt>Stream</dt><dd>{page.stream} · {names[page.stream_status]} · {names[page.stream_ownership]}</dd>
        <dt>{t("Collection declaration ETag", "集合声明 ETag")}</dt><dd>{page.declaration_revision}</dd>
        <dt>{t("Collection read completed", "集合读取完成时间")}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd></dl>
      <p role="status">{t("Filtered total", "筛选后总数")}: {page.total}</p>
      {page.items.length === 0 ? <p>{page.total > 0 ? t("This page is empty after collection changes; return to the previous page.", "集合变化后当前页为空，请返回上一页。") : state.query.q || state.query.mode ? t("No matching Consumers.", "没有匹配的消费者。") : t("No Consumers in this collection.", "该集合中没有消费者。")}</p> :
        <div className="table-scroll" tabIndex="0" role="region" aria-label={t("Consumer collection — scroll horizontally for all columns", "消费者集合——横向滚动查看全部列")}><table className="consumer-table">
          <thead><tr>{["Consumer", t("Declared", "已声明"), t("Observed", "观测状态"), t("Ownership", "所有权"), t("Mode", "模式"), t("Filter Subjects", "过滤 Subject"), t("Pending", "待投递"), t("Ack pending", "待确认")].map(label => <th scope="col" key={label}>{label}</th>)}</tr></thead>
          <tbody>{page.items.map(row => {
            const subjects = row.observed ? (row.observed.filter_subjects?.length ? row.observed.filter_subjects : row.observed.filter_subject ? [row.observed.filter_subject] : []) : row.expected?.filterSubjects;
            return <tr key={`${row.stream}/${row.name}`}><th scope="row"><a href={consumerDetailURL(row.stream,row.name)} onClick={event => { if (event.button===0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {event.preventDefault();router.navigate(consumerDetailURL(row.stream,row.name));} }}>{row.name}</a></th><td>{row.expected ? t("Yes", "是") : t("No (external)", "否（外部）")}</td><td>{names[row.status]}</td><td>{names[row.ownership]}</td><td>{row.observed?.mode ?? row.expected?.mode ?? unknown}</td><td>{subjects?.length ? subjects.join(", ") : t("No filter", "无过滤")}</td><td>{consumerCounter(row,"pending") ?? unknown}</td><td>{consumerCounter(row,"ack_pending") ?? unknown}</td></tr>;
          })}</tbody>
        </table></div>}
      <nav className="pagination" aria-label={t("Consumer pages", "消费者分页")}>
        <button disabled={page.offset === 0} onClick={() => changeQuery({offset: page.items.length ? Math.max(0,page.offset-page.limit) : Math.max(0,Math.floor((page.total-1)/page.limit)*page.limit)})}>{t("Previous Consumers", "上一页消费者")}</button>
        <span>{page.items.length ? `${page.offset+1}–${page.offset+page.items.length}` : "0"} / {page.total}</span>
        <button disabled={page.offset+page.items.length >= page.total} onClick={() => changeQuery({offset: page.offset+page.limit})}>{t("Next Consumers", "下一页消费者")}</button>
      </nav>
    </>}
  </section>;
}
