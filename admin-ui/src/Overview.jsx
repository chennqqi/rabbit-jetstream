import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createOverview} from "./overview.mjs";
import {createNodes} from "./nodes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";
import {systemLabels} from "./system-labels.mjs";
import {MetricHistory} from "./MetricHistory.jsx";

export function Overview({api,language,router,refreshSeconds=10}) {
  const zh=language==="zh",unknown=zh?"未知":"Unknown",accessible=systemLabels(language);
  const model=useMemo(()=>createOverview(api),[api]),nodes=useMemo(()=>createNodes(api,{retainOnRefresh:true}),[api]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),monitor=useSyncExternalStore(nodes.subscribe,nodes.snapshot);
  const [,setClock]=useState(Date.now()),[paused,setPaused]=useState(document.hidden);
  const clock=Date.now(); // Compare with this render, not the previous timer tick.
  const refresh=useMemo(()=>createRefreshLoop(async()=>{
    await Promise.all([model.load(),nodes.load()]);
    return !model.snapshot().info.failure&&!model.snapshot().queues.failure&&!nodes.snapshot().failure&&!nodes.snapshot().snapshot?.nodes.some(node=>node.status!=="available");
  }),[model,nodes]);
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{refresh.stop();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);model.clear();nodes.clear();};
  },[model,nodes,refresh]);
  const count=(object,key)=>consumerCounter({observed:object},key)??unknown;
  const link=(path,label)=><a href={path} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(path);}}}>{label}</a>;
  function evidence(source) {return <>
    {source.phase==="loading"&&<p role="status">{source.readAt?(zh?"正在刷新，保留上次数据…":"Refreshing; retaining last data…"):(zh?"正在读取…":"Loading…")}</p>}
    {source.failure&&<p role="alert">{source.failure==="denied"?(zh?"读取被拒绝，请检查凭据和权限。":"Read denied; check credentials and permissions."):source.failure==="disabled"?(zh?"资源读取接口已禁用，已清除此前数据。请检查服务端认证配置。":"Resource read API disabled; prior data cleared. Check server authentication configuration."):(zh?"此来源不可用或响应无效，不代表资源数量为零。":"This source is unavailable or invalid; resource counts are not zero.")}</p>}
    {source.readAt&&<p>{zh?"读取完成":"Read completed"}: <time dateTime={source.readAt}>{source.readAt}</time></p>}
    {source.readAt&&(paused||source.failure||clock-Date.parse(source.readAt)>30000||clock<Date.parse(source.readAt))&&<p role="status">{zh?"旧观测数据：已暂停、刷新失败或超过 30 秒；不代表当前健康状态。":"Stale observation: paused, refresh failed or older than 30 seconds; not current health evidence."}</p>}
  </>;}
  const issues=monitor.snapshot?.nodes.filter(node=>node.status!=="available")??[];
  return <section aria-label={accessible.overview}>
    <h2>{zh?"总览":"Overview"}</h2>
    <p>{zh?"各来源独立读取，不是同一时刻的原子快照。监控读取成功不代表健康、HA 或发布资格。":"Sources are read independently, not as an atomic snapshot. Successful monitoring reads do not establish health, HA or release qualification."}</p>
    <p>{paused?(zh?"标签页隐藏，自动刷新已暂停。":"Tab hidden; automatic refresh paused."):refreshSeconds===0?(zh?"自动刷新已关闭，仅手动刷新。":"Automatic refresh disabled; manual refresh only."):(zh?`自动刷新：每批完成后 ${refreshSeconds} 秒；失败退避最长 60 秒。`:`Automatic refresh: ${refreshSeconds} seconds after each batch; failure backoff up to 60 seconds.`)} {zh?"30 秒为控制台新鲜度阈值，不是 Broker 健康阈值。":"The 30-second freshness threshold is a console rule, not a broker health threshold."}</p>
    <button onClick={()=>void refresh.refresh()} disabled={paused||state.info.phase==="loading"||state.queues.phase==="loading"||monitor.phase==="loading"}>{zh?"刷新总览":"Refresh overview"}</button>
    <section aria-label={accessible.monitoring}><h3>{zh?"监控问题与覆盖范围":"Monitoring issues and coverage"}</h3>
      {evidence(monitor)}
      {monitor.snapshot&&<>
        {monitor.snapshot.total===0?<p>{zh?"未配置监控端点，无法判断节点情况。":"No monitoring endpoints configured; node state is unknown."}</p>:<p>{zh?"配置端点数":"Configured endpoints"}: {monitor.snapshot.total} · {zh?"读取失败或部分失败":"Failed or partial reads"}: {issues.length}</p>}
        {issues.length>0&&<ul>{issues.map((node,index)=><li key={index}>{node.endpoint}: {node.status==="unavailable"?(zh?"不可达":"Unavailable"):(zh?"部分读取失败":"Partial read failure")}</li>)}</ul>}
        <p>{zh?"覆盖范围仅限配置的端点；问题不是已发送的告警。":"Coverage is limited to configured endpoints; these observations are not delivered alerts."}</p>
      </>}
      {link("/admin/nodes",zh?"查看节点":"View nodes")}
    </section>
    <section aria-label={accessible.account}><h3>{zh?"管理服务与账户统计":"Management and account"}</h3>
      {evidence(state.info)}
      {state.info.value&&<>
        <dl>{[[zh?"管理服务名称":"Management service name",state.info.value.name],[zh?"管理服务版本":"Management service version",state.info.value.version],[zh?"管理服务运行秒数":"Management uptime seconds",count(state.info.value,"uptime_seconds")],[zh?"账户 Stream 数":"Account Streams",count(state.info.value.jetstream,"streams")],[zh?"账户 Consumer 数":"Account Consumers",count(state.info.value.jetstream,"consumers")],[zh?"账户内存使用（字节）":"Account memory used (bytes)",count(state.info.value.jetstream,"memory_used")],[zh?"账户存储使用（字节）":"Account storage used (bytes)",count(state.info.value.jetstream,"storage_used")]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{value}</dd></React.Fragment>)}</dl>
        <p>{zh?"服务名称不等于声明的部署规格；Stream/Consumer 统计包含外部资源，不代表 Queue 数或待投递数。":"Service name is not declared deployment intent. Stream/Consumer totals include external resources and are not Queue or pending-message counts."}</p>
      </>}
      {link("/admin/streams",zh?"查看 Streams":"View Streams")}
    </section>
    <section aria-label={accessible.queues}><h3>{zh?"已声明 Queue":"Declared Queues"}</h3>
      {evidence(state.queues)}
      {state.queues.value&&<p>{zh?"声明总数":"Declaration total"}: {state.queues.value.total}</p>}
      <p>{zh?"使用服务端完整集合总数，不使用当前页行数，不代表资源已收敛。":"Uses the server collection total, not loaded page length; this does not establish resource convergence."}</p>
      {link("/admin/queues",zh?"查看 Queues":"View Queues")}
    </section>
    <MetricHistory api={api} language={language}/>
  </section>;
}
