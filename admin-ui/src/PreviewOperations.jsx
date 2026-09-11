import React from "react";
import {previewOperations} from "./preview-operations.mjs";

export function PreviewOperations({preview,language}) {
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,operations=previewOperations(preview.result);
  const value=text=>text===undefined?<span>{t("Not supplied by server","服务端未提供")}</span>:<pre className="declaration-json">{text===""?'""':text}</pre>;
  return <section aria-label={t("Observed-to-desired resource changes","观测资源到目标资源的变化")}>
    <h4>{t("Resource operations and field changes","资源操作与字段变化")}</h4>
    <p>{t("These are server comparisons of observed resources with the proposed generated configuration, not a declaration-to-declaration diff. Impact labels are advisory, not approval, health or qualification. Missing old/new values are not interpreted as zero or absence. Preview may become stale; apply rechecks safety.","以下是服务端对观测资源与拟生成配置的比较，不是两份声明之间的差异。影响等级仅供参考，不代表批准、健康或资格。未提供的旧/新值不解释为零或不存在。预览可能过时，提交时会重新检查安全条件。")}</p>
    {operations===null?<p role="alert">{t("Structured operation data is unavailable or invalid. Inspect the raw preview; no change summary can be established.","结构化操作数据缺失或无效。请检查原始预览，无法建立变更摘要。")}</p>:operations.length===0?<p>{t("The server supplied no operation rows; this alone is not proof that nothing changes.","服务端未提供操作行，仅凭此不能证明没有变化。")}</p>:operations.map(operation=><section key={JSON.stringify([operation.resource,operation.name])} className="preview-operation">
      <h5><code>{operation.resource}</code> · <code>{operation.name}</code></h5>
      <p>{t("Action","操作")}: <code>{operation.action}</code> · {t("Reported impact","报告的影响")}: <code>{operation.impact}</code> · {operation.blocked?t("Blocked","已阻止"):t("Not blocked by this preview","本次预览未阻止")}</p>
      {operation.reason&&<p>{t("Server reason","服务端原因")}: {operation.reason}</p>}
      {operation.changes.length===0?<p>{t("No field-level rows supplied for this operation. Create/ensure actions may still propose work; consult the generated plan.","此操作未提供逐字段变化行。Create/ensure 操作仍可能需要执行，请查看生成计划。")}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={`${operation.resource} ${operation.name} ${t("field changes","字段变化")}`}><table>
        <thead><tr><th>{t("Field","字段")}</th><th>{t("Observed value","观测值")}</th><th>{t("Proposed value","目标值")}</th><th>{t("Reported impact","报告的影响")}</th></tr></thead>
        <tbody>{operation.changes.map((change,index)=><tr key={index}><th scope="row"><code>{change.path}</code></th><td>{value(change.from)}</td><td>{value(change.to)}</td><td><code>{change.impact}</code></td></tr>)}</tbody>
      </table></div>}
    </section>)}
  </section>;
}
