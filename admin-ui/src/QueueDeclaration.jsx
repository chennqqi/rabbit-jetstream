import React, {useEffect, useMemo, useSyncExternalStore} from "react";
import {stringifyJSON} from "./api.mjs";
import {QueueConsumers} from "./QueueConsumers.jsx";
import {routingLabels} from "./routing-labels.mjs";
import {QueueSummary,QueueEvents} from "./QueuePanels.jsx";
import {queueTabs,queueTabURL} from "./routes.mjs";
import {createQueueDeclaration} from "./queue-declaration.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {DLQDiagnostics} from "./DLQDiagnostics.jsx";
import {RoutingProbe} from "./RoutingProbe.jsx";
import {QueueExport} from "./QueueExport.jsx";
import {MetricHistory} from "./MetricHistory.jsx";

// Declaration view is not an observed topology/health summary.
export function QueueDeclaration({api, name, language, route, router, canPreview, canAudit, canDelete, refreshSeconds=10}) {
  const model=useMemo(()=>createQueueDeclaration(api,name),[api,name]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const zh = language === "zh";
  const tab=route.tab??(route.consumerQuery?"consumers":"summary");
  const labels=zh?["摘要","配置","路由","消费者","事件"]:["Summary","Configuration","Routing","Consumers","Events"],routing=routingLabels(language);
  return <section aria-labelledby="queue-heading" className="queue-list queue-detail">
    <p><a href="/admin/queues" onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate("/admin/queues");}}}>{zh?"所有 Queue":"All Queues"}</a> / {name}</p>
    <header className="queue-resource-header"><h2 id="queue-heading">Queue: {name}</h2>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{zh?"刷新 Queue 页面":"Refresh Queue page"}</button>
    {canPreview && <p><a href={`/admin/queues/by-name/${encodeURIComponent(name)}/edit`} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(event.currentTarget.getAttribute("href"));}}}>{zh?"编辑草稿与预览":"Edit draft and preview"}</a></p>}
    {canDelete&&<p><a href={`/admin/queues/by-name/${encodeURIComponent(name)}/delete`} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(event.currentTarget.getAttribute("href"));}}}>{zh?"审阅删除影响":"Review deletion impact"}</a></p>}
    </header>
    {state.phase==="ready"&&<dl className="declaration-meta"><dt>{zh ? "Plan 版本" : "Plan revision"}</dt><dd>{state.body.revision}</dd><dt>{zh ? "声明 ETag（原值）" : "Declaration ETag (original)"}</dt><dd>{state.etag??(zh?"未知":"Unknown")}</dd><dt>{zh?"声明读取完成":"Declaration read completed"}</dt><dd><ReadTime value={state.readAt} language={language}/></dd></dl>}
    <details className="reading-help"><summary>{zh?"只读刷新与返回说明":"Read-only refresh and navigation guide"}</summary><p>{zh?"整页刷新重新读取声明及当前面板；不修改服务端或编辑草稿，也不是原子快照。返回所有 Queue 会打开未筛选列表；浏览器后退可恢复原列表查询。":"Page refresh rereads the declaration and active panel; it changes neither server state nor editor drafts and is not an atomic snapshot. All Queues opens the unfiltered list; browser Back restores the prior list query."}</p></details>
    <nav aria-label={zh?"Queue 详情标签":"Queue detail tabs"}>{queueTabs.map((key,index)=><a key={key} aria-current={tab===key?"page":undefined} href={queueTabURL(name,key,route.consumerQuery,route.eventBefore)} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();router.navigate(queueTabURL(name,key,route.consumerQuery,route.eventBefore));}}}>{labels[index]} </a>)}</nav>
    {state.phase === "loading" && <p role="status">{zh ? "正在读取…" : "Loading…"}</p>}
    {state.phase === "error" && <p role="alert">{state.status === 404 && state.code === "not_found" ? (zh ? "Queue 声明不存在。" : "Queue declaration not found.") : state.status === 401 || state.status === 403 ? (zh ? "读取被拒绝，请检查凭据与权限。" : "Read denied. Check credentials and permissions.") : (zh ? "声明读取不可用，不代表资源为空。" : "Declaration read unavailable; this is not an empty resource.")}</p>}
    {state.phase === "ready" && <>
      {tab==="summary"&&<><QueueSummary api={api} plan={state.body.plan} language={language} router={router} route={route} refreshSeconds={refreshSeconds}/><MetricHistory api={api} language={language} queue={name} metrics={[{id:"queue-messages",en:"Stored Queue messages",zh:"Queue 存储消息"},{id:"queue-bytes",en:"Stored Queue bytes",zh:"Queue 存储字节"}]}/></>}
      {tab==="routing"&&<p>{zh?"声明输入 Subjects（与优先级存储 Subjects 区分）":"Declared input Subjects (distinct from priority storage Subjects)"}: {(state.body.plan.declarationSubjects??state.body.document?.spec?.subjects)?.join(", ")??(zh?"未知":"Unknown")}</p>}
      {tab==="configuration"&&<><h3>{zh?"声明配置（不是观测状态）":"Declared configuration (not observed state)"}</h3>{state.body.document?<pre className="declaration-json">{stringifyJSON(state.body.document)}</pre>:<p>{zh?"此声明无法无损转换为可编辑文档。":"This declaration cannot be losslessly converted to an editable document."}</p>}<details><summary>{zh?"声明 Plan（原始数据）":"Declared Plan (raw data)"}</summary><pre className="declaration-json">{stringifyJSON(state.body.plan)}</pre></details></>}
      {tab==="routing"&&<><h3>{zh?"声明路由映射":"Declared routing map"}</h3><p>{zh?"只读 Plan 映射，不是实际投递记录。":"Read-only Plan mapping, not actual delivery records."}</p>{state.body.plan.routing?.length?<div className="table-scroll"><table><thead><tr>{[routing.exchange,routing.type,routing.keys,routing.subjects].map(label=><th key={label}>{label}</th>)}</tr></thead><tbody>{state.body.plan.routing.map((row,index)=><tr key={index}><td>{row.exchange}</td><td>{row.type}</td><td>{row.keys?.join(", ")}</td><td>{row.subjects?.join(", ")}</td></tr>)}</tbody></table></div>:<p>{zh?"无声明的 Exchange 绑定；Stream Subjects 如下：":"No declared Exchange bindings; Stream Subjects:"} {state.body.plan.stream.subjects?.join(", ")}</p>}</>}
      {tab==="consumers"&&<QueueConsumers api={api} queue={name} language={language} declarationETag={state.etag} route={route} router={router} refreshSeconds={refreshSeconds} />}
      {tab==="routing"&&<RoutingProbe api={api} name={name} revision={state.body.revision} etag={state.etag} language={language}/>}
      {tab==="configuration"&&<DLQDiagnostics api={api} plan={state.body.plan} declarationETag={state.etag} declarationReadAt={state.readAt} language={language} router={router}/>}
      {tab==="configuration"&&<QueueExport api={api} name={name} document={state.body.document} etag={state.etag} language={language}/>}
      {tab==="events"&&(canAudit?<QueueEvents api={api} name={name} language={language} router={router} route={route}/>:<p role="alert">{zh?"当前身份没有审计读取权限。":"This identity has no audit read permission."}</p>)}
    </>}
  </section>;
}
