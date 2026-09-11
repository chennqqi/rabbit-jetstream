import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createConnectionSubscriptions,subscriptionPage} from "./connection-subscriptions.mjs";

export function ConnectionSubscriptions({api,id,cid,language}){
  const t=(en,zh)=>language==="zh"?zh:en,model=useMemo(()=>createConnectionSubscriptions(api,id,cid),[api,id,cid]),state=useSyncExternalStore(model.subscribe,model.snapshot),[query,setQuery]=useState(""),[offset,setOffset]=useState(0),limit=50;
  useEffect(()=>()=>model.clear(),[model]);
  let page=null;try{if(state.observation)page=subscriptionPage(state.observation,query,offset,limit);}catch{}
  useEffect(()=>{if(page&&offset>=page.total&&offset!==0)setOffset(Math.max(0,Math.floor(Math.max(0,page.total-1)/limit)*limit));},[page?.total,offset]);
  const errors={denied:t("Subscription read denied.","订阅读取被拒绝。"),disabled:t("Resource reads disabled.","资源读取已禁用。"),limit:t("More than 1,000 subscriptions were reported; expansion was refused without partial rows.","报告的订阅超过 1,000 条；已拒绝展开且不返回部分行。"),missing:t("The open connection disappeared before subscription observation.","订阅观测前打开的连接已消失。"),node_missing:t("Node not found in complete configured-endpoint coverage.","完整配置端点覆盖中未找到节点。"),ambiguous:t("Node identity is ambiguous.","节点身份有歧义。"),query:t("Subscription request rejected.","订阅请求被拒绝。"),invalid:t("Invalid subscription response; data cleared.","订阅响应无效，数据已清除。"),unavailable:t("Subscription observation unavailable; no empty result is inferred.","订阅观测不可用，不会据此推断为空。")};
  return <section className="connection-subscriptions" aria-label={t("Connection subscriptions","连接订阅")}>
    <h3>{t("Subscriptions","订阅")}</h3>
    <p>{t("Load explicitly. Subjects, queue groups and exact SIDs may contain business-sensitive identifiers. This complete bounded observation is not Consumer ownership, backlog or health, and performs no message operation.","仅显式加载。Subject、队列组和精确 SID 可能包含业务敏感标识。此有界完整观测不代表 Consumer 所属关系、积压或健康，也不会执行消息操作。")}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{state.observation?t("Refresh subscriptions","刷新订阅"):t("Load subscriptions","加载订阅")}</button>
    {state.phase==="loading"&&<p role="status">{t("Reading a bounded subscription observation…","正在读取有界订阅观测…")}</p>}{state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {state.observation&&page&&<><p>{t("Observed","观测时间")}: <time dateTime={state.observation.observed_at}>{state.observation.observed_at}</time> · {t("Management read","管理读取")}: <time dateTime={state.observation.read_at}>{state.observation.read_at}</time></p>
      {state.failure==="unavailable"&&<p role="status">{t("Showing the last successful historical observation.","显示上一次成功的历史观测。")}</p>}
      <label>{t("Filter this complete observation","筛选此完整观测")}<input value={query} maxLength="256" onChange={event=>{setQuery(event.target.value);setOffset(0);}} /></label>
      {page.items.length?<div className="table-scroll" role="region" tabIndex="0" aria-label={t("Subscription rows","订阅行")}><table><thead><tr><th>SID</th><th>Subject</th><th>{t("Queue group","队列组")}</th><th>{t("Delivered messages","已投递消息")}</th><th>{t("Auto-unsubscribe maximum","自动退订上限")}</th></tr></thead><tbody>{page.items.map(row=><tr key={row.sid}><th scope="row">{row.sid}</th><td>{row.subject}</td><td>{row.queue??t("None","无")}</td><td>{String(row.messages)}</td><td>{row.maximum===undefined?t("Unreported","未报告"):String(row.maximum)}</td></tr>)}</tbody></table></div>:<p>{page.total===0&&query?t("No subscriptions match this local filter.","没有订阅匹配此本地筛选。") : t("No subscriptions were reported in this observation.","此观测未报告订阅。")}</p>}
      <nav aria-label={t("Subscription pages","订阅分页")}><button disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-limit))}>{t("Previous","上一页")}</button><span>{page.items.length?`${offset+1}–${offset+page.items.length}`:"0"} / {page.total}</span><button disabled={offset+limit>=page.total} onClick={()=>setOffset(offset+limit)}>{t("Next","下一页")}</button></nav></>}
  </section>;
}
