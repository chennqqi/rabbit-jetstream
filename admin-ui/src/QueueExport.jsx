import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createQueueExport} from "./queue-export.mjs";

export function QueueExport({api,name,document,etag,language}){
  const model=useMemo(()=>createQueueExport(api,name,document,etag),[api,name,document,etag]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),[includeLabels,setIncludeLabels]=useState(false),[download,setDownload]=useState(null);
  const t=(en,zh)=>language==="zh"?zh:en;
  useEffect(()=>()=>model.clear(),[model]);
  useEffect(()=>{
    if(!state.file){setDownload(null);return;}
    try {const url=URL.createObjectURL(new Blob([state.file.content],{type:"application/json"})),changeUrl=state.file.changeContent?URL.createObjectURL(new Blob([state.file.changeContent],{type:"application/json"})):null;setDownload({file:state.file,url,changeUrl});return()=>{URL.revokeObjectURL(url);if(changeUrl)URL.revokeObjectURL(changeUrl);};}
    catch {setDownload({file:state.file,error:true});}
  },[state.file]);
  const errors={denied:t("Export read denied.","导出读取被拒绝。"),disabled:t("Resource reads are disabled.","资源读取未启用。"),missing:t("Queue declaration not found.","Queue 声明不存在。"),changed:t("Declaration changed or cannot be reconstructed. Refresh the Queue page.","声明已变化或无法完整还原，请刷新 Queue 页面。"),invalid:t("Export evidence is invalid or differs from the displayed declaration. No download was prepared.","导出证据无效或与页面声明不符，未生成下载。"),limit:t("Export exceeds the size limit; no partial file is offered.","导出超过大小上限，不提供部分文件。"),unavailable:t("Export is unavailable. Retry explicitly.","导出不可用，请手动重试。")};
  return <section aria-label={t("Queue declaration export","Queue 声明导出")}>
    <h3>{t("Export one Queue declaration","导出单个 Queue 声明")}</h3>
    <p>{t("Downloads configuration only, not messages or a backup. DLQ targets are references, not included documents. Import requires dependency review and a fresh destination preview; source ETag is not a destination write precondition.","仅下载配置，不包含消息，也不是备份。DLQ 目标仅保留引用，不包含其文档。导入需要依赖审阅及目标端重新预览，来源 ETag 不是目标写入前置条件。")}</p>
    <p>{t("Labels are excluded by default. Names and routing values may still be sensitive; inspect before sharing. Clearing the session does not delete files already downloaded.","默认排除标签。名称和路由值仍可能敏感，分享前请检查。清除会话不会删除已下载文件。")}</p>
    {!document?<p role="alert">{t("This declaration cannot be faithfully exported.","此声明无法完整还原以供导出。")}</p>:<>
      <label><input type="checkbox" checked={includeLabels} onChange={event=>{model.clear();setIncludeLabels(event.target.checked);}}/>{t("Include all free-form labels (may contain sensitive data)","包含全部自由文本标签（可能包含敏感数据）")}</label>
      <button type="button" disabled={state.phase==="loading"} onClick={()=>void model.prepare(includeLabels)}>{t("Prepare declaration download","准备声明下载")}</button>
      {state.phase==="loading"&&<><p role="status">{t("Reading and verifying export…","正在读取并核验导出…")}</p><button type="button" onClick={()=>model.clear()}>{t("Cancel export","取消导出")}</button></>}
      {state.failure&&<p role="alert">{errors[state.failure]}</p>}
      {state.file&&<><p>{t("Source ETag: ","来源 ETag：")}{state.file.etag} · {t("Omitted labels: ","省略标签数：")}{state.file.omitted}</p>
        <p>{t("Prepared from a saved-declaration read at: ","根据以下时间读取的已保存声明准备：")}<time dateTime={state.file.readAt}>{state.file.readAt}</time></p>
        {download?.file===state.file&&(download.error?<p role="alert">{t("Browser could not prepare the local file.","浏览器无法准备本地文件。")}</p>:<><a href={download.url} download={state.file.name}>{t("Download Queue JSON","下载 Queue JSON")}</a>{download.changeUrl&&<><p>{t("The versioned change package includes all labels and the current source ETag. Edit its document, then use Bulk changes. Sharing it may expose sensitive configuration.","版本化更新包包含全部标签和当前来源 ETag。编辑其中的 document 后可用于“批量变更”；分享该文件可能暴露敏感配置。")}</p><a href={download.changeUrl} download={state.file.changeName}>{t("Download editable change package","下载可编辑更新包")}</a></>}</>)}
      </>}
    </>}
  </section>;
}
