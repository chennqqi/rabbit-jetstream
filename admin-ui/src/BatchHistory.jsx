import React,{useEffect,useState} from "react";
import {stringifyJSON} from "./api.mjs";
import {batchPhaseLabel} from "./batch-phase.mjs";
import {batchEvidence} from "./batch-evidence.mjs";

function BatchRecord({record,index,language}){
  const [download,setDownload]=useState(null),t=(en,zh)=>language==="zh"?zh:en;
  useEffect(()=>{
    try {
      const url=URL.createObjectURL(new Blob([batchEvidence(record)],{type:"application/json"}));
      setDownload({record,url});
      return()=>URL.revokeObjectURL(url);
    } catch {setDownload({record,error:true});}
  },[record]);
  return <details><summary>{t("Batch","批次")} {index+1} — {record.archivedAt} — {record.items.length} {t("items","项")}</summary>
    <p>{t("Versioned itemized evidence; contains full declarations and may contain sensitive labels. It is not a server outcome certificate, message backup, replay file or rollback plan.","版本化逐项证据；包含完整声明，可能含敏感标签。它不是服务端结果证书、消息备份、重放文件或回滚方案。")}</p>
    {download?.record===record&&(download.error?<p role="alert">{t("Could not prepare the local evidence file.","无法准备本地证据文件。")}</p>:<a href={download.url} download={`rjs-batch-evidence-${index+1}.json`}>{t("Download itemized batch evidence JSON","下载逐项批次证据 JSON")}</a>)}
    <ol>{record.items.map(item=><li key={item.index}>{item.filename} — {item.queue||t("Unknown Queue","未知 Queue")} — {batchPhaseLabel(item.outcome.phase,language)}{item.outcome.requestId&&<> — {t("Request ID","请求标识")}: {item.outcome.requestId}</>}</li>)}</ol>
    <details><summary>{t("Read-only files, plan and outcome evidence","只读文件、规划和结果证据")}</summary><pre className="declaration-json">{stringifyJSON(record)}</pre></details>
  </details>;
}

export function BatchHistory({records,language}){
  if(!records?.length)return null;
  const t=(en,zh)=>language==="zh"?zh:en;
  return <section className="queue-import" aria-label={t("Archived batches","已归档批次")}>
    <h3>{t("Archived batches (session memory)","已归档批次（会话内存）")}</h3>
    <p>{t("Historical snapshots, not current health or write authorization. No automatic rollback or retry. Reloading or clearing the session loses these records. Retained Queue names remain protected.","历史快照不代表当前健康状态或写入授权，不会自动回滚或重试。刷新页面或清除会话会丢失这些记录，保留的 Queue 名称仍受保护。")}</p>
    {records.map((record,index)=><BatchRecord key={record.archivedAt+index} record={record} index={index} language={language}/>)}
  </section>;
}
