import React from "react";
import {summaryConsumerCounts} from "./summary-consumer.mjs";
import {consumerCounter} from "./queue-consumers.mjs";
import {ReadTime} from "./DisplayValues.jsx";
import {summaryMetricsLabels} from "./summary-labels.mjs";
import {StaleEvidence} from "./stale-evidence.jsx";
import {RouteLink} from "./route-link.jsx";

export function SummaryMetrics(props) {
  const scope = props.scope;
  const text = summaryMetricsLabels(props.language);
  return scope ? <ScopedMetrics {...props} scope={scope} key={`${scope.stream}/${scope.name}`}/> : <p role="alert">{text.invalidScope}</p>;
}

function ScopedMetrics({scope, streamState, consumerState: state, language, router, onRefresh, disabled, paused, clock}) {
  const text = summaryMetricsLabels(language);
  const counts = summaryConsumerCounts(scope, state, {allowHistorical: true});
  const values = [[text.values.stored, streamState.resource ? consumerCounter({observed: streamState.resource}, "messages") : null], [text.values.pending, counts.pending], [text.values.ackPending, counts.ackPending]];
  return <div role="group" aria-label={text.metrics}>
    <p>{text.primary}: <RouteLink href={scope.url} router={router}>{scope.stream} / {scope.name}</RouteLink></p>
    <button disabled={disabled} onClick={onRefresh}>{text.refresh}</button>
    {state.phase === "loading" && <p role="status">{text.loading}</p>}
    {state.failure && <p role="alert">{state.failure === "missing" ? text.missing : text.failed}</p>}
    {state.resource && (state.phase === "loading" || state.failure) && <p role="status">{text.historical}</p>}
    <StaleEvidence readAt={state.readAt} paused={paused} failure={state.failure} clock={clock} note={text.stale}/>
    <dl className="summary-metrics">{values.map(([label, value]) => <div key={label}><dt>{label}</dt><dd className={label === text.values.pending && typeof value === "bigint" && value > 0n || label === text.values.pending && Number.isSafeInteger(value) && value > 0 ? "metric-attention" : undefined}>{value ?? text.unknown}</dd></div>)}</dl>
    <p>{text.readCompleted}: <ReadTime value={state.readAt} language={language}/></p>
    <details className="reading-help"><summary>{text.guide}</summary><p>{text.guideText}</p></details>
  </div>;
}
