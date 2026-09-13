import React, {useEffect, useMemo, useSyncExternalStore} from "react";
import {createMetricHistory, historyWindows} from "./metric-history.mjs";
import {metricHistoryLabels, metricOptionLabel} from "./metric-history-labels.mjs";

function segments(series, start, end, min, max) {
  const width = 760, height = 220, pad = 18, range = max - min || 1;
  const x = time => pad + (Date.parse(time) - start) / (end - start) * (width - pad * 2);
  const y = value => height - pad - (Number(value) - min) / range * (height - pad * 2);
  const result = [];
  let current = [];
  for (const sample of series.samples) {
    if ((sample.gapBefore || sample.reset) && current.length) { result.push(current); current = []; }
    current.push(`${x(sample.time).toFixed(2)},${y(sample.value).toFixed(2)}`);
  }
  if (current.length) result.push(current);
  return {width, height, result};
}

export function MetricHistory({api, language, queue="", metrics}) {
  const text = metricHistoryLabels(language);
  const options = metrics?.map(option => ({...option, label: metricOptionLabel(option, language)})) ?? text.defaults;
  const model = useMemo(() => createMetricHistory(api, {metric: options[0].id, queue}), [api, queue]);
  const state = useSyncExternalStore(model.subscribe, model.snapshot);
  const value = state.value;
  useEffect(() => { void model.load(); return () => model.clear(); }, [model]);
  const all = value?.series.flatMap(item => item.samples.map(sample => Number(sample.value))) ?? [];
  let min = Infinity, max = -Infinity;
  for (const number of all) { if (number < min) min = number; if (number > max) max = number; }
  if (!all.length) min = max = 0;
  const selectedLabel = options.find(option => option.id === value?.metric)?.label ?? value?.metric;
  return <section className="metric-history" aria-label={text.title}><h3>{text.title}</h3><p>{text.description}</p>
    <form onSubmit={event => { event.preventDefault(); void model.load(); }}><label>{text.metric}<select value={state.selection.metric} onChange={event => model.select({metric: event.target.value, window: state.selection.window})}>{options.map(option => <option key={option.id} value={option.id}>{option.label}</option>)}</select></label><label>{text.window}<select value={state.selection.window} onChange={event => model.select({metric: state.selection.metric, window: event.target.value})}>{historyWindows.map(window => <option key={window}>{window}</option>)}</select></label><button disabled={state.phase === "loading"}>{text.refresh}</button></form>
    {state.phase === "loading" && <p role="status">{text.querying}</p>}{state.error && <p role="alert">{text.errors[state.error] ?? text.errors.unavailable}</p>}
    {value && <>{all.length === 0 ? <p>{text.noSamples}</p> : value.series.map((series, index) => {
      if (!series.samples.length) return null;
      const graph = segments(series, Date.parse(value.start), Date.parse(value.end), min, max);
      return <figure key={index}><svg viewBox={`0 0 ${graph.width} ${graph.height}`} role="img" aria-label={`${selectedLabel}: ${series.samples.length} ${text.samples}`} preserveAspectRatio="none">{graph.result.map((points, i) => <polyline key={i} points={points.join(" ")} fill="none" stroke="currentColor" strokeWidth="3" vectorEffect="non-scaling-stroke"/>)}{series.samples.map(sample => { const point = segments({samples: [sample]}, Date.parse(value.start), Date.parse(value.end), min, max).result[0][0].split(","); return <circle key={sample.time} cx={point[0]} cy={point[1]} r="3" fill="currentColor"/>; })}</svg><figcaption>{Object.entries(series.labels).map(([key, label]) => `${key}=${label}`).join(", ") || text.defaultSeries}</figcaption></figure>;
    })}{all.length > 0 && <details><summary>{text.exactSamples}</summary>{value.series.map((series, index) => <table key={index}><caption>{Object.entries(series.labels).map(([key, label]) => `${key}=${label}`).join(", ") || value.metric}</caption><thead><tr><th>{text.time}</th><th>{text.value}</th><th>{text.evidence}</th></tr></thead><tbody>{series.samples.map(sample => <tr key={sample.time}><td><time dateTime={sample.time}>{sample.time}</time></td><td>{sample.value}</td><td>{[sample.gapBefore && text.gap, sample.reset && text.reset].filter(Boolean).join(", ") || text.none}</td></tr>)}</tbody></table>)}</details>}</>}
  </section>;
}
