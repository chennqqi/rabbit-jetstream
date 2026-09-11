import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createCapabilities} from "./capabilities.mjs";
import {stringifyJSON} from "./api.mjs";
import {systemLabels} from "./system-labels.mjs";

export function ConsoleCapabilities({api,language}){
  const model=useMemo(()=>createCapabilities(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),zh=language==="zh",accessible=systemLabels(language);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const t=(en,cn)=>zh?cn:en,body=state.body;
  return <section aria-label={accessible.capabilities} className="queue-list console-capabilities">
    <h3>{t("Server-declared capabilities","服务端声明的能力")}</h3>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{t("Refresh server capabilities","刷新服务端能力")}</button>
    {state.phase==="loading"&&<p role="status">{t("Reading contract metadata…","正在读取契约元数据…")}</p>}
    {state.phase==="error"&&<p role="alert">{t("Capabilities unavailable, denied or incompatible. Deployment intent and defaults are unknown; do not infer them from reachable nodes.","能力接口不可用、拒绝访问或契约不兼容。部署期望和默认值未知，不得按可达节点推断。")}</p>}
    {state.phase==="ready"&&<>
      <dl className="declaration-meta">
        <dt>{t("Declared deployment profile","声明的部署模式")}</dt><dd>{body.deployment.profile}</dd>
        <dt>{t("Profile source","模式来源")}</dt><dd>{body.deployment.source}</dd>
        <dt>{t("Parser-supported replicas","解析器支持副本数")}</dt><dd>{body.queue.supportedReplicas.join(", ")}</dd>
        <dt>{t("Parser-supported storage","解析器支持存储")}</dt><dd>{body.queue.supportedStorage.join(", ")}</dd>
        <dt>{t("Parser-supported priority range","解析器支持优先级范围")}</dt><dd>{body.queue.minimumPriority}–{body.queue.maximumPriority}</dd>
        <dt>{t("Production qualification","生产资格")}</dt><dd>{body.qualification.status==="reported"?t("Manifest statement reported","已报告清单声明"):t("Unreported by this API","此接口未报告")}</dd>
        {body.qualification.status==="reported"&&<><dt>{t("Manifest statement","清单声明")}</dt><dd>{body.qualification.statement}</dd><dt>{t("Manifest SHA-256","清单 SHA-256")}</dt><dd><code>{body.qualification.manifestDigest}</code></dd></>}
        <dt>{t("Read completed","读取完成")}</dt><dd>{state.readAt}</dd>
        <dt>{t("Queue document version","Queue 文档版本")}</dt><dd>{body.queue.apiVersion}</dd>
      </dl>
      <p>{t("Deployment profile is configuration, not observed topology or health. Supported values are parser limits, not qualified production profiles. A reported manifest statement is shown verbatim and is not upgraded into a qualification verdict. Replica count still requires explicit selection.","部署模式来自配置，不是观测拓扑或健康状态。支持值是解析器范围，不是已验收生产方案。已报告的清单声明只会原样显示，不会被升级为资格结论。副本数仍须明确选择。")}</p>
      <details><summary>{t("Canonical omitted-field defaults","服务端省略字段默认值")}</summary><pre className="declaration-json">{stringifyJSON(body.queue.defaults)}</pre><p>{t("These partial defaults are not a complete Queue document or a deployment recommendation. They are not copied into drafts.","这些局部默认值不是完整 Queue 文档或部署建议，不会复制到草稿。")}</p></details>
      <details><summary>{t("Implemented API contract identifiers","已实现 API 契约标识")}</summary><ul>{body.features.map(feature=><li key={feature}><code>{feature}</code></li>)}</ul><p>{t("Contract implementation is not permission, backend availability or release qualification.","契约实现不代表权限、后端可用性或发布资格。")}</p></details>
      {state.schema&&<details><summary>{t("Verified Queue authoring schema","已核对的 Queue 编写 Schema")}</summary>
        <p>{t("All document fields are described. Custom formats and cross-resource rules still require server preview. Schema defaults are annotations and are never inserted into drafts.","描述全部文档字段。自定义格式及跨资源规则仍须服务端预览验证。Schema 默认值仅作说明，不会插入草稿。")}</p>
        <p><code>{body.queue.schema.id}</code></p>
        <pre className="declaration-json">{stringifyJSON(state.schema)}</pre>
      </details>}
    </>}
  </section>;
}
