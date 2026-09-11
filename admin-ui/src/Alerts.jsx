import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createAlerts} from "./alerts.mjs";

export function Alerts({api,language}){
  const zh=language==="zh",model=useMemo(()=>createAlerts(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),t=(en,cn)=>zh?cn:en;
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  return <section aria-label={t("Operational alerts","运维告警")}><h2>{t("Operational alerts","运维告警")}</h2>
    <p>{t("Source: server-configured Prometheus rules. This console reads state only; it does not send notifications. Inactive means not currently firing, not proven recovery. Recovered is shown only after this management process observed a firing-to-inactive transition.","来源：服务端配置的 Prometheus 规则。本控制台只读取状态，不发送通知。未触发只表示当前未触发，不证明已恢复；只有本管理进程观察到触发转为未触发后才显示已恢复。")}</p>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{t("Refresh alerts","刷新告警")}</button>
    {state.phase==="loading"&&<p role="status">{t("Reading alert rules…","正在读取告警规则…")}</p>}
    {state.failure&&<p role="alert">{state.failure==="disabled"?t("Prometheus alert backend is not configured.","未配置 Prometheus 告警后端。") : state.failure==="denied"?t("Alert read was denied.","告警读取被拒绝。") : t("Alert rules are unavailable or incompatible.","告警规则不可用或响应不兼容。")}</p>}
    {state.value&&<>{state.value.consoleUrl?<p><a href={state.value.consoleUrl} target="_blank" rel="noopener noreferrer">{t("Open Prometheus alerts","打开 Prometheus 告警")}</a></p>:<p>{t("No browser-visible external alert console is configured.","未配置浏览器可访问的外部告警控制台。")}</p>}{state.value.missingRules.length>0&&<p role="alert">{t("Incomplete alert coverage. Missing repository rules: ","告警覆盖不完整。缺少仓库规则：")}{state.value.missingRules.join(", ")}</p>}<p>{t("Observed at","观测时间")}: <time dateTime={state.value.observedAt}>{state.value.observedAt}</time></p>
      {state.value.rules.length===0?<p>{t("No repository-defined rules were returned; alert coverage is unknown.","未返回仓库定义的规则，告警覆盖情况未知。")}</p>:<table><caption>{t("Configured operational rules","已配置运维规则")}</caption><thead><tr><th>{t("Rule","规则")}</th><th>{t("Severity","级别")}</th><th>{t("State","状态")}</th><th>{t("Threshold expression","阈值表达式")}</th><th>{t("For (seconds)","持续时间（秒）")}</th></tr></thead><tbody>{state.value.rules.map(rule=><tr key={rule.name}><th scope="row">{rule.summary||rule.name}<small>{rule.name}</small></th><td>{rule.severity}</td><td>{rule.state}{rule.activeAt&&<small>{t(" Active since "," 触发时间 ")}<time dateTime={rule.activeAt}>{rule.activeAt}</time></small>}{rule.recoveredAt&&<small>{t(" Recovered at "," 恢复时间 ")}<time dateTime={rule.recoveredAt}>{rule.recoveredAt}</time></small>}</td><td><code>{rule.query}</code></td><td>{rule.duration}</td></tr>)}</tbody></table>}
    </>}
  </section>;
}
