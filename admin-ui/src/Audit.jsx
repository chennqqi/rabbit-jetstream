import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createAuditList} from "./audit-list.mjs";
import {auditFields,auditURL} from "./audit-query.mjs";
import {stringifyJSON} from "./api.mjs";
import {createAuditExport} from "./audit-export.mjs";
import {auditLabels} from "./audit-labels.mjs";

export function Audit({api,language,route,router}) {
  const zh=language==="zh",model=useMemo(()=>createAuditList(api),[api]),accessible=auditLabels(language);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  const exporter=useMemo(()=>createAuditExport(api),[api]);
  const exported=useSyncExternalStore(exporter.subscribe,exporter.snapshot);
  const [download,setDownload]=useState(null);
  useEffect(()=>{return()=>exporter.cancel();},[exporter,route]);
  useEffect(()=>{
    if(!exported.content){setDownload(null);return;}
    const url=URL.createObjectURL(new Blob([exported.content],{type:"application/json"}));setDownload(url);
    return()=>URL.revokeObjectURL(url);
  },[exported.content]);
  const [form,setForm]=useState(route.query),[validation,setValidation]=useState(false);
  useEffect(()=>{setForm(route.query);setValidation(false);void model.load(route.query);return()=>model.clear();},[model,route]);
  const labels=zh?{requestId:"请求 ID",resource:"资源名称",actor:"操作者",phase:"阶段",action:"操作",outcome:"记录结果"}:{requestId:"Request ID",resource:"Resource name",actor:"Actor",phase:"Phase",action:"Action",outcome:"Recorded outcome"};
  labels.from=zh?"起始时间（包含）":"From (inclusive)";labels.until=zh?"截止时间（不包含）":"Until (exclusive)";
  function navigate(query){try{setValidation(false);if(!router.navigate(auditURL(query)))void model.load(query);}catch{setValidation(true);}}
  const page=state.page;
  return <section aria-label={accessible.events}><h2>{zh?"审计":"Audit"}</h2>
    <p>{zh?"所有条件精确匹配且同时满足。每窗口最多扫描 256 个序号，不提供匹配总数。审计记录不是操作结果的自动归因，也不代表资源已收敛。":"All filters are exact and combined. Each window scans at most 256 sequence positions; no matching total is available. Audit records are not automatic outcome attribution or convergence evidence."}</p>
    <form onSubmit={event=>{event.preventDefault();navigate({...form,before:null});}}>
      <p>{zh?"时间使用带时区的 RFC3339 格式，例如 2026-09-10T08:00:00+08:00；最多支持九位小数。留空表示不限制该边界。":"Times use RFC3339 with a timezone, e.g. 2026-09-10T08:00:00+08:00; up to nine fractional digits. Blank means no bound."}</p>
      {auditFields.map(key=><React.Fragment key={key}><label htmlFor={`audit-${key}`}>{labels[key]}</label>{key==="phase"?<select id={`audit-${key}`} value={form[key]} onChange={event=>setForm({...form,[key]:event.target.value})}><option value="">{zh?"全部":"All"}</option><option value="intent">intent</option><option value="outcome">outcome</option></select>:<input id={`audit-${key}`} value={form[key]} onChange={event=>setForm({...form,[key]:event.target.value})}/>}</React.Fragment>)}
      <button type="submit">{zh?"查询审计":"Search audit"}</button>
      {validation&&<p role="alert">{zh?"筛选条件无效；每项最多 256 个 UTF-8 字节。":"Invalid filters; each allows at most 256 UTF-8 bytes."}</p>}
    </form>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load(route.query)}>{zh?"重读当前窗口":"Reload current window"}</button>
    <button onClick={()=>navigate({...route.query,before:null})}>{zh?"从最新记录重新开始":"Restart from newest"}</button>
    <section aria-label={accessible.export}><h3>{zh?"导出审计证据":"Export audit evidence"}</h3>
      <p>{zh?"使用已提交的筛选条件，从最新保留记录开始（忽略当前页游标），不会自动应用未提交输入。JSON 包含每窗口读取时间和缺口，不是完整历史快照。上限为 256 个窗口或 16 MiB；失败/超限不提供截断文件。下载后文件不受清除会话影响，请妥善保管。":"Uses submitted filters from the newest retained records, ignoring the current page cursor and unapplied form edits. JSON includes per-window timestamps and gaps, not a full historical snapshot. Limit: 256 windows or 16 MiB; errors/limits produce no truncated download. Clearing the session does not remove downloaded files; store them securely."}</p>
      <button disabled={exported.phase==="loading"} onClick={()=>void exporter.start(route.query)}>{zh?"准备筛选结果导出":"Prepare filtered export"}</button>
      {exported.phase==="loading"&&<button onClick={()=>exporter.cancel()}>{zh?"取消导出":"Cancel export"}</button>}
      {exported.phase!=="idle"&&<p role="status">{zh?"已读取窗口/事件":"Read windows / events"}: {exported.windows} / {exported.events}</p>}
      {exported.failure&&<p role="alert">{exported.failure==="limit"?(zh?"达到浏览器导出安全上限，未生成文件。大规模导出仍需专用工具。":"Browser export safety limit reached; no file generated. Larger exports still require dedicated tooling."):(zh?"导出读取失败，未生成文件。请检查服务或权限后重新开始。":"Export read failed; no file generated. Check service/access, then restart explicitly.")}</p>}
      {exported.phase==="ready"&&download&&<a href={download} download="rjs-audit-evidence.json">{zh?"下载审计 JSON":"Download audit JSON"}</a>}
    </section>
    {state.phase==="loading"&&<p role="status">{zh?"正在读取审计…":"Loading audit…"}</p>}
    {state.failure&&<p role="alert">{state.failure==="denied"?(zh?"审计读取被拒绝。":"Audit read denied."):(zh?"审计查询失败或响应无效，不代表没有记录。":"Audit query failed or returned invalid data; this does not mean no records exist.")}</p>}
    {page&&<>
      <p>{zh?"窗口扫描/缺失/匹配":"Window scanned / missing / matched"}: {page.scanned} / {page.missing} / {page.items.length} · <time dateTime={state.readAt}>{state.readAt}</time></p>
      <p>{zh?"观测保留序号范围":"Observed retained sequence range"}: {String(page.firstSequence)}–{String(page.lastSequence)}</p>
      {!page.streamPresent&&<p>{zh?"未发现保留审计存储；不能证明从未执行操作。":"No retained audit storage observed; this does not prove no operation occurred."}</p>}
      {page.items.length===0?<p>{zh?"当前窗口没有匹配记录。":"No matching records in this window."}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={accessible.window}><table><thead><tr><th>{zh?"序号":"Sequence"}</th><th>{zh?"时间":"Time"}</th><th>{labels.resource}</th><th>{labels.action}</th><th>{labels.phase}</th><th>{zh?"详情":"Details"}</th></tr></thead><tbody>{page.items.map(event=><tr key={String(event.sequence)}><th scope="row">{String(event.sequence)}</th><td>{event.time??"—"}</td><td>{event.resourceName??"—"}</td><td>{event.action??"—"}</td><td>{event.phase??"—"}</td><td><details><summary>{event.id}</summary><pre className="declaration-json">{stringifyJSON(event)}</pre></details></td></tr>)}</tbody></table></div>}
      <button disabled={page.nextBefore===null} onClick={()=>navigate({...route.query,before:String(page.nextBefore)})}>{zh?"读取更早窗口":"Read older window"}</button>
      {page.nextBefore===null&&<p>{zh?"已到本次观测的保留下界，不证明过期或未来事件不存在。":"Reached the observed retained lower boundary; expired or future events may still exist."}</p>}
    </>}
  </section>;
}
