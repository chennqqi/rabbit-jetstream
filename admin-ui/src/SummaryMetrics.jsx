import React from "react";
import {summaryConsumerCounts} from "./summary-consumer.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {summaryLabels} from "./summary-labels.mjs";

export function SummaryMetrics(props) {
  const scope=props.scope;
  return scope?<ScopedMetrics {...props} scope={scope} key={`${scope.stream}/${scope.name}`}/>:<p role="alert">{props.language==="zh"?"声明未提供有效的主 Consumer 身份，无法读取其指标。":"Declaration has no valid primary Consumer identity; its metrics cannot be read."}</p>;
}

function ScopedMetrics({scope,streamState,consumerState:state,language,router,onRefresh,disabled,paused,clock}) {
  const zh=language==="zh",unknown=zh?"未知":"Unknown",accessible=summaryLabels(language);
  const counts=summaryConsumerCounts(scope,state,{allowHistorical:true});
  return <div role="group" aria-label={accessible.metrics}>
    <p>{zh?"声明指定的主 Consumer":"Declared primary Consumer"}: <a href={scope.url} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(scope.url);}}}>{scope.stream} / {scope.name}</a></p>
    <button disabled={disabled} onClick={onRefresh}>{zh?"刷新摘要 Consumer":"Refresh summary Consumer"}</button>
    {state.phase==="loading"&&<p role="status">{zh?"正在读取主 Consumer…":"Loading primary Consumer…"}</p>}
    {state.failure&&<p role="alert">{state.failure==="missing"?(zh?"本次未找到声明指定的主 Consumer；不是零积压。":"Declared primary Consumer was not found in this read; this is not zero backlog."):(zh?"主 Consumer 读取失败、被拒绝或响应无效。仅在下方明确标记时保留历史指标。":"Primary Consumer read failed, was denied or was invalid. Historical metrics are retained only when explicitly labeled below.")}</p>}
    {state.resource&&(state.phase==="loading"||state.failure)&&<p role="status">{zh?"主 Consumer 指标来自上次成功读取，不代表当前观测。":"Primary Consumer metrics are from the last successful read, not current observations."}</p>}
    {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{zh?"主 Consumer 为旧观测，30 秒阈值不是健康证明。":"Stale primary Consumer observation; the 30-second threshold is not health evidence."}</p>}
    <dl className="summary-metrics">{[[zh?"存储消息数（Stream）":"Stored messages (Stream)",streamState.resource?consumerCounter({observed:streamState.resource},"messages"):null],[zh?"待投递（主 Consumer）":"Pending (primary Consumer)",counts.pending],[zh?"待确认（主 Consumer）":"Ack pending (primary Consumer)",counts.ackPending]].map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value??unknown}</dd></div>)}</dl>
    <p>{zh?"Consumer 读取完成":"Consumer read completed"}: <ReadTime value={state.readAt} language={language}/></p>
    <details className="reading-help"><summary>{zh?"仅此 Consumer，不可汇总 · 指标说明":"This Consumer only; not additive — metric guide"}</summary><p>{zh?"待投递/待确认仅属于此 Consumer，不是整个 Queue 或全部优先级的汇总。Stream 与 Consumer 分别读取，三项数值不构成可相加的消息分类，也不证明归属标记或配置一致。":"Pending/ack-pending belong only to this Consumer, not the whole Queue or all priorities. Stream and Consumer are read separately; these are not additive message categories or proof of ownership markers/configuration agreement."}</p></details>
  </div>;
}
