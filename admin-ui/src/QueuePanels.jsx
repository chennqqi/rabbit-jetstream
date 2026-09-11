import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createSummaryRefresh} from "./summary-refresh.mjs";
import {createAuditList} from "./audit-list.mjs";
import {readAuditQuery,auditURL} from "./audit-query.mjs";
import {streamDetailURL,queueTabURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";
import {SummaryMetrics} from "./SummaryMetrics.jsx";
import {ExactDuration,ReadTime} from "./DisplayValues.jsx";
import {summaryLabels} from "./summary-labels.mjs";

export function QueueSummary({api,plan,language,router,route,refreshSeconds=10}) {
  const zh=language==="zh",name=plan.stream.name,accessible=summaryLabels(language);
  const view=useMemo(()=>createSummaryRefresh(api,plan),[api,plan]),{stream,consumer,refresh}=view;
  const state=useSyncExternalStore(stream.subscribe,stream.snapshot),consumerState=useSyncExternalStore(consumer.subscribe,consumer.snapshot);
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0),clock=Date.now();
  const busy=paused||state.phase==="loading"||consumerState.phase==="loading";
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{view.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[view,refresh]);
  return <><div className="queue-summary-grid"><section className="summary-evidence"><header className="observation-heading"><h3>{zh?"实际 Stream 观测":"Observed Stream"}: {name}</h3>
    <button disabled={busy} onClick={()=>void refresh.refresh("stream")}>{zh?"刷新观测":"Refresh observation"}</button>
    </header><details className="reading-help"><summary>{zh?"独立读取，不代表收敛 · 观测说明":"Independent reads, not convergence — observation guide"}</summary><p>{zh?"存储消息数不是 Consumer 待投递数；声明和观测分别读取，不构成收敛或健康结论。":"Stored messages are not Consumer pending messages. Declaration and observation are separate reads, not a convergence or health verdict."}</p></details>
    {state.phase==="loading"&&<p role="status">{zh?"正在读取…":"Loading…"}</p>}
    <p>{paused?(zh?"标签页隐藏，摘要自动刷新已暂停。":"Tab hidden; automatic summary refresh paused."):refreshSeconds===0?(zh?"摘要自动刷新已关闭，仅手动刷新。":"Automatic summary refresh disabled; manual refresh only."):(zh?`摘要自动刷新：每批完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。不刷新声明。`:`Automatic summary refresh: ${refreshSeconds} seconds after each batch; failure backoff up to 60 seconds. Declaration is not refreshed.`)}</p>
    {state.failure&&<p role="alert">{state.failure==="missing"?(zh?"声明对应的 Stream 本次观测缺失。":"Declared Stream is missing in this observation."):(zh?"Stream 观测失败或无权限。仅在下方明确标记时保留历史指标。":"Stream observation failed or was denied. Historical metrics are retained only when explicitly labeled below.")}</p>}
    {state.resource&&(state.phase==="loading"||state.failure)&&<p role="status">{zh?"Stream 指标和副本信息来自上次成功读取，不代表当前观测。":"Stream metrics and replica information are from the last successful read, not current observations."}</p>}
    {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{zh?"Stream 为旧观测，30 秒阈值不是健康证明。":"Stale Stream observation; the 30-second threshold is not health evidence."}</p>}
    <SummaryMetrics scope={view.scope} consumerState={consumerState} streamState={state} language={language} router={router} disabled={busy} paused={paused} clock={clock} onRefresh={()=>void refresh.refresh("consumer")}/>
    {state.resource&&<><ReplicaEvidence stream={state.resource} desired={plan.stream.replicas} language={language}/><dl>{[["bytes",zh?"存储字节":"Stored bytes"],["consumers",zh?"实际 Consumer 数":"Observed Consumers"]].map(([field,label])=><React.Fragment key={field}><dt>{label}</dt><dd>{consumerCounter({observed:state.resource},field)??(zh?"未知":"Unknown")}</dd></React.Fragment>)}</dl><ReadTime value={state.readAt} language={language}/></>}
    <p><a href={streamDetailURL(name)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(streamDetailURL(name));}}}>{zh?"查看 Stream 详情":"View Stream detail"}</a></p>
  </section><section className="summary-config" aria-label={zh?"Queue 声明配置":"Queue declared configuration"}><h3>{zh?"Queue 声明配置":"Queue declared configuration"}</h3><p>{zh?"声明值，不是实时观测。":"Declared values, not live observations."}</p><dl>{[["Stream",name],[zh?"存储":"Storage",plan.stream.storage],[zh?"副本":"Replicas",plan.stream.replicas],[zh?"保留时间上限":"Max age",plan.stream.maxAgeNanos,"duration"],[zh?"ACK 等待":"ACK wait",plan.consumer?.ackWaitNanos,"duration"],[zh?"最大投递":"Max deliveries",plan.consumer?.maxDeliver]].map(([label,value,kind])=><div key={label}><dt>{label}</dt><dd>{kind==="duration"?<ExactDuration value={value} language={language}/>:value===undefined||value===null?(zh?"未知":"Unknown"):String(value)}</dd></div>)}</dl><p>{zh?"保留时间上限为 0 表示无此项时间限制；其他保留策略仍有效。原始纳秒值可在配置标签的 Plan 中查看。":"Max age 0 means no age limit; other retention policies still apply. Raw nanoseconds remain available in the Configuration tab's Plan."}</p></section></div>
    <section aria-label={accessible.diagnostics}><h3>{zh?"进一步排查 Consumer":"Investigate Consumers"}</h3><p>{zh?"列表包含预期、缺失及额外 Consumer；在那里按名称、过滤 Subject 或模式筛选，再进入精确详情。":"The collection includes expected, missing and additional Consumers. Filter there by name, filter Subject or mode, then open exact detail."}</p><a href={queueTabURL(plan.queue,"consumers",route?.consumerQuery)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(event.currentTarget.getAttribute("href"));}}}>{zh?"查看 Consumer 列表":"View Consumer collection"}</a></section></>;
}

