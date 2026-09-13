import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createAlerts} from "./alerts.mjs";
import {alertsLabels} from "./alerts-labels.mjs";
import {runEventInvalidations} from "./event-invalidations.mjs";

export function Alerts({api,language}){
  const zh=language==="zh",model=useMemo(()=>createAlerts(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),text=alertsLabels(language);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  useEffect(()=>{const controller=new AbortController();void runEventInvalidations(api,{signal:controller.signal,onInvalidate:resource=>resource==="alerts"?model.load():undefined});return()=>controller.abort();},[api,model]);
  return <section aria-label={text.operational_alerts}><h2>{text.operational_alerts}</h2>
    <p>{text.source_server_configured_prometheus_rules_this_console}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{text.refresh_alerts}</button>
    {state.phase==="loading"&&<p role="status">{text.reading_alert_rules}</p>}
    {state.failure&&<p role="alert">{state.failure==="disabled"?text.prometheus_alert_backend_is_not_configured : state.failure==="denied"?text.alert_read_was_denied : text.alert_rules_are_unavailable_or_incompatible}</p>}
    {state.value&&<>{state.value.consoleUrl?<p><a href={state.value.consoleUrl} target="_blank" rel="noopener noreferrer">{text.open_prometheus_alerts}</a></p>:<p>{text.no_browser_visible_external_alert_console_is}</p>}{state.value.missingRules.length>0&&<p role="alert">{text.incomplete_alert_coverage_missing_repository_rules}{state.value.missingRules.join(", ")}</p>}<p>{text.observed_at}: <time dateTime={state.value.observedAt}>{state.value.observedAt}</time></p>
      {state.value.rules.length===0?<p>{text.no_repository_defined_rules_were_returned_alert}</p>:<table><caption>{text.configured_operational_rules}</caption><thead><tr><th>{text.rule}</th><th>{text.severity}</th><th>{text.state}</th><th>{text.threshold_expression}</th><th>{text.for_seconds}</th></tr></thead><tbody>{state.value.rules.map(rule=><tr key={rule.name}><th scope="row">{rule.summary||rule.name}<small>{rule.name}</small></th><td>{rule.severity}</td><td>{rule.state}{rule.activeAt&&<small>{text.active_since}<time dateTime={rule.activeAt}>{rule.activeAt}</time></small>}{rule.recoveredAt&&<small>{text.recovered_at}<time dateTime={rule.recoveredAt}>{rule.recoveredAt}</time></small>}</td><td><code>{rule.query}</code></td><td>{rule.duration}</td></tr>)}</tbody></table>}
    </>}
  </section>;
}
