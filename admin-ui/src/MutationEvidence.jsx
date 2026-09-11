import React from "react";
import {errorMutationEvidence} from "./mutation-evidence.mjs";

export function MutationEvidence({state,language}){
  const evidence=errorMutationEvidence(state.error),t=(en,zh)=>language==="zh"?zh:en;
  if(!state.requestId||!["uncertain","inspecting"].includes(state.phase)||!evidence)return null;
  return <section aria-label={t("Receiving-attempt evidence","接收端单次尝试证据")}>
    <h3>{t("Receiving-attempt evidence","接收端单次尝试证据")}</h3>
    <p>{t("This describes one receiving attempt, not every transport replay or the original request's durable outcome. No retry is enabled.",
      "仅描述接收端的一次尝试，不代表所有传输重放或原始请求的持久结果。不会解锁重试。")}</p>
    <dl><dt>{t("Reported phase","报告阶段")}</dt><dd>{evidence.phase}</dd>
      <dt>{t("Queue resource effects in this attempt","本次尝试的 Queue 资源影响")}</dt>
      <dd>{evidence.resourceEffects==="none"?t("None reported; audit/lock metadata excluded.","报告无资源影响，不包含审计/锁元数据。"):t("Possible; partial effects or completion are not resolved.","可能有影响，尚不能判定部分执行或已完成。")}</dd>
      {evidence.intentId&&<><dt>{t("Intent identifier (persistence not guaranteed)","意图标识（不保证已持久化）")}</dt><dd><code>{evidence.intentId}</code></dd></>}
    </dl>
  </section>;
}
