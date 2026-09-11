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
import {readLanguage,saveLanguage} from "./language.mjs";
import {readRefreshPreference,saveRefreshPreference,validRefreshPreference} from "./refresh-preference.mjs";
import {consolePageTitle} from "./page-title.mjs";
import {hasAuthenticatedConsole} from "./console-state.mjs";
import "./shell.css";

const api = createAPI({requireMutationCapabilities:true});
const session = createSession(api);
const browserOIDC = createBrowserOIDC(api,session);
const router = createRouter();
const initialLanguage=readLanguage(navigator.language);
const messages = {
  en: {
    title: "Management console", candidate: "Development candidate — production workflows are still being integrated.",
    token: "Bearer token", signIn: "Verify identity", verifying: "Verifying…", clear: "Clear local session",
    privacy: "Your token stays in memory. Reloading clears it. This does not revoke the server credential.",
    identity: "Verified identity", role: "Role", expires: "Verified expiry", unknown: "Unknown", policy: "Resource-read policy",
    errors: {"missing-token": "Enter a token.", "credentials-rejected": "Credentials were rejected; this does not necessarily mean expiry.", "role-denied": "This identity has no permitted management role.", "auth-disabled": "Server authentication is not configured.", expired: "The verified credential has expired.", "invalid-response": "The server returned an invalid identity response.", unavailable: "Identity verification is unavailable. Check the server and connection, then retry explicitly."},
  },
  zh: {
    title: "管理控制台", candidate: "开发候选版本 — 生产工作流仍在接入中。",
    token: "Bearer Token", signIn: "验证身份", verifying: "正在验证…", clear: "清除本机会话",
    privacy: "Token 仅保存在内存，刷新页面即清除。这不会撤销服务端凭据。",
    identity: "已验证身份", role: "角色", expires: "已验证到期时间", unknown: "未知", policy: "资源读取策略",
    errors: {"missing-token": "请输入 Token。", "credentials-rejected": "凭据被拒绝，不一定代表已过期。", "role-denied": "该身份没有允许的管理角色。", "auth-disabled": "服务端尚未配置认证。", expired: "已验证的凭据已过期。", "invalid-response": "服务端返回了无效的身份响应。", unavailable: "身份验证不可用，请检查服务端及连接后手动重试。"},
  },
};

function RetainedDraft({model, name, language}) {
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  return <details><summary>{language === "zh" ? "保留的草稿与请求证据" : "Retained draft and request evidence"}: {name}</summary>
    <pre className="declaration-json">{stringifyJSON({phase: state.phase, raw: state.raw, mergeRaw: state.mergeRaw, requestId: state.requestId ?? null, submittedPlan: state.submittedPlan ?? null, inspection: state.inspection ?? null, acceptedOperations: state.acceptedOperations ?? []})}</pre>
    {state.draft&&<QueueEvidence state={state} language={language}/>}
  </details>;
}

function RetainedDeletion({model,language}) {
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  return <details><summary>{language==="zh"?"删除证据":"Deletion evidence"}: {state.name}</summary><pre className="declaration-json">{stringifyJSON(state)}</pre><DeleteEvidence state={state} language={language}/></details>;
}

