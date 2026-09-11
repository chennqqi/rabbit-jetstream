import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createCompatibility} from "./compatibility.mjs";
import {ConsoleCapabilities} from "./ConsoleCapabilities.jsx";
import {systemLabels} from "./system-labels.mjs";

export function Compatibility({api,language}){
  const t=(en,zh)=>language==="zh"?zh:en,model=useMemo(()=>createCompatibility(api),[api]),accessible=systemLabels(language);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const evidence=source=><>{source.phase==="loading"&&<p role="status">{t("Reading compatibility metadata…","正在读取兼容性元数据…")}</p>}{source.phase==="error"&&<p role="alert">{t("Source unavailable, denied or incompatible; previous metadata cleared.","来源不可用、拒绝访问或不兼容，已清除此前元数据。")}</p>}{source.readAt&&<p>{t("Read completed","读取完成")}: <time dateTime={source.readAt}>{source.readAt}</time></p>}</>;
  const build=state.build.value,sdk=state.sdk.value,unknown=t("Unreported","未报告");
  return <section aria-label={accessible.compatibility}><h2>{t("Compatibility","兼容性")}</h2>
    <p>{t("Independent reported metadata, not an atomic snapshot, artifact attestation or production qualification. No upgrade, restart or feature changes are performed here.","各来源独立报告的元数据，不是原子快照、制品认证或生产资格。此处不执行升级、重启或功能变更。")}</p>
    <button disabled={Object.values(state).some(s=>s.phase==="loading")} onClick={()=>void model.load()}>{t("Refresh compatibility metadata","刷新兼容性元数据")}</button>
    <section aria-label={accessible.build}><h3>{t("Management process build","管理进程构建")}</h3>{evidence(state.build)}{build&&<dl>
      {[[t("Reported version","报告版本"),build.version],[t("Go runtime","Go 运行时"),build.goVersion],[t("Build target","构建目标"),`${build.os}/${build.arch}`],[t("VCS revision","VCS 版本"),build.revision??unknown],[t("Revision source","版本来源"),build.revisionSource??unknown],[t("Modified source at build","构建时源码已修改"),build.modified===undefined?unknown:String(build.modified)],[t("Embedded UI identity","内嵌 UI 身份"),build.uiAssets?.digest??unknown],[t("Embedded UI files","内嵌 UI 文件数"),build.uiAssets?.fileCount??unknown]].map(([key,value])=><React.Fragment key={key}><dt>{key}</dt><dd><code>{value}</code></dd></React.Fragment>)}
    </dl>}<p>{t("The UI digest identifies the exact embedded files served by this process; it is not a signature or qualification verdict. Missing VCS metadata does not prove a clean or released build.","UI 摘要标识该进程实际提供的精确内嵌文件集；它不是签名或资格结论。缺少 VCS 元数据不能证明构建干净或已经发布。")}</p></section>
    <section aria-label={accessible.sdk}><h3>{t("Published native SDK contract","发布的原生 SDK 契约")}</h3>{evidence(state.sdk)}{sdk&&<>
      <dl>{[[t("Schema","Schema"),sdk.schema],[t("Declared availability","声明的可用阶段"),sdk.availability],[t("Delivery guarantee","投递保证"),sdk.delivery.delivery_guarantee],[t("Publisher confirmation","发布确认"),sdk.delivery.publisher_confirm],[t("Acknowledgment policy","确认策略"),sdk.delivery.ack_policy]].map(([key,value])=><React.Fragment key={key}><dt>{key}</dt><dd>{value}</dd></React.Fragment>)}</dl>
      <h4>{t("Message headers","消息头")}</h4><ul>{sdk.headers.map((header,index)=><li key={index}><code>{header.name}</code> — {header.required?t("required","必填"):t("optional","可选")}</li>)}</ul>
    </>}<p>{t("A published contract does not prove an SDK release is available or compatible with an installed client. This is not RabbitMQ wire-protocol compatibility.","公开契约不证明 SDK 已发布或与已安装客户端兼容，也不代表 RabbitMQ 线协议兼容。")}</p></section>
    <ConsoleCapabilities api={api} language={language}/>
  </section>;
}
