import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createBatchImport} from "./batch-import.mjs";

export function BatchImport({api,binding,language,onStart,retained=false}){
  const model=useMemo(()=>createBatchImport(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>()=>model.clear(),[model]);useEffect(()=>model.invalidatePlan(),[model,binding]);
  const t=(en,zh)=>language==="zh"?zh:en,result=state.binding===binding?state.result:null;
  const [reviewed,setReviewed]=useState(null);
  const problems={invalid_declaration:t("Invalid declaration","声明无效"),duplicate_queue:t("Duplicate Queue name","Queue 名称重复"),external_dependency_unverified:t("External dependency not validated","外部依赖未验证"),dependency_cycle:t("Dependency cycle member","依赖循环成员"),blocked_dependency:t("Blocked by prerequisite","被前置依赖阻塞")};
  const failures={bounds:t("Select 1–100 nonempty files, at most 1 MiB each and 4 MiB total including the request envelope.","请选择 1–100 个非空文件，单个最多 1 MiB，含请求封装共最多 4 MiB。"),files:t("Some files are not valid UTF-8 JSON. None will be silently skipped.","部分文件不是有效 UTF-8 JSON，不会静默跳过。"),denied:t("Planning denied. Operator permission is required.","规划被拒绝，需要 operator 权限。"),changed:t("Capabilities changed. Reload the creation schema and plan again.","能力已变化，请重新读取创建 Schema 后规划。"),invalid:t("Invalid planning evidence; no order is shown.","规划证据无效，不展示顺序。"),unavailable:t("Planning unavailable. Retry explicitly; this is not an empty result.","规划不可用，请手动重试；这不是空结果。")};
  return <section className="queue-import" aria-label={t("Batch import planning","批次导入规划")}>
    <h3>{t("Plan multiple Queue files","规划多个 Queue 文件")}</h3>
    <p>{t("Select files locally, then explicitly send their declarations for planning. File names stay local. This step does not create Queues. An order is not write authorization; external declaration presence is not live resource health. Per-item execution requires a separate review of these files and each Queue preview.","先在本地选择文件，再显式发送声明进行规划，文件名不发送。本步不创建 Queue，顺序不代表写入授权，外部声明存在不代表实时资源健康。逐项执行需要独立审阅这些文件和每个 Queue 的预览。")}</p>
    <label>{t("Queue JSON files","Queue JSON 文件集")}<input type="file" multiple accept="application/json,.json" onChange={event=>{void model.read(event.target.files);event.target.value="";}}/></label>
    {state.files.length>0&&<ol>{state.files.map((file,index)=><li key={index}>{file.name} — {file.status==="loaded"?t("JSON read; semantics unchecked","已读 JSON，语义待检"):file.status==="invalid"?t("Invalid file","文件无效"):t("Not read","未读取")}</li>)}</ol>}
    <button type="button" disabled={!binding||!state.files.length||state.files.some(file=>file.status!=="loaded")||state.phase==="planning"} onClick={()=>void model.plan(binding)}>{t("Plan import only","仅规划导入")}</button>
    {state.phase==="reading"||state.phase==="planning"?<p role="status">{t("Working…","处理中…")}</p>:null}
    {state.files.length>0&&<button type="button" onClick={()=>model.clear()}>{t("Clear batch files and plan","清除批次文件与规划")}</button>}
    {state.failure&&<p role="alert">{failures[state.failure]}</p>}
    {result&&<>
      <p role="status">{result.ready?t("Internally ordered only; destination preview and explicit apply are still required.","仅声明内部已排序，仍需目标端预览和显式提交。"):t("Some items are blocked. Internal order excludes unresolved dependencies; preview order is not readiness or apply approval.","部分项被阻塞。内部顺序排除未决依赖；预览顺序不代表就绪或批准提交。")}</p>
      <ol>{result.items.map(item=><li key={item.index}><strong>{state.files[item.index].name}</strong> — Queue: {item.queue||t("Unknown","未知")}
        {item.problems.length?<ul>{item.problems.map((problem,index)=><li key={index}>{problems[problem.code]} {problem.dependency??""}</li>)}</ul>:<p>{t("No internal planning problem; not an apply approval.","无内部规划问题，不代表批准提交。")}</p>}
      </li>)}</ol>
      <h4>{t("Dependency-first order","依赖优先顺序")}</h4><ol>{result.order.map(index=><li key={index}>{result.items[index].queue} — {state.files[index].name}</li>)}</ol>
      <h4>{t("Destination preview order — not approval","目标端预览顺序——非批准")}</h4><ol>{(result.review_order??result.order).map(index=><li key={index}>{result.items[index].queue} — {state.files[index].name}</li>)}</ol>
      <h4>{t("External declaration observations","外部声明观测")}</h4>{result.external.length?<ul>{result.external.map(item=><li key={item.queue}>{item.queue} — {({present:t("Declaration present","声明存在"),missing:t("Declaration missing","声明缺失"),unavailable:t("Unavailable","不可用"),unrepresentable:t("Cannot reconstruct","不可还原")})[item.status]} {item.etag??""}</li>)}</ul>:<p>{t("No external references in this plan.","本规划没有外部引用。")}</p>}
      {onStart&&<>
        <label><input type="checkbox" checked={reviewed===result} onChange={event=>setReviewed(event.target.checked?result:null)}/>{t("I reviewed the files and blocking problems. Prepare a session batch, without applying anything.","我已审阅文件和阻塞问题，准备会话批次，但不提交任何资源。")}</label>
        <button type="button" disabled={retained||reviewed!==result||!(result.review_order??result.order).length} onClick={()=>onStart(state)}>{t("Prepare per-item review","准备逐项审阅")}</button>
        {retained&&<p>{t("A batch is already retained in this session. Resume it above; do not replace its request evidence.","本会话已有保留批次，请从上方恢复，不替换请求证据。")}</p>}
      </>}
    </>}
  </section>;
}
