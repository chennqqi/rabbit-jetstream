import React, {useEffect, useMemo, useState, useSyncExternalStore} from "react";
import {nodeSourceAvailable, nodeJSMetric} from "./nodes.mjs";
import {createNodeRefresh} from "./node-refresh.mjs";
import {nodeDetailURL,nodeConnectionsURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {stringifyJSON} from "./api.mjs";
import {nodeLabels} from "./node-labels.mjs";

export function Nodes({api, language, route, router, refreshSeconds=10}) {
  const zh=language==="zh", unknown=zh?"未知":"Unknown",accessible=nodeLabels(language);
  const view=useMemo(()=>createNodeRefresh(api),[api,route.kind,route.id]);
  const {model,refresh}=view;
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  const [paused,setPaused]=useState(document.hidden),[,setClock]=useState(0),clock=Date.now();
  useEffect(()=>{refresh.setInterval(refreshSeconds*1000);},[refresh,refreshSeconds]);
  useEffect(()=>{
    const visibility=()=>{setPaused(document.hidden);setClock(Date.now());refresh.visibility(document.hidden);};
    setPaused(document.hidden);document.addEventListener("visibilitychange",visibility);refresh.start(document.hidden);
    const tick=setInterval(()=>setClock(Date.now()),1000);
    return()=>{view.clear();clearInterval(tick);document.removeEventListener("visibilitychange",visibility);};
  },[view,refresh]);
  const matches=state.snapshot?.nodes.filter(node=>node.id===route.id)??[];
  const selected=matches.length===1?matches[0]:null;
  let connectionsURL=null;try{if(selected)connectionsURL=nodeConnectionsURL(route.id);}catch{}
  const status=node=>({available:zh?"监控读取成功":"Monitoring reads available",degraded:zh?"部分监控读取失败":"Partial monitoring failure",unavailable:zh?"监控不可达":"Monitoring unavailable"}[node.status]);
  const count=(node,key)=>nodeSourceAvailable(node,"varz") ? consumerCounter({observed:node},key)??unknown : unknown;
  return <section aria-label={accessible.view}>
    <h2>{route.kind==="node" ? `Node: ${route.id}` : (zh?"节点":"Nodes")}</h2>
    <p>{zh?"仅展示已配置的监控端点，不是自动发现的完整集群成员。监控读取成功不代表节点健康或生产资格。":"Configured monitoring endpoints only, not a discovered complete cluster membership. Successful monitoring reads are not a health or production-qualification verdict."}</p>
    <p>{paused?(zh?"标签页隐藏，自动刷新已暂停。":"Tab hidden; automatic refresh paused."):refreshSeconds===0?(zh?"自动刷新已关闭，仅手动刷新。":"Automatic refresh disabled; manual refresh only."):(zh?`自动刷新：每批完成后 ${refreshSeconds} 秒；监控部分失败或不可用时退避最长 60 秒。`:`Automatic refresh: ${refreshSeconds} seconds after each batch; partial or unavailable monitoring backs off up to 60 seconds.`)}</p>
    <button disabled={paused||state.phase==="loading"} onClick={()=>void refresh.refresh()}>{zh?"刷新节点":"Refresh nodes"}</button>
    {state.phase==="loading"&&<p role="status">{state.snapshot?(zh?"正在刷新，保留上次节点观测…":"Refreshing; retaining last node observation…"):(zh?"正在读取节点…":"Loading nodes…")}</p>}
    {state.failure&&<p role="alert">{state.failure==="denied"?(zh?"凭据被拒绝或无读取权限。":"Credentials rejected or read access denied."):state.failure==="disabled"?(zh?"资源读取接口已禁用，已清除此前数据。请检查服务端认证配置。":"Resource read API disabled; prior data cleared. Check server authentication configuration."):(zh?"节点观测不可用或响应无效，不代表没有节点。":"Node observation unavailable or invalid; this does not mean no nodes exist.")}</p>}
    {state.snapshot&&<>
      {(state.phase==="loading"||state.failure)&&<p role="status">{zh?"以下节点、身份及指标来自上次成功读取，不代表当前观测。":"Nodes, identities and metrics below are from the last successful read, not current observations."}</p>}
      <p>{zh?"读取完成":"Read completed"}: <time dateTime={state.readAt}>{state.readAt}</time></p>
      {state.readAt&&(paused||state.failure||clock-Date.parse(state.readAt)>30000||clock<Date.parse(state.readAt))&&<p role="status">{zh?"旧观测数据：已暂停、刷新失败或超过 30 秒；此阈值不表示节点健康状态。":"Stale observation: paused, refresh failed or older than 30 seconds; this threshold is not node health evidence."}</p>}
      {route.kind==="nodes" ? <>
        {state.snapshot.total===0 ? <p>{zh?"没有配置监控端点。":"No monitoring endpoints configured."}</p> : <div className="table-scroll" role="region" tabIndex="0" aria-label={accessible.collection}><table><thead><tr><th>Node ID</th><th>{zh?"监控端点":"Monitoring endpoint"}</th><th>{zh?"读取状态":"Read status"}</th></tr></thead><tbody>{state.snapshot.nodes.map((node,index)=><tr key={`${node.endpoint}-${index}`}><th scope="row">{node.id ? <a href={nodeDetailURL(node.id)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(nodeDetailURL(node.id));}}}>{node.id}</a> : unknown}</th><td>{node.endpoint}</td><td>{status(node)}{node.errors?.map((error,i)=><p key={i}>{error}</p>)}</td></tr>)}</tbody></table></div>}
      </> : !selected ? <p role="alert">{zh?"本次观测无法唯一定位此节点。端点失联、节点重启或重复配置均有可能；不能据此认定节点已删除。":"This observation cannot uniquely identify the node. An unreachable endpoint, restart or duplicate configuration may be responsible; deletion is not established."}</p> : <>
        <p>{status(selected)}</p>
        {connectionsURL&&<p><a href={connectionsURL} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(connectionsURL);}}}>{zh?"查看节点连接":"View node connections"}</a></p>}
        <dl>{[[zh?"名称":"Name",selected.name??unknown],[zh?"监控端点":"Endpoint",selected.endpoint],[zh?"版本":"Version",selected.version??unknown],[zh?"运行时版本":"Go version",selected.go_version??unknown],[zh?"运行时长":"Uptime",selected.uptime??unknown],[zh?"集群名称":"Cluster name",selected.cluster_name??unknown],[zh?"内存（字节）":"Memory bytes",count(selected,"memory_bytes")],[zh?"CPU 核数":"CPU cores",count(selected,"cores")],[zh?"连接数":"Connections",count(selected,"connections")],[zh?"订阅数":"Subscriptions",count(selected,"subscriptions")],[zh?"慢消费者数":"Slow consumers",count(selected,"slow_consumers")],[zh?"接收消息累计数":"Incoming messages (cumulative)",count(selected,"in_messages")],[zh?"发送消息累计数":"Outgoing messages (cumulative)",count(selected,"out_messages")],["CPU %",nodeSourceAvailable(selected,"varz")&&typeof selected.cpu_percent==="number"&&Number.isFinite(selected.cpu_percent)&&selected.cpu_percent>=0?String(selected.cpu_percent):unknown],[zh?"服务端观测时间":"Server observation",nodeSourceAvailable(selected,"varz")&&selected.observed_at&&!selected.observed_at.startsWith("0001-")?selected.observed_at:unknown]].map(([label,value])=><React.Fragment key={label}><dt>{label}</dt><dd>{String(value)}</dd></React.Fragment>)}</dl>
        <p>{zh?"累计消息数不是每秒速率，也不是待投递消息数。":"Cumulative message counters are neither per-second rates nor pending messages."}</p>
        <h3>{zh?"已连接对等节点":"Connected peers"}</h3>
        <p>{nodeSourceAvailable(selected,"routez")&&Array.isArray(selected.connected_peers)?(selected.connected_peers.length?selected.connected_peers.join(", "):(zh?"本次读取未观测到连接的对等节点。":"No connected peers observed in this read.")):unknown}</p>
        <h3>{zh?"各来源读取结果":"Source observations"}</h3>
        <dl>{["varz","routez","jsz"].map(source=><React.Fragment key={source}><dt>{source}</dt><dd>{nodeSourceAvailable(selected,source)?(zh?"读取成功":"Read succeeded"):(zh?"不可用或未读取":"Unavailable or not read")} · {selected.sources?.[source]?.read_at??unknown}</dd></React.Fragment>)}</dl>
        <h3>JetStream</h3>
        <p>{zh?"启用状态":"Enabled"}: {nodeSourceAvailable(selected,"varz")&&typeof selected.jetstream?.enabled==="boolean"?String(selected.jetstream.enabled):unknown}</p>
        <p>{zh?"读取成功不代表每个字段均有报告。缺失、无效或读取失败的指标保持未知，不补零；元数据待处理数不是消息积压。":"A successful read does not mean every field was reported. Missing, invalid or unavailable metrics remain unknown, not zero; metadata pending is not message backlog."}</p>
        <dl aria-label={accessible.metrics}>{[["memory_bytes",zh?"JetStream 内存（字节）":"JetStream memory bytes"],["storage_bytes",zh?"JetStream 存储（字节）":"JetStream storage bytes"],["streams",zh?"Stream 数":"Streams"],["consumers",zh?"Consumer 数":"Consumers"],["messages",zh?"存储消息数":"Stored messages"],["meta_cluster_size",zh?"元数据集群规模":"Metadata cluster size"],["meta_pending",zh?"元数据待处理数":"Metadata pending"]].map(([key,label])=><React.Fragment key={key}><dt>{label}</dt><dd>{nodeJSMetric(selected,key)??unknown}</dd></React.Fragment>)}</dl>
        {nodeSourceAvailable(selected,"jsz")&&<details><summary>{zh?"JetStream 原始观测":"Raw JetStream observation"}</summary><pre className="declaration-json">{stringifyJSON(selected.jetstream)}</pre></details>}
        {selected.errors?.map((error,index)=><p role="alert" key={index}>{error}</p>)}
      </>}
    </>}
  </section>;
}
