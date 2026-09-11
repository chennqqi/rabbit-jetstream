import React,{useEffect,useMemo,useState} from "react";
import {dlqEvidence} from "./dlq-evidence.mjs";

export function DLQEvidence({plan,state,declarationETag,declarationReadAt,language}){
  const [download,setDownload]=useState(null),zh=language==="zh";
  const content=useMemo(()=>{try{return dlqEvidence({plan,state,declarationETag,declarationReadAt});}catch{return null;}},[plan,state,declarationETag,declarationReadAt]);
  useEffect(()=>{
    if(content===null){setDownload(null);return;}
    try{const url=URL.createObjectURL(new Blob([content],{type:"application/json"}));setDownload({content,url});return()=>URL.revokeObjectURL(url);}catch{setDownload({content,error:true});}
  },[content]);
  return <section aria-label={zh?"DLQ 证据下载":"DLQ evidence download"}>
    <p>{zh?"仅保存已读取的诊断快照，不发送请求。不是逐 Queue 转移历史或结果证明。文件包含运行标识与配置元数据；清除会话不会删除已下载文件。":"Saves only the already-read diagnostic snapshot; sends no requests. Not per-Queue transfer history or outcome proof. Contains operational identifiers and configuration metadata; clearing the session does not delete downloaded files."}</p>
    {content===null||download?.content===content&&download.error?<p role="alert">{zh?"无法准备证据文件，当前页面证据未清除。":"Could not prepare evidence file; current page evidence was not cleared."}</p>:download?.content===content&&<a href={download.url} download="rjs-dlq-diagnostic-evidence.json">{zh?"下载 DLQ 证据 JSON":"Download DLQ evidence JSON"}</a>}
  </section>;
}
