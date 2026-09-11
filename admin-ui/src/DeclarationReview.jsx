import React from "react";
import {declarationReview} from "./declaration-review.mjs";
import {stringifyJSON} from "./api.mjs";
import {NormalizationReview} from "./NormalizationReview.jsx";

export function DeclarationReview({state,language}) {
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,review=declarationReview(state);
  const value=text=>text===undefined?t("Not supplied by server","服务端未提供"):text===""?'""':text;
  return <section className="declaration-review" aria-label={t("Declaration change review","声明变更审阅")}>
    <h4>{t("Declaration changes","声明变化")}</h4>
    <p>{t("This compares the original accepted declaration with the normalized target. It is separate from observed-resource repair below. Impact labels do not authorize changes or establish health. The normalized document never replaces your draft or original ETag.","此处比较原始已接受声明与规范化目标，与下方观测资源修复分开。影响等级不授权变更，也不证明健康。规范化文档不会替换草稿或原始 ETag。")}</p>
    {!state.create&&<p>{t("Original ETag for this preview","本次预览的原始 ETag")}: <code>{state.etag}</code></p>}
    {review.status==="missing"&&<p>{t("This server did not supply declaration review. No declaration difference can be inferred from its absence; the resource preview remains separate.","服务端未提供声明审阅，不能从缺失推断声明无变化；资源预览是独立信息。")}</p>}
    {review.status==="invalid"&&<p role="alert">{t("Declaration review is malformed or does not match this draft identity/revision. Confirmation is unavailable; retry preview explicitly.","声明审阅格式无效或与草稿身份/版本不匹配。无法确认提交，请手动重新预览。")}</p>}
    {review.status==="unavailable"&&<p>{t("A lossless declaration comparison is unavailable. Server reason:","无法进行无损声明比较。服务端原因：")} <code>{review.reason}</code></p>}
    {review.status==="create"&&<p>{t("Create-only: no previous declaration or old-value diff is fabricated. Review the normalized target and proposed resource operations.","仅创建：不伪造旧声明或旧值差异。请审阅规范化目标及拟执行的资源操作。")}</p>}
    {review.status==="available"&&(review.changes.length===0?<p>{t("No normalized declaration changes reported. Observed resources may still need repair; inspect their operations separately.","未报告规范化声明变化。观测资源仍可能需要修复，请单独检查资源操作。")}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={t("Declaration field differences","声明字段差异")}><table>
      <thead><tr><th>{t("Field","字段")}</th><th>{t("Original declaration","原始声明")}</th><th>{t("Normalized target","规范化目标")}</th><th>{t("Reported impact","报告的影响")}</th></tr></thead>
      <tbody>{review.changes.map(change=><tr key={change.path}><th scope="row"><code>{change.path}</code></th><td><pre className="declaration-json">{value(change.from)}</pre></td><td><pre className="declaration-json">{value(change.to)}</pre></td><td><code>{change.impact}</code></td></tr>)}</tbody>
    </table></div>)}
    {review.document&&<>
      <NormalizationReview submitted={state.draft} normalized={review.document} language={language}/>
      <details><summary>{t("Normalized target Queue document","规范化目标 Queue 文档")}</summary><p>{t("Includes server normalization/defaults. Units or formatting may differ from the submitted JSON without changing generated configuration.","包含服务端规范化/默认值。单位或格式可能与提交 JSON 不同，但不改变生成配置。")}</p><pre className="declaration-json">{stringifyJSON(review.document)}</pre></details>
    </>}
  </section>;
}
