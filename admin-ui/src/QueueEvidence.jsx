import React,{useEffect,useState} from "react";
import {queueEvidence} from "./queue-evidence.mjs";

export function QueueEvidence({state,language}) {
  const [download,setDownload]=useState(null);
  useEffect(()=>{
    try {
      const url=URL.createObjectURL(new Blob([queueEvidence(state)],{type:"application/json"}));
      setDownload({state,url});
      return()=>URL.revokeObjectURL(url);
    } catch {setDownload({state,error:true});}
  },[state]);
  const zh=language==="zh";
  return <section aria-label={zh?"编辑器证据下载":"Editor evidence download"}>
    <p>{zh?"保存当前草稿、请求及已读取证据，不会发送请求或解除写入锁定。文件含配置和审计数据，请妥善保管；清除会话不会删除下载文件。它不是完整历史或结果证明，不可用于自动重放。":"Saves this draft, request and already-read evidence without sending requests or unlocking writes. Contains configuration and audit data; store securely. Clearing the session does not delete downloaded files. Not complete history or outcome proof; not for automatic replay."}</p>
    {download?.state===state&&(download.error?<p role="alert">{zh?"无法准备证据文件；当前内存证据未被清除。":"Could not prepare evidence file; in-memory evidence has not been cleared."}</p>:<a href={download.url} download="rjs-queue-editor-evidence.json">{zh?"下载编辑器证据 JSON":"Download editor evidence JSON"}</a>)}
  </section>;
}
