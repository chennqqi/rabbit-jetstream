import React, {useEffect, useMemo, useSyncExternalStore} from "react";
import {stringifyJSON} from "./api.mjs";
import {QueueConsumers} from "./QueueConsumers.jsx";
import {routingLabels} from "./routing-labels.mjs";
import {QueueSummary, QueueEvents} from "./QueuePanels.jsx";
import {queueTabs, queueTabURL} from "./routes.mjs";
import {createQueueDeclaration} from "./queue-declaration.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {DLQDiagnostics} from "./DLQDiagnostics.jsx";
import {RoutingProbe} from "./RoutingProbe.jsx";
import {QueueExport} from "./QueueExport.jsx";
import {CopyValue} from "./copy-value.jsx";
import {MetricHistory} from "./MetricHistory.jsx";
import {ResourceBreadcrumb} from "./resource-breadcrumb.jsx";
import {RouteLink} from "./route-link.jsx";
import {queueDeclarationLabels} from "./queue-declaration-labels.mjs";

// Declaration view is not an observed topology/health summary.
export function QueueDeclaration({api, name, language, route, router, canPreview, canAudit, canDelete, refreshSeconds=10}) {
  const model = useMemo(() => createQueueDeclaration(api, name), [api, name]);
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  useEffect(() => { void model.load(); return () => model.clear(); }, [model]);
  const text = queueDeclarationLabels(language);
  const tab = route.tab ?? (route.consumerQuery ? "consumers" : "summary");
  const routing = routingLabels(language);
  return <section aria-labelledby="queue-heading" className="queue-list queue-detail">
    <ResourceBreadcrumb trail={[{label: text.allQueues, href: "/admin/queues"}, {label: name}]} router={router} language={language}/>
    <header className="queue-resource-header"><h2 id="queue-heading">Queue: {name}</h2>
      <button disabled={state.phase === "loading"} onClick={() => void model.load()}>{text.refresh}</button>
      {canPreview && <p><RouteLink href={`/admin/queues/by-name/${encodeURIComponent(name)}/edit`} router={router}>{text.edit}</RouteLink></p>}
      {canDelete && <p><RouteLink href={`/admin/queues/by-name/${encodeURIComponent(name)}/delete`} router={router}>{text.delete}</RouteLink></p>}
    </header>
    {state.phase === "ready" && <dl className="declaration-meta"><dt>{text.revision}</dt><dd><CopyValue value={state.body.revision} language={language} short/></dd><dt>{text.etag}</dt><dd>{state.etag ?? text.unknown}</dd><dt>{text.readCompleted}</dt><dd><ReadTime value={state.readAt} language={language}/></dd></dl>}
    <details className="reading-help"><summary>{text.guide}</summary><p>{text.guideText}</p></details>
    <nav aria-label={text.tabLabel}>{queueTabs.map((key, index) => <RouteLink key={key} aria-current={tab === key ? "page" : undefined} href={queueTabURL(name, key, route.consumerQuery, route.eventBefore)} router={router}>{text.tabs[index]} </RouteLink>)}</nav>
    {state.phase === "loading" && <p role="status">{text.loading}</p>}
    {state.phase === "error" && <p role="alert">{state.status === 404 && state.code === "not_found" ? text.errors.missing : state.status === 401 || state.status === 403 ? text.errors.denied : text.errors.unavailable}</p>}
    {state.phase === "ready" && <>
      {tab === "summary" && <><QueueSummary api={api} plan={state.body.plan} language={language} router={router} route={route} refreshSeconds={refreshSeconds}/><MetricHistory api={api} language={language} queue={name} metrics={[{id: "queue-messages", en: text.metrics.messages, zh: text.metrics.messages}, {id: "queue-bytes", en: text.metrics.bytes, zh: text.metrics.bytes}]}/></>}
      {tab === "routing" && <p>{text.inputSubjects}: {(state.body.plan.declarationSubjects ?? state.body.document?.spec?.subjects)?.join(", ") ?? text.unknown}</p>}
      {tab === "configuration" && <><h3>{text.configuration}</h3>{state.body.document ? <pre className="declaration-json">{stringifyJSON(state.body.document)}</pre> : <p>{text.notEditable}</p>}<details><summary>{text.rawPlan}</summary><pre className="declaration-json">{stringifyJSON(state.body.plan)}</pre></details></>}
      {tab === "routing" && <><h3>{text.routingMap}</h3><p>{text.routingNote}</p>{state.body.plan.routing?.length ? <div className="table-scroll"><table><thead><tr>{[routing.exchange, routing.type, routing.keys, routing.subjects].map(label => <th key={label}>{label}</th>)}</tr></thead><tbody>{state.body.plan.routing.map((row, index) => <tr key={index}><td>{row.exchange}</td><td>{row.type}</td><td>{row.keys?.join(", ")}</td><td>{row.subjects?.join(", ")}</td></tr>)}</tbody></table></div> : <p>{text.noBindings} {state.body.plan.stream.subjects?.join(", ")}</p>}</>}
      {tab === "consumers" && <QueueConsumers api={api} queue={name} language={language} declarationETag={state.etag} route={route} router={router} refreshSeconds={refreshSeconds}/>}
      {tab === "routing" && <RoutingProbe api={api} name={name} revision={state.body.revision} etag={state.etag} language={language}/>}
      {tab === "configuration" && <DLQDiagnostics api={api} plan={state.body.plan} declarationETag={state.etag} declarationReadAt={state.readAt} language={language} router={router}/>}
      {tab === "configuration" && <QueueExport api={api} name={name} document={state.body.document} etag={state.etag} language={language}/>}
      {tab === "events" && (canAudit ? <QueueEvents api={api} name={name} language={language} router={router} route={route}/> : <p role="alert">{text.auditDenied}</p>)}
    </>}
  </section>;
}
