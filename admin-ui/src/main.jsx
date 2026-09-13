import React, {useEffect, useRef, useState, useSyncExternalStore} from "react";
import {createRoot} from "react-dom/client";
import {createAPI, stringifyJSON} from "./api.mjs";
import {createSession} from "./session.mjs";
import {createBrowserOIDC} from "./browser-oidc.mjs";
import {QueueList} from "./QueueList.jsx";
import {QueueDeclaration} from "./QueueDeclaration.jsx";
import {ConsumerDetail} from "./ConsumerDetail.jsx";
import {StreamDetail} from "./StreamDetail.jsx";
import {GlobalConsumers} from "./GlobalConsumers.jsx";
import {Nodes} from "./Nodes.jsx";
import {Connections} from "./Connections.jsx";
import {ConnectionDetail} from "./ConnectionDetail.jsx";
import {Overview} from "./Overview.jsx";
import {Audit} from "./Audit.jsx";
import {QueueEditor} from "./QueueEditor.jsx";
import {QueueDelete} from "./QueueDelete.jsx";
import {DeleteEvidence} from "./DeleteEvidence.jsx";
import {createQueueDelete} from "./queue-delete.mjs";
import {archiveQueueEditors,queueEditors} from "./editor-handoff.mjs";
import {EditorHandoff,ArchivedEditors} from "./EditorHandoff.jsx";
import {QueueEvidence} from "./QueueEvidence.jsx";
import {QueueCreate} from "./QueueCreate.jsx";
import {BatchHistory} from "./BatchHistory.jsx";
import {BulkChange} from "./BulkChange.jsx";
import {emptyCreationForm,retainCreationDraft} from "./queue-create.mjs";
import {createQueueDraft} from "./queue-draft.mjs";
import {createRouter} from "./routes.mjs";
import {ConsoleNavigation} from "./ConsoleNavigation.jsx";
import {SessionIdentity} from "./SessionIdentity.jsx";
import {Settings} from "./Settings.jsx";
import {Compatibility} from "./Compatibility.jsx";
import {Diagnostics} from "./Diagnostics.jsx";
import {Alerts} from "./Alerts.jsx";
import {AccessManagement} from "./AccessManagement.jsx";
import {readLanguage,saveLanguage} from "./language.mjs";
import {readRefreshPreference,saveRefreshPreference,validRefreshPreference} from "./refresh-preference.mjs";
import {consolePageTitle} from "./page-title.mjs";
import {hasAuthenticatedConsole} from "./console-state.mjs";
import {loginMessages} from "./login-messages.mjs";
import {useDialog,DialogHost} from "./dialog.jsx";
import {EvidenceIndicator} from "./evidence-indicator.jsx";
import "./shell.css";

const api = createAPI({requireMutationCapabilities:true});
const session = createSession(api);
const browserOIDC = createBrowserOIDC(api,session);
const router = createRouter();
const initialLanguage=readLanguage(navigator.language);
const messages = loginMessages;

function RetainedDraft({model, name, language}) {
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  return <details><summary>{messages[language].retainedDraft}: {name}</summary>
    <pre className="declaration-json">{stringifyJSON({phase: state.phase, raw: state.raw, mergeRaw: state.mergeRaw, requestId: state.requestId ?? null, submittedPlan: state.submittedPlan ?? null, inspection: state.inspection ?? null, acceptedOperations: state.acceptedOperations ?? []})}</pre>
    {state.draft&&<QueueEvidence state={state} language={language}/>}
  </details>;
}

function RetainedDeletion({model,language}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  return <details><summary>{messages[language].deletionEvidence}: {state.name}</summary><pre className="declaration-json">{stringifyJSON(state)}</pre><DeleteEvidence state={state} language={language}/></details>;
}