function App() {
  const drafts=useRef(new Map());
  const deletions=useRef(new Map());
  const archives=useRef(new Map());
  const tokenInput=useRef(null),hadIdentity=useRef(false),hadAuthenticatedConsole=useRef(false);
  const [,refreshHandoff]=useState(0);
  const deleting=name=>deletions.current.has(name)&&deletions.current.get(name).snapshot().phase!=="idle";
  const editing=name=>drafts.current.has(name)||drafts.current.has(`create:${name}`);
  const creation=useRef({model:null,form:emptyCreationForm(),completed:[]});
  const dirty=()=>!!creation.current.batch||!!creation.current.batchHistory?.length||archives.current.size>0||[...deletions.current.values()].some(model=>model.snapshot().phase!=="idle")||[...drafts.current.values()].some(model=>model.snapshot().modified)||(!creation.current.model&&Object.values(creation.current.form).some(Boolean));
  useEffect(()=>{
    const leaving=event=>{if(dirty()){event.preventDefault();event.returnValue="";}};
    window.addEventListener("beforeunload",leaving);return()=>window.removeEventListener("beforeunload",leaving);
  },[]);
  function clearSession(){
    const states=[...drafts.current.values(),...deletions.current.values()].map(model=>model.snapshot());
    if(states.some(state=>state.phase==="submitting")){window.alert(language==="zh"?"提交正在进行，暂不能清除会话。":"A submission is pending; session cannot be cleared yet.");return;}
    const unresolved=states.some(state=>["uncertain","inspecting"].includes(state.phase));
    if((dirty()||unresolved) && !window.confirm(unresolved ? (language==="zh"?"写入结果未知。清除会丢弃请求证据，不会取消或回滚服务端操作。请先保存请求标识，确认清除？":"Write outcome is unknown. Clearing discards request evidence and does not cancel or roll back the server operation. Record the request ID first. Clear anyway?") : (language==="zh"?"清除会话将丢弃所有草稿与保留的内存证据，确认继续？":"Clear session and discard all drafts and retained in-memory evidence?")))return;
    for(const model of drafts.current.values())model.discard();drafts.current.clear();
    for(const model of deletions.current.values())model.discard();deletions.current.clear();session.clear();
    archives.current.clear();creation.current={model:null,form:emptyCreationForm(),completed:[]};return true;
  }
  const [language, setLanguage] = useState(initialLanguage);
  const [languageSaved,setLanguageSaved]=useState(true);
  const [refreshSeconds,setRefreshSeconds]=useState(readRefreshPreference);
  const [refreshSaved,setRefreshSaved]=useState(true);
  const [oidcAvailable,setOIDCAvailable]=useState(false),[oidcFailure,setOIDCFailure]=useState(false);
  useEffect(()=>{let active=true;(async()=>{try{if(await browserOIDC.complete())return;const config=await browserOIDC.load();if(active)setOIDCAvailable(!!config);}catch{if(active)setOIDCFailure(true);}})();return()=>{active=false;};},[]);
  function changeRefresh(value){if(!validRefreshPreference(value))return;setRefreshSeconds(value);setRefreshSaved(saveRefreshPreference(value));}
  const state = useSyncExternalStore(session.subscribe, session.snapshot);
  useEffect(()=>{if(hadIdentity.current&&!state.identity)tokenInput.current?.focus();hadIdentity.current=!!state.identity;},[state.identity]);
  const route = useSyncExternalStore(router.subscribe, router.snapshot);
  const authenticatedConsole=hasAuthenticatedConsole(state.phase,state.identity);
  useEffect(()=>{if(!hadAuthenticatedConsole.current&&authenticatedConsole)document.getElementById("console-content")?.focus();hadAuthenticatedConsole.current=authenticatedConsole;},[authenticatedConsole]);
  const pageTitle=consolePageTitle({phase:state.phase,authenticated:authenticatedConsole,route,language});
  useEffect(()=>{document.title=pageTitle;},[pageTitle]);
  const text = messages[language];
  const busy = state.phase === "verifying";
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
    {authenticatedConsole&&<button type="button" className="skip-navigation" onClick={()=>document.getElementById("console-content")?.focus()}>{language==="zh"?"跳到页面内容":"Skip to page content"}</button>}
    {authenticatedConsole&&<p className="visually-hidden route-announcement" role="status" aria-live="polite" aria-atomic="true">{pageTitle}</p>}
    {state.identity&&<h1 className="visually-hidden">{text.title}</h1>}
    <header className="console-topbar"><span className="brand"><strong>RJS</strong><span>Rabbit JetStream<small>{text.title}</small></span></span>
      {state.identity && <SessionIdentity identity={state.identity} text={text} onClear={clearSession}/>}
      {state.identity?.actor.startsWith("oidc:")&&browserOIDC.config?.logoutEndpoint&&<button type="button" onClick={()=>{if(clearSession())browserOIDC.logout();}}>{language==="zh"?"退出企业 SSO":"Sign out from SSO"}</button>}
      <button onClick={changeLanguage}>{language === "en" ? "简体中文" : "English"}</button></header>
    {!state.identity&&<h1>{text.title}</h1>}<p className="candidate">{text.candidate}</p>
    {!languageSaved&&<p role="status">{language==="zh"?"本机存储不可用，语言仅在本页生效，无法记住此次选择。":"Local storage is unavailable. Language changed for this page, but this choice could not be saved."}</p>}
    {!state.identity && <form onSubmit={signIn} aria-busy={busy}>
      <label htmlFor="token">{text.token}</label>
      <input ref={tokenInput} id="token" name="token" type="password" autoComplete="off" spellCheck="false" required disabled={busy} aria-describedby="privacy" />
      <button type="submit" disabled={busy}>{busy ? text.verifying : text.signIn}</button>
      {busy && <button type="button" onClick={() => session.clear()}>{text.clear}</button>}
      {state.failure && <p role="alert">{text.errors[state.failure.kind]}</p>}
    </form>}
    {!state.identity&&oidcAvailable&&<button type="button" disabled={busy} onClick={()=>{setOIDCFailure(false);void browserOIDC.start().catch(()=>setOIDCFailure(true));}}>{language==="zh"?"使用企业 SSO 登录":"Sign in with SSO"}</button>}
    {!state.identity&&oidcFailure&&<p role="alert">{language==="zh"?"SSO 登录不可用或回调无效，请重新开始登录。":"SSO is unavailable or the callback was invalid. Start sign-in again."}</p>}
    {state.phase === "expired" && <p role="alert">{language === "zh" ? "已验证的凭据已到期，已停止新请求。草稿与请求证据仍保留在本页内存中；已发出的操作不会被取消或回滚。请先保存必要证据，再清除会话并重新登录。" : "The verified credential expired; new requests are blocked. Drafts and request evidence remain in page memory. Already dispatched operations are not canceled or rolled back. Record necessary evidence before clearing the session and signing in again."}</p>}
    {state.phase === "expired" && <section>{[...drafts.current].map(([name,model]) => <RetainedDraft key={name} name={name} model={model} language={language} />)}{!creation.current.model && <pre className="declaration-json">{stringifyJSON(creation.current.form)}</pre>}</section>}
    {state.phase==="expired"&&<section>{[...deletions.current].map(([name,model])=><RetainedDeletion key={name} model={model} language={language}/>)}</section>}
    {state.phase==="expired"&&<BatchHistory records={creation.current.batchHistory} language={language}/>}
    {state.phase==="expired"&&[...archives.current].map(([name,records])=><ArchivedEditors key={name} records={records} language={language}/>)}
    {authenticatedConsole && <>
      <ConsoleNavigation language={language} route={route} router={router} permissions={state.identity.permissions}/>
      <div id="console-content" tabIndex="-1">
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
        route.kind === "audit" && state.identity.permissions.includes("audit:read") ? <Audit api={api} language={language} route={route} router={router} /> :
        route.kind === "stream" ? <StreamDetail key={route.name} api={api} name={route.name} language={language} route={route} router={router} refreshSeconds={refreshSeconds} /> :
        route.kind === "queue" ? <QueueDeclaration key={route.name} api={api} name={route.name} language={language} route={route} router={router} canPreview={state.identity.permissions.includes("queue:preview")} canAudit={state.identity.permissions.includes("audit:read")} canDelete={state.identity.permissions.includes("queue:delete")} refreshSeconds={refreshSeconds} /> :
        route.kind === "delete-queue" && state.identity.permissions.includes("queue:delete") ? <><EditorHandoff entries={queueEditors(drafts.current,route.name)} language={language} onArchive={()=>{if(archiveQueueEditors({drafts:drafts.current,archives:archives.current,creation:creation.current,name:route.name,confirmed:true}))refreshHandoff(value=>value+1);else window.alert(language==="zh"?"当前操作进行中或结果未知，交接未执行。":"Operation pending or outcome unknown; no handoff performed.");}}/><QueueDelete blocked={editing(route.name)} model={(()=>{if(!deletions.current.has(route.name))deletions.current.set(route.name,createQueueDelete(api,route.name,{canStart:()=>!editing(route.name)}));return deletions.current.get(route.name);})()} language={language}/></> :
        route.kind === "edit-queue" && state.identity.permissions.includes("queue:preview") ? deleting(route.name)?<p role="alert">{language==="zh"?"此 Queue 保留了删除审阅或请求证据。请先返回删除页面取消审阅；已提交的操作须先保存证据并清除会话。":"Deletion review or request evidence is retained for this Queue. Return to deletion to cancel the review; submitted operations require saving evidence and clearing the session."}</p>:<QueueEditor model={(()=>{if(!drafts.current.has(route.name))drafts.current.set(route.name,createQueueDraft(api,route.name));return drafts.current.get(route.name);})()} language={language} /> :
        route.kind === "bulk-change" && state.identity.permissions.includes("queue:apply") ? <BulkChange api={api} language={language} prepare={async(document,etag)=>{const name=document.metadata.name;if(deleting(name)||editing(name))throw Object.assign(Error("Retained operation owns Queue"),{code:"retained"});const model=createQueueDraft(api,name);drafts.current.set(name,model);await model.load();const loaded=model.snapshot();if(loaded.phase!=="editing"||loaded.etag!==etag){model.archive();drafts.current.delete(name);throw Object.assign(Error("Queue changed after batch preview"),{code:loaded.phase==="editing"?"changed":"unavailable"});}model.edit(stringifyJSON(document));return model;}}/> :
        route.kind === "create-queue" && state.identity.permissions.includes("queue:apply") ? <QueueCreate api={api} setup={creation.current} language={language} prepare={document=>{if(deleting(document.metadata.name))throw Object.assign(new Error("Deletion retained"),{code:"retained-name"});return retainCreationDraft(drafts.current,document,value=>createQueueDraft(api,value.metadata.name,{document:value}));}} /> :
        route.kind === "consumer" ? <ConsumerDetail key={`${route.stream}/${route.name}`} api={api} stream={route.stream} name={route.name} language={language} refreshSeconds={refreshSeconds} /> :
        <p role="alert">{language === "zh" ? "该页面尚未实现或 URL 无效；不会自动执行创建或修改。" : "This page is not implemented or its URL is invalid; no create or update is performed."}</p>}
      </div>
    </>}
    <p id="privacy">{text.privacy}</p>
  </main>;
}

document.documentElement.lang = initialLanguage==="zh" ? "zh-CN" : "en";
createRoot(document.getElementById("root")).render(<App />);