export function QueueEvents({api,name,language,router,route}) {
  const zh=language==="zh",model=useMemo(()=>createAuditList(api),[api]);
  const query=useMemo(()=>readAuditQuery(new URLSearchParams({resource:name}).toString()),[name]);
  const before=route.eventBefore??null,state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load({...query,before});return()=>model.clear();},[model,query,before]);
  return <section><h3>{zh?"Queue 管理审计":"Queue management audit"}</h3>
    <p>{zh?"仅按资源名称关联管理事件，不是消息投递历史，不证明某次写入已收敛。":"Management events correlated by resource name, not message-delivery history or proof of convergence."}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load({...query,before})}>{zh?"刷新事件窗口":"Refresh event window"}</button>
    {state.phase==="loading"&&<p role="status">{zh?"正在读取事件…":"Loading events…"}</p>}
    {state.failure&&<p role="alert">{zh?"审计读取失败或被拒绝，不代表没有事件。":"Audit read failed or denied; this does not mean no events exist."}</p>}
    {state.page&&<><p>{zh?"当前窗口匹配数":"Matches in this window"}: {state.page.items.length} · {state.readAt}</p>
      {state.page.items.length===0&&<p>{zh?"当前窗口无匹配；不证明从未发生操作。":"No matches in this window; absence of operations is not established."}</p>}
      {state.page.items.map(event=><details key={String(event.sequence)}><summary>{event.time} · {event.action} · {event.phase} · {event.id}</summary><pre className="declaration-json">{stringifyJSON(event)}</pre></details>)}
      <button disabled={state.page.nextBefore===null} onClick={()=>router.navigate(queueTabURL(name,"events",route.consumerQuery,String(state.page.nextBefore)))}>{zh?"更早事件窗口":"Older event window"}</button></>}
    <p><a href={auditURL(query)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(auditURL(query));}}}>{zh?"打开完整审计筛选与导出":"Open full audit filters and export"}</a></p>
  </section>;
}