export function App() {
  const drafts=useRef(new Map());
  const deletions=useRef(new Map());
  const archives=useRef(new Map());
  const usernameInput=useRef(null),hadIdentity=useRef(false),hadAuthenticatedConsole=useRef(false);
  const [,refreshHandoff]=useState(0);
  const dialog=useDialog();
  const deleting=name=>deletions.current.has(name)&&deletions.current.get(name).snapshot().phase!=="idle";
  const editing=name=>drafts.current.has(name)||drafts.current.has(`create:${name}`);
  const creation=useRef({model:null,form:emptyCreationForm(),completed:[]});
  const dirty=()=>!!creation.current.batch||!!creation.current.batchHistory?.length||archives.current.size>0||[...deletions.current.values()].some(model=>model.snapshot().phase!=="idle")||[...drafts.current.values()].some(model=>model.snapshot().modified)||(!creation.current.model&&Object.values(creation.current.form).some(Boolean));
  useEffect(()=>{
    const leaving=event=>{if(dirty()){event.preventDefault();event.returnValue="";}};
    window.addEventListener("beforeunload",leaving);return()=>window.removeEventListener("beforeunload",leaving);
  },[]);
  function retainedStates(){return [...drafts.current.values(),...deletions.current.values()].map(model=>model.snapshot());}
  function discardRetained(){
    for(const model of drafts.current.values())model.discard();drafts.current.clear();
    for(const model of deletions.current.values())model.discard();deletions.current.clear();
    archives.current.clear();creation.current={model:null,form:emptyCreationForm(),completed:[]};
  }
  async function clearSession(){
    const states=retainedStates();
    if(states.some(state=>state.phase==="submitting")){await dialog.alert(text.clearSubmitting);return false;}
    const unresolved=states.some(state=>["uncertain","inspecting"].includes(state.phase));
    if(dirty()||unresolved){
      const message=unresolved
        ?text.clearUnknown
        :text.clearDirty;
      if(!await dialog.confirm(message,{tone:"danger",confirmLabel:text.clearConfirm}))return false;
    }
    discardRetained();session.clear();return true;
  }
  async function switchTenant(tenant,{syncURL=true}={}){
    const states=retainedStates();
    if(states.some(state=>state.phase==="submitting")){await dialog.alert(text.switchSubmitting);return false;}
    const unresolved=states.some(state=>["uncertain","inspecting"].includes(state.phase));
    if(dirty()||unresolved){
      if(!await dialog.confirm(text.switchDirty,{tone:"danger",confirmLabel:text.switchConfirm}))return false;
    }
    discardRetained();session.selectTenant(tenant);if(syncURL)router.setTenant(tenant);return true;
  }
  const [language, setLanguage] = useState(initialLanguage);
  const [languageSaved,setLanguageSaved]=useState(true);
  const [refreshSeconds,setRefreshSeconds]=useState(readRefreshPreference);
  const [refreshSaved,setRefreshSaved]=useState(true);
  const [oidcAvailable,setOIDCAvailable]=useState(false),[oidcFailure,setOIDCFailure]=useState(false);
  useEffect(()=>{let active=true;(async()=>{try{if(await browserOIDC.complete())return;const config=await browserOIDC.load();if(active)setOIDCAvailable(!!config);}catch{if(active)setOIDCFailure(true);}})();return()=>{active=false;};},[]);
  function changeRefresh(value){if(!validRefreshPreference(value))return;setRefreshSeconds(value);setRefreshSaved(saveRefreshPreference(value));}
  const state = useSyncExternalStore(session.subscribe, session.snapshot);
  useEffect(()=>{if(hadIdentity.current&&!state.identity)usernameInput.current?.focus();hadIdentity.current=!!state.identity;},[state.identity]);
  const route = useSyncExternalStore(router.subscribe, router.snapshot);
  useEffect(()=>{
    if(state.phase!=="authenticated"||!state.identity?.active_tenant)return;
    const active=state.identity.active_tenant,wanted=route.tenant;
    if(!wanted){router.setTenant(active);return;}
    if(wanted===active)return;
    if(!state.identity.tenants.includes(wanted)){router.setTenant(active);return;}
    // A clean console switches synchronously; with retained work the URL is
    // reverted immediately and the switch proceeds only after explicit
    // confirmation, so a stale URL can never discard drafts silently.
    const states=retainedStates();
    const blocked=states.some(state=>["submitting","uncertain","inspecting"].includes(state.phase));
    if(blocked||dirty()){router.setTenant(active);void (async()=>{if(await switchTenant(wanted,{syncURL:false}))router.setTenant(wanted);})();return;}
    void switchTenant(wanted,{syncURL:false});
  },[state.phase,state.identity?.active_tenant,route.tenant]);
  const authenticatedConsole=hasAuthenticatedConsole(state.phase,state.identity);
  const tenantReady=!state.identity?.active_tenant||route.tenant===state.identity.active_tenant;
  useEffect(()=>{if(!hadAuthenticatedConsole.current&&authenticatedConsole)document.getElementById("console-content")?.focus();hadAuthenticatedConsole.current=authenticatedConsole;},[authenticatedConsole]);
  const pageTitle=consolePageTitle({phase:state.phase,authenticated:authenticatedConsole,route,language});
  useEffect(()=>{document.title=pageTitle;},[pageTitle]);
  const text = messages[language];
  const busy = state.phase === "verifying";
  function login(event) {
    event.preventDefault();
    const form = event.currentTarget, data = new FormData(form);
    const username = data.get("username"), password = data.get("password");
    form.reset();
    void session.signInWithPassword(username, password);
  }
  function signIn(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const token = new FormData(form).get("token");
    form.reset(); // Do not retain a second copy in React state or the input.
    void session.signIn(token);
  }
  function changeLanguage() {
    const next = language === "en" ? "zh" : "en";
    document.documentElement.lang = next === "zh" ? "zh-CN" : "en";
    setLanguage(next);
    setLanguageSaved(saveLanguage(next));
  }
  return <main className={`session-shell ${state.identity?"console-shell":"login-shell"}`}>
    {authenticatedConsole&&tenantReady&&<button type="button" className="skip-navigation" onClick={()=>document.getElementById("console-content")?.focus()}>{text.skipContent}</button>}
    {authenticatedConsole&&tenantReady&&<p className="visually-hidden route-announcement" role="status" aria-live="polite" aria-atomic="true">{pageTitle}</p>}
    {state.identity&&<h1 className="visually-hidden">{text.title}</h1>}
    <header className="console-topbar"><span className="brand"><strong>RJS</strong><span>Rabbit JetStream<small>{text.title}</small></span></span>
      {state.identity && <SessionIdentity identity={state.identity} text={text} onClear={()=>void clearSession()} onTenantChange={tenant=>void switchTenant(tenant)}/>}
      {authenticatedConsole&&<EvidenceIndicator drafts={drafts.current} deletions={deletions.current} router={router} language={language}/>}
      {state.identity?.actor.startsWith("oidc:")&&browserOIDC.config?.logoutEndpoint&&<button type="button" onClick={()=>{void clearSession().then(cleared=>{if(cleared)browserOIDC.logout();});}}>{text.ssoLogout}</button>}
      <button onClick={changeLanguage}>{text.switchLanguage}</button></header>
    {!state.identity&&<h1>{text.title}</h1>}<p className="candidate">{text.candidate}</p>
    {!languageSaved&&<p role="status">{text.storageUnavailable}</p>}
    {!state.identity && <form className="local-login" onSubmit={login} aria-busy={busy}>
      <label htmlFor="username">{text.username}</label>
      <input ref={usernameInput} id="username" name="username" autoComplete="username" spellCheck="false" required disabled={busy}/>
      <label htmlFor="password">{text.password}</label>
      <input id="password" name="password" type="password" autoComplete="current-password" required disabled={busy} aria-describedby="privacy"/>
      <button type="submit" disabled={busy}>{busy ? text.verifying : text.login}</button>
      {busy && <button type="button" onClick={() => session.clear()}>{text.clear}</button>}
      {state.failure && <p role="alert">{text.errors[state.failure.kind]}</p>}
    </form>}
    {!state.identity&&<details className="recovery-login"><summary>{text.recovery}</summary><form onSubmit={signIn} aria-busy={busy}>
      <label htmlFor="token">{text.token}</label>
      <input id="token" name="token" type="password" autoComplete="off" spellCheck="false" required disabled={busy} aria-describedby="privacy" />
      <button type="submit" disabled={busy}>{busy ? text.verifying : text.signIn}</button>
    </form></details>}
    {!state.identity&&oidcAvailable&&<button type="button" disabled={busy} onClick={()=>{setOIDCFailure(false);void browserOIDC.start().catch(()=>setOIDCFailure(true));}}>{text.ssoSignIn}</button>}
    {!state.identity&&oidcFailure&&<p role="alert">{text.ssoFailure}</p>}
    {state.phase === "expired" && <p role="alert">{text.expiredEvidence}</p>}
    {state.phase === "expired" && <section>{[...drafts.current].map(([name,model]) => <RetainedDraft key={name} name={name} model={model} language={language} />)}{!creation.current.model && <pre className="declaration-json">{stringifyJSON(creation.current.form)}</pre>}</section>}
    {state.phase==="expired"&&<section>{[...deletions.current].map(([name,model])=><RetainedDeletion key={name} model={model} language={language}/>)}</section>}
    {state.phase==="expired"&&<BatchHistory records={creation.current.batchHistory} language={language}/>}
    {state.phase==="expired"&&[...archives.current].map(([name,records])=><ArchivedEditors key={name} records={records} language={language}/>)}
    {authenticatedConsole && tenantReady && <>
      <ConsoleNavigation language={language} route={route} router={router} permissions={state.identity.permissions}/>
      <div id="console-content" key={state.identity.active_tenant??"legacy"} tabIndex="-1">
      {route.name&&["queue","edit-queue","delete-queue"].includes(route.kind)&&<ArchivedEditors records={archives.current.get(route.name)} language={language}/>}
      {route.kind === "queues" ? <QueueList key={state.identity.actor} api={api} language={language} route={route} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind === "compatibility" ? <Compatibility api={api} language={language}/> :
        route.kind === "diagnostics" && state.identity.permissions.includes("diagnostics:create") ? <Diagnostics api={api} language={language}/> :
        route.kind === "alerts" ? <Alerts api={api} language={language}/> :
        route.kind === "consumers" ? <GlobalConsumers api={api} language={language} route={route} router={router} canRefresh={state.identity.permissions.includes("queue:apply")}/> :
        route.kind === "streams" ? <QueueList key="streams" resource="streams" api={api} language={language} route={route} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind==="node-connection" ? <ConnectionDetail api={api} language={language} route={route} router={router} refreshSeconds={refreshSeconds}/> :
        route.kind==="node-connections" ? <Connections api={api} language={language} route={route} router={router} refreshSeconds={refreshSeconds}/> :
        ["nodes","node"].includes(route.kind) ? <Nodes api={api} language={language} route={route} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind === "overview" ? <Overview api={api} language={language} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind === "settings" ? <Settings api={api} identity={state.identity} language={language} onLanguage={changeLanguage} onClear={clearSession} refreshSeconds={refreshSeconds} onRefresh={changeRefresh} refreshSaved={refreshSaved}/> :
        route.kind === "access" && state.identity.permissions.includes("access:manage") ? <AccessManagement api={api} language={language} actor={state.identity.actor}/> :
        route.kind === "audit" && state.identity.permissions.includes("audit:read") ? <Audit api={api} language={language} route={route} router={router} /> :
        route.kind === "stream" ? <StreamDetail key={route.name} api={api} name={route.name} language={language} route={route} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind === "queue" ? <QueueDeclaration key={route.name} api={api} name={route.name} language={language} route={route} router={router} canPreview={state.identity.permissions.includes("queue:preview")} canAudit={state.identity.permissions.includes("audit:read")} canDelete={state.identity.permissions.includes("queue:delete")} refreshSeconds={refreshSeconds} /> :
        route.kind === "delete-queue" && state.identity.permissions.includes("queue:delete") ? <><EditorHandoff entries={queueEditors(drafts.current,route.name)} language={language} onArchive={()=>{if(archiveQueueEditors({drafts:drafts.current,archives:archives.current,creation:creation.current,name:route.name,confirmed:true}))refreshHandoff(value=>value+1);else void dialog.alert(text.handoffBlocked);}}/><QueueDelete blocked={editing(route.name)} model={(()=>{if(!deletions.current.has(route.name))deletions.current.set(route.name,createQueueDelete(api,route.name,{canStart:()=>!editing(route.name)}));return deletions.current.get(route.name);})()} language={language} router={router}/></> :
        route.kind === "edit-queue" && state.identity.permissions.includes("queue:preview") ? deleting(route.name)?<p role="alert">{text.deletionOwnsQueue}</p>:<QueueEditor model={(()=>{if(!drafts.current.has(route.name))drafts.current.set(route.name,createQueueDraft(api,route.name));return drafts.current.get(route.name);})()} language={language} router={router} /> :
        route.kind === "bulk-change" && state.identity.permissions.includes("queue:apply") ? <BulkChange api={api} language={language} prepare={async(document,etag)=>{const name=document.metadata.name;if(deleting(name)||editing(name))throw Object.assign(Error("Retained operation owns Queue"),{code:"retained"});const model=createQueueDraft(api,name);drafts.current.set(name,model);await model.load();const loaded=model.snapshot();if(loaded.phase!=="editing"||loaded.etag!==etag){model.archive();drafts.current.delete(name);throw Object.assign(Error("Queue changed after batch preview"),{code:loaded.phase==="editing"?"changed":"unavailable"});}model.edit(stringifyJSON(document));return model;}}/> :
        route.kind === "create-queue" && state.identity.permissions.includes("queue:apply") ? <QueueCreate api={api} setup={creation.current} language={language} prepare={document=>{if(deleting(document.metadata.name))throw Object.assign(new Error("Deletion retained"),{code:"retained-name"});return retainCreationDraft(drafts.current,document,value=>createQueueDraft(api,value.metadata.name,{document:value}));}} /> :
        route.kind === "consumer" ? <ConsumerDetail key={`${route.stream}/${route.name}`} api={api} stream={route.stream} name={route.name} language={language} router={router} refreshSeconds={refreshSeconds} /> :
        <p role="alert">{text.unavailablePage}</p>}
      </div>
    </>}
    <DialogHost dialog={dialog.dialog} language={language}/>
    <p id="privacy">{text.privacy}</p>
  </main>;
}

  document.documentElement.lang = Object.freeze({en:"en",zh:"zh-CN"})[initialLanguage]??"en";
export function mountApp(element = document.getElementById("root")) {
  if (!element) throw new TypeError("Admin UI root element is missing");
  const root = createRoot(element);
  root.render(<App />);
  return root;
}

if (import.meta.env.MODE !== "test") mountApp();
