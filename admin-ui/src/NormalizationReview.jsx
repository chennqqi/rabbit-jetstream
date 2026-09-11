import React from "react";
import {normalizationReview} from "./normalization-review.mjs";

export function NormalizationReview({submitted,normalized,language}) {
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,{rows,truncated}=normalizationReview(submitted,normalized);
  const labels=zh?{added:"规范化后补入",omitted:"规范化后省略",different:"表示不同"}:{added:"Added by normalization",omitted:"Omitted by normalization",different:"Representation differs"};
  return <details><summary>{t("Submitted versus normalized fields","提交值与规范化值对照")}</summary>
    <p>{t("Compares this draft with the server's normalized target, including values it supplied for omitted fields. This is a representation comparison, not a second semantic change plan: units, ordering and omitted defaults may differ without changing configuration. No values are copied back into your draft. Paths use JSON Pointer escaping (~0 for ~, ~1 for /). Arrays remain whole values.","比较本次草稿与服务端规范化目标，包括服务端为省略字段补入的值。这是表示形式对照，不是第二份语义变更计划：单位、顺序及默认值的省略方式不同不一定改变配置。不向草稿回填任何值。路径使用 JSON Pointer 转义（~0 表示 ~，~1 表示 /），数组整体展示。")}</p>
    {rows.length===0?<p>{t("No representation differences found in the returned document. This does not prove resource convergence.","返回文档中未发现表示差异，不证明资源已收敛。")}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={t("Normalization field comparison","规范化字段对照")}><table>
      <thead><tr><th>{t("Field path","字段路径")}</th><th>{t("Submitted","提交值")}</th><th>{t("Normalized","规范化值")}</th><th>{t("Representation","表示形式")}</th></tr></thead>
      <tbody>{rows.map(row=><tr key={row.path}><th scope="row"><code>{row.path}</code></th><td><pre className="declaration-json">{row.before===undefined?t("Not supplied","未提供"):row.before}</pre></td><td><pre className="declaration-json">{row.after===undefined?t("Not supplied","未提供"):row.after}</pre></td><td>{labels[row.kind]}</td></tr>)}</tbody>
    </table></div>}
    {truncated&&<p role="status">{t("Only the first 256 differing fields are shown. Inspect the complete draft and normalized document for the remainder.","仅显示前 256 个不同字段，其余请查看完整草稿与规范化文档。")}</p>}
  </details>;
}
