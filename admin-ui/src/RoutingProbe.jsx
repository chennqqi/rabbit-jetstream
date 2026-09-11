import React,{useEffect,useMemo,useState,useSyncExternalStore} from "react";
import {createRoutingProbe} from "./routing-probe.mjs";
import {routingLabels} from "./routing-labels.mjs";

export function RoutingProbe({api,name,revision,etag,language}){
  const model=useMemo(()=>createRoutingProbe(api,name,revision,etag),[api,name,revision,etag]);
  const state=useSyncExternalStore(model.subscribe,model.snapshot),[mode,setMode]=useState("subject"),[input,setInput]=useState({subject:"",exchange:"",type:"direct",routingKey:""});
  useEffect(()=>()=>model.clear(),[model]);
  const t=(en,zh)=>language==="zh"?zh:en,labels=routingLabels(language);
  function edit(key,value){model.clear();setInput(old=>({...old,[key]:value,...(key==="type"&&value==="fanout"?{routingKey:""}:{})}));}
  const errors={denied:t("Read denied. Check credentials and permissions.","读取被拒绝，请检查凭据与权限。"),disabled:t("Resource reads are disabled.","资源读取未启用。"),missing:t("Queue declaration not found.","Queue 声明不存在。"),changed:t("Declaration changed or cannot be reconstructed. Refresh the Queue page before retrying.","声明已变化或无法完整还原。请刷新 Queue 页面后重试。"),query:t("Invalid or unsupported probe input. Priority Queues require a literal priority Subject.","输入无效或不受支持。优先级 Queue 请使用字面优先级 Subject。"),limit:t("Routing evidence exceeds the response limit; no partial result is shown.","路由证据超过响应上限；不展示部分结果。"),invalid:t("Invalid response; no routing conclusion is available.","响应无效，无法得出路由结论。"),unavailable:t("Read unavailable. Retry explicitly; this does not mean no match.","读取不可用，请手动重试；这不代表没有匹配。")};
  const result=state.result,list=items=>items.length?<ul>{items.map((value,i)=><li key={i}><code>{value}</code></li>)}</ul>:t("None","无");
  return <section className="routing-probe" aria-label={t("Routing probe","路由探测")}>
    <h3>{t("Test routing without publishing","不发布消息的路由检查")}</h3>
    <p>{t("Checks the saved declaration, not editor drafts or live delivery. Binding and generated Stream matches are separate. No message is sent. Runs only on request; changing input clears prior evidence.","检查已保存声明，不检查编辑草稿或实际投递。绑定匹配与生成的 Stream 匹配分别展示，不发送消息。仅手动执行，修改输入会清除此前证据。")}</p>
    <form onSubmit={event=>{event.preventDefault();void model.run(mode==="subject"?{subject:input.subject}:{exchange:input.exchange,type:input.type,routingKey:input.routingKey});}}>
      <label>{t("Probe mode","探测模式")}<select aria-label={t("Probe mode","探测模式")} value={mode} onChange={event=>{model.clear();setMode(event.target.value);}}><option value="subject">{t("Literal Subject","字面 Subject")}</option><option value="exchange">{t("Exchange target","交换机目标")}</option></select></label>
      {mode==="subject"?<label>Subject<input required value={input.subject} onChange={event=>edit("subject",event.target.value)} spellCheck="false"/></label>:<>
        <label>{labels.exchange}<input required value={input.exchange} onChange={event=>edit("exchange",event.target.value)} spellCheck="false"/></label>
        <label>{t("Exchange type","交换机类型")}<select aria-label={t("Exchange type","交换机类型")} value={input.type} onChange={event=>edit("type",event.target.value)}>{["direct","topic","fanout"].map(type=><option key={type}>{type}</option>)}</select></label>
        <label>{t("Routing key","路由键")}<input required={input.type==="direct"} disabled={input.type==="fanout"} value={input.routingKey} onChange={event=>edit("routingKey",event.target.value)} spellCheck="false"/></label>
      </>}
      <button disabled={state.phase==="loading"}>{t("Check routing","检查路由")}</button>
      {state.phase==="loading"&&<button type="button" onClick={()=>model.clear()}>{t("Cancel","取消")}</button>}
    </form>
    {state.phase==="loading"&&<p role="status">{t("Checking saved declaration…","正在检查已保存声明…")}</p>}
    {state.failure&&<p role="alert">{errors[state.failure]}</p>}
    {result&&<>
      <p role="status">{result.matchedStreamSubjects.length?t("Generated Stream Subject matched. This is not proof of delivery.","生成的 Stream Subject 匹配；这不是投递成功的证据。"):t("No generated Stream Subject matched. Binding matches, if any, do not override this result.","没有生成的 Stream Subject 匹配。即使绑定匹配，也不改变此结果。")}</p>
      <dl><dt>Queue</dt><dd>{result.queue}</dd><dt>{t("Declaration revision","声明修订")}</dt><dd>{result.revision}</dd><dt>{t("Read completed (local time)","读取完成（本机时间）")}</dt><dd><time dateTime={state.readAt}>{state.readAt}</time></dd><dt>{t("Resolved Subject","解析后的 Subject")}</dt><dd>{result.subject}</dd><dt>Stream</dt><dd>{result.stream}</dd><dt>{t("Generated Stream Subjects","生成的 Stream Subjects")}</dt><dd>{list(result.streamSubjects)}</dd><dt>{t("Matched Stream Subjects","匹配的 Stream Subjects")}</dt><dd>{list(result.matchedStreamSubjects)}</dd></dl>
      {result.bindings.length?<div className="table-scroll" role="region" tabIndex={0} aria-label={t("Binding results — scroll horizontally","绑定结果——可横向滚动")}><table><caption>{t("Binding translation and matches","绑定翻译与匹配")}</caption><thead><tr>{[labels.exchange,labels.type,labels.keys,labels.generatedSubjects,labels.matchedSubjects].map(label=><th key={label} scope="col">{label}</th>)}</tr></thead><tbody>{result.bindings.map((row,i)=><tr key={i}><td>{row.exchange}</td><td>{row.type}</td><td>{list(row.keys)}</td><td>{list(row.subjects)}</td><td>{list(row.matchedSubjects)}</td></tr>)}</tbody></table></div>:<p>{t("No declared exchange bindings.","没有声明的交换机绑定。")}</p>}
    </>}
  </section>;
}
