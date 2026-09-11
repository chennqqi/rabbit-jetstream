import React,{useEffect,useState} from "react";
import {deleteEvidence} from "./delete-evidence.mjs";

export function DeleteEvidence({state,language}){
  const [download,setDownload]=useState(null),zh=language==="zh";
  useEffect(()=>{
    try{
      const url=URL.createObjectURL(new Blob([deleteEvidence(state)],{type:"application/json"}));
      setDownload({state,url});return()=>URL.revokeObjectURL(url);
    }catch{setDownload({state,error:true});}
  },[state]);
  return <section aria-label={zh?"删除证据下载":"Deletion evidence download"}>
    <p>{zh?"下载请求、预检及已读取的证据，不发送请求或解锁重试。包含配置与审计数据，请妥善保管；清除会话不会删除下载文件。不是完整历史、结果证明或自动重放输入。":"Downloads request, preflight and already-read evidence without requests or unlocking retry. Contains configuration and audit data; store securely. Clearing the session does not delete downloaded files. Not complete history, outcome proof or automatic replay input."}</p>
    {download?.state===state&&(download.error?<p role="alert">{zh?"证据文件准备失败，内存中的证据仍保留。":"Evidence download unavailable; in-memory evidence is retained."}</p>:<a href={download.url} download="rjs-queue-delete-evidence.json">{zh?"下载删除证据 JSON":"Download deletion evidence JSON"}</a>)}
  </section>;
}
