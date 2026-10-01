import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createAuditList} from "./audit-list.mjs";
import {auditFields,auditURL} from "./audit-query.mjs";
import {stringifyJSON} from "./api.mjs";
import {createAuditExport} from "./audit-export.mjs";
import {auditLabels} from "./audit-labels.mjs";
import {runEventInvalidations} from "./event-invalidations.mjs";

export function Audit({api,language,route,router}) {
  const model=useMemo(()=>createAuditList(api),[api]),accessible=auditLabels(language);
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
  useEffect(()=>{const controller=new AbortController();void runEventInvalidations(api,{signal:controller.signal,onInvalidate:resource=>resource==="audit"?model.load(route.query):undefined});return()=>controller.abort();},[api,model,route]);
  const labels=accessible.fields;
  function navigate(query){try{setValidation(false);if(!router.navigate(auditURL(query)))void model.load(query);}catch{setValidation(true);}}
  const page=state.page;
  return <section aria-label={accessible.events}><h2>{accessible.title}</h2>
    <p>{accessible.description}</p>
    <form className="filter-toolbar" onSubmit={event=>{event.preventDefault();navigate({...form,before:null});}}>
      <p>{accessible.timeHelp}</p>
      {auditFields.map(key=><React.Fragment key={key}><label htmlFor={`audit-${key}`}>{labels[key]}</label>{key==="phase"?<select id={`audit-${key}`} value={form[key]} onChange={event=>setForm({...form,[key]:event.target.value})}><option value="">{accessible.all}</option><option value="intent">intent</option><option value="outcome">outcome</option></select>:<input id={`audit-${key}`} value={form[key]} onChange={event=>setForm({...form,[key]:event.target.value})}/>}</React.Fragment>)}
      <button type="submit">{accessible.search}</button>
      {validation&&<p role="alert">{accessible.invalidFilters}</p>}
    </form>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load(route.query)}>{accessible.reload}</button>
    <button onClick={()=>navigate({...route.query,before:null})}>{accessible.restart}</button>
    <section aria-label={accessible.export}><h3>{accessible.exportTitle}</h3>
      <p>{accessible.exportHelp}</p>
      <button disabled={exported.phase==="loading"} onClick={()=>void exporter.start(route.query)}>{accessible.prepareExport}</button>
      {exported.phase==="loading"&&<button onClick={()=>exporter.cancel()}>{accessible.cancelExport}</button>}
      {exported.phase!=="idle"&&<p role="status">{accessible.progress}: {exported.windows} / {exported.events}</p>}
      {exported.failure&&<p role="alert">{exported.failure==="limit"?accessible.exportLimit:accessible.exportFailure}</p>}
      {exported.phase==="ready"&&download&&<a href={download} download="rjs-audit-evidence.json">{accessible.download}</a>}
    </section>
    {state.phase==="loading"&&<p role="status">{accessible.loading}</p>}
    {state.failure&&<p role="alert">{state.failure==="denied"?accessible.denied:accessible.queryFailure}</p>}
    {page&&<>
      <p>{accessible.windowCounts}: {page.scanned} / {page.missing} / {page.items.length} · <time dateTime={state.readAt}>{state.readAt}</time></p>
      <p>{accessible.sequenceRange}: {String(page.firstSequence)}–{String(page.lastSequence)}</p>
      {!page.streamPresent&&<p>{accessible.noStorage}</p>}
      {page.items.length===0?<p>{accessible.noMatches}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={accessible.window}><table><caption className="visually-hidden">{accessible.caption}</caption><thead><tr><th>{accessible.sequence}</th><th>{accessible.time}</th><th>{labels.resource}</th><th>{labels.action}</th><th>{labels.phase}</th><th>{accessible.details}</th></tr></thead><tbody>{page.items.map(event=><tr key={String(event.sequence)}><th scope="row">{String(event.sequence)}</th><td>{event.time??"—"}</td><td>{event.resourceName??"—"}</td><td>{event.action??"—"}</td><td>{event.phase??"—"}</td><td><details><summary>{event.id}</summary><pre className="declaration-json">{stringifyJSON(event)}</pre></details></td></tr>)}</tbody></table></div>}
      <button disabled={page.nextBefore===null} onClick={()=>navigate({...route.query,before:String(page.nextBefore)})}>{accessible.older}</button>
      {page.nextBefore===null&&<p>{accessible.lowerBoundary}</p>}
    </>}
  </section>;
}
