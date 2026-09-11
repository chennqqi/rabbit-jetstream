import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createConnectionDetail} from "./connection-detail.mjs";
import {nodeConnectionsURL} from "./routes.mjs";
import {connectionLabels} from "./connection-labels.mjs";
import {ConnectionSubscriptions} from "./ConnectionSubscriptions.jsx";

export function ConnectionDetail({api,route,router,language,refreshSeconds=10}){
  const t=(en,zh)=>language==="zh"?zh:en,{id,cid,query}=route,accessible=connectionLabels(language);
  const view=useMemo(()=>createConnectionDetail(api,id,cid),[api,id,cid]),state=useSyncExternalStore(view.subscribe,view.snapshot),{detail}=state;
  const [hidden,setHidden]=useState(document.hidden),[,tick]=useState(0),now=Date.now(),back=nodeConnectionsURL(id,query);
  useEffect(()=>{view.refresh.setInterval(refreshSeconds*1000);},[view,refreshSeconds]);
  useEffect(()=>{const changed=()=>{setHidden(document.hidden);view.refresh.visibility(document.hidden);};setHidden(document.hidden);document.addEventListener("visibilitychange",changed);view.refresh.start(document.hidden);const timer=setInterval(()=>tick(Date.now()),1000);return()=>{clearInterval(timer);document.removeEventListener("visibilitychange",changed);view.clear();};},[view]);
  const errors={missing:t("This open connection was not found on the observed node. Its disappearance reason is unknown.","观测节点上未找到此打开连接，消失原因未知。"),node_missing:t("Node not found in complete configured-endpoint coverage.","完整配置端点覆盖中未找到节点。"),ambiguous:t("Node identity is ambiguous; connection evidence cleared.","节点身份有歧义，连接证据已清除。"),denied:t("Connection read denied.","连接读取被拒绝。"),disabled:t("Resource reads disabled.","资源读取已禁用。"),query:t("Connection query rejected.","连接查询被拒绝。"),invalid:t("Invalid connection response; old data cleared.","连接响应无效，旧数据已清除。"),unavailable:t("Connection observation unavailable; absence is not established.","连接观测不可用，不能据此判断连接不存在。")};
  return <section className="connection-detail" aria-label={accessible.detail}>
    <a href={back} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(back);}}}>{t("Back to connections","返回连接列表")}</a>
    <h2>{t("Connection","连接")}: {cid}</h2><p>{t("Node","节点")}: {id}</p>
    <p>{t("Identity is node ID plus CID. This read is not connection history, Consumer health or a delivery guarantee. General client metadata remains excluded; bounded subscription detail is a separate explicit read below.","身份由节点 ID 与 CID 共同确定。本次读取不是连接历史、Consumer 健康状况或投递保证。通用客户端元数据仍被排除；下方有界订阅详情是独立的显式读取。")}</p>
    <p>{hidden?t("Automatic refresh paused while hidden.","隐藏时暂停自动刷新。"):refreshSeconds===0?t("Automatic refresh disabled; manual only.","自动刷新已关闭，仅手动刷新。"):t(`Refresh ${refreshSeconds}s after completion; failures back off up to 60s.`,`完成后 ${refreshSeconds} 秒刷新，失败退避最长 60 秒。`)}</p>
    <button disabled={hidden||state.phase==="loading"} onClick={()=>void view.refresh.refresh()}>{t("Refresh connection","刷新连接详情")}</button>
    {state.phase==="loading"&&<p role="status">{t("Reading connection…","正在读取连接…")}</p>}{state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {detail&&<>{(state.phase==="loading"||state.failure)&&<p role="status">{t("Historical connection evidence from the last successful read, not a current observation.","上次成功读取的历史连接证据，并非当前观测。")}</p>}
      {(hidden||state.failure||now-Date.parse(state.readAt)>30000||now<Date.parse(state.readAt))&&<p role="status">{t("Stale observation; freshness is not health.","旧观测；新鲜度不代表健康。")}</p>}
      <p>{t("Server observation","服务端观测")}: <time dateTime={detail.observed_at}>{detail.observed_at}</time> · {t("Management read","管理读取")}: <time dateTime={detail.read_at}>{detail.read_at}</time></p>
      <p>{t("Message and byte counters are cumulative for this connection. Pending bytes and subscriptions are current gauges, not Consumer backlog or rates.","消息和字节计数为此连接的累计值。待发送字节及订阅数为当前值，不是 Consumer 积压或速率。")}</p>
      <dl>{[["pending_bytes","Pending bytes","待发送字节"],["in_msgs","Received messages","接收消息"],["out_msgs","Sent messages","发送消息"],["in_bytes","Received bytes","接收字节"],["out_bytes","Sent bytes","发送字节"],["subscriptions","Subscriptions","订阅数"]].map(([key,en,zh])=><React.Fragment key={key}><dt>{t(en,zh)}</dt><dd>{detail.item[key]===undefined?t("Unknown","未知"):String(detail.item[key])}</dd></React.Fragment>)}</dl>
      <ConnectionSubscriptions api={api} id={id} cid={cid} language={language}/>
    </>}
  </section>;
}
