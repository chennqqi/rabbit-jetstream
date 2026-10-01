import React,{useEffect,useState} from "react";
import {stringifyJSON} from "./api.mjs";
import {batchPhaseLabel} from "./batch-phase.mjs";
import {batchEvidence} from "./batch-evidence.mjs";
import {batchHistoryLabels} from "./batch-history-labels.mjs";

function BatchRecord({record,index,language}){
  const [download,setDownload]=useState(null),text=batchHistoryLabels(language);
  useEffect(()=>{
    try{
      const url=URL.createObjectURL(new Blob([batchEvidence(record)],{type:"application/json"}));
      setDownload({record,url});
      return()=>URL.revokeObjectURL(url);
    }catch{setDownload({record,error:true});}
  },[record]);
  return <details><summary>{text.batch} {index+1} · {record.archivedAt} · {record.items.length} {text.items}</summary>
    <p>{text.evidenceNote}</p>
    {download?.record===record&&(download.error?<p role="alert">{text.prepareFailed}</p>:<a href={download.url} download={`rjs-batch-evidence-${index+1}.json`}>{text.download}</a>)}
    <ol>{record.items.map(item=><li key={item.index}>{item.filename} · {item.queue||text.unknownQueue} · {batchPhaseLabel(item.outcome.phase,language)}{item.outcome.requestId&&<> · {text.requestId}: {item.outcome.requestId}</>}</li>)}</ol>
    <details><summary>{text.details}</summary><pre className="declaration-json">{stringifyJSON(record)}</pre></details>
  </details>;
}

export function BatchHistory({records,language}){
  if(!records?.length)return null;
  const text=batchHistoryLabels(language);
  return <section className="queue-import" aria-label={text.archive}>
    <h3>{text.title}</h3><p>{text.historyNote}</p>
    {records.map((record,index)=><BatchRecord key={record.archivedAt+index} record={record} index={index} language={language}/>) }
  </section>;
}
