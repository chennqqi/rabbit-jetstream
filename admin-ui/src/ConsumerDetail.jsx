import React, {useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createConsumerRefresh} from "./consumer-refresh.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {ConsumerDiagnosis} from "./ConsumerDiagnosis.jsx";
import {createNodeRefresh} from "./node-refresh.mjs";

export function ConsumerDetail({api,stream,name,language,refreshSeconds=10}) {
  const view=useMemo(()=>createConsumerRefresh(api,stream,name),[api,stream,name]);
  const {model,refresh}=view;
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  const nodeView=useMemo(()=>createNodeRefresh(api),[api]);
  const nodeState=useSyncExternalStore(nodeView.model.subscribe,nodeView.model.snapshot);
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0),clock=Date.now();
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);nodeView.refresh.setInterval(refreshSeconds*1000);},[refresh,nodeView,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);nodeView.refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);nodeView.refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{view.clear();nodeView.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[view,refresh,nodeView]);
  const t=(en,zh)=>language==="zh"?zh:en, unknown=t("Unknown","未知");
  const errors={missing:t("Consumer not found at this observation. It may have been removed.","本次观测未找到 Consumer，可能已被删除。"),denied:t("Read denied. Check credentials and permissions.","读取被拒绝，请检查凭据与权限。"),invalid:t("Invalid Consumer response; no partial data shown.","Consumer 响应无效，不展示部分数据。"),unavailable:t("Consumer observation unavailable; not a missing or empty Consumer.","Consumer 观测不可用，不等于 Consumer 缺失或为空。")};
  const resource=state.resource;
  return <section className="queue-list" aria-labelledby="consumer-heading">
    <h2 id="consumer-heading">Consumer: {name}</h2><p>Stream: {stream}</p>
    <p>{t("Exact observed Consumer, independent of list pagination. Counters are not Queue totals; this is not a health verdict.","独立于列表分页的精确 Consumer 观测。计数不是 Queue 汇总，也不代表健康结论。")}</p>
    <p>{paused?t("Tab hidden; automatic refresh paused.","标签页隐藏，自动刷新已暂停。"):refreshSeconds===0?t("Automatic refresh disabled; manual refresh only.","自动刷新已关闭，仅手动刷新。"):t(`Automatic refresh: ${refreshSeconds} seconds after each read; failure backoff up to 60 seconds.`,`自动刷新：每次读取完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。`)}</p>
    <button disabled={paused||state.phase==="loading"||nodeState.phase==="loading"} onClick={()=>void Promise.all([refresh.refresh(),nodeView.refresh.refresh()])}>{t("Refresh Consumer","刷新 Consumer")}</button>
    {state.phase==="loading" && <p role="status">{resource?t("Refreshing; retaining last Consumer observation…","正在刷新，保留上次 Consumer 观测…"):t("Loading Consumer…","正在读取 Consumer…")}</p>}
    {state.failure && <p role="alert">{state.failure==="disabled"?t("Resource read API disabled; prior data cleared. Check server authentication configuration.","资源读取接口已禁用，已清除此前数据。请检查服务端认证配置。"):errors[state.failure]}</p>}
    {resource && <>
      {(state.phase==="loading"||state.failure)&&<p role="status">{t("Configuration and counters below are from the last successful read, not current observations.","以下配置和计数来自上次成功读取，不代表当前观测。")}</p>}
      {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{t("Stale observation: paused, refresh failed or older than 30 seconds; this threshold is not Consumer health evidence.","旧观测数据：已暂停、刷新失败或超过 30 秒；此阈值不表示 Consumer 健康状态。")}</p>}
      <dl><dt>{t("Read completed","读取完成时间")}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd>
        <dt>{t("Mode","模式")}</dt><dd>{resource.mode}</dd>
        <dt>Durable</dt><dd>{resource.durable || t("Not configured","未配置")}</dd>
        <dt>{t("Filter Subjects","过滤 Subject")}</dt><dd>{(resource.filter_subjects?.length ? resource.filter_subjects : resource.filter_subject ? [resource.filter_subject] : []).join(", ") || t("No filter","无过滤")}</dd>
        {[["pending","Pending","待投递"],["ack_pending","Ack pending","待确认"],["redelivered","Redelivered","重投"],["waiting","Waiting pulls","等待中的 Pull 请求"]].map(([key,en,zh])=><React.Fragment key={key}><dt>{t(en,zh)}</dt><dd>{consumerCounter({observed:resource},key)??unknown}</dd></React.Fragment>)}
        <dt>{t("ACK policy","ACK 策略")}</dt><dd>{resource.ack_policy??unknown}</dd>
        <dt>{t("ACK wait (nanoseconds)","ACK 等待（纳秒）")}</dt><dd>{consumerCounter({observed:resource},"ack_wait_nanos")??unknown}</dd>
      </dl>
      <details><summary>{t("Raw observation (exact integers)","原始观测（精确整数）")}</summary><pre className="declaration-json">{stringifyJSON(resource)}</pre></details>
      <ConsumerDiagnosis state={state} nodeState={nodeState} language={language} now={clock} paused={paused}/>
    </>}
  </section>;
}
