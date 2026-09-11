import React from "react";
import {queueTemplates,templateQueueDocument} from "./queue-templates.mjs";
import {stringifyJSON} from "./api.mjs";

export function QueueTemplates({form,change,controls,language}){
  const t=(en,zh)=>language==="zh"?zh:en;
  let document=null;
  if(controls&&form.templateId){try{document=templateQueueDocument(form,controls);}catch{}}
  return <fieldset>
    <legend>{t("Declaration templates (local drafts)","声明模板（本地草稿）")}</legend>
    <p>{t("Templates only add the fields shown below. Choose name, Subjects, replicas, storage and message limit explicitly above. No resource is created by selecting or reviewing a template; server preview and confirmation are still required.","模板仅添加下面列出的字段。请在上方明确填写名称、Subject、副本、存储及消息上限。选择或审阅模板不会创建资源，仍须服务端预览并确认。")}</p>
    <label htmlFor="create-template">{t("Template","模板")}</label>
    <select id="create-template" value={form.templateId??""} onChange={event=>change("templateId",event.target.value)}>
      <option value="">{t("No template","不使用模板")}</option>
      {queueTemplates.map(item=><option key={item.id} value={item.id}>{language==="zh"?item.zh:item.en}</option>)}
    </select>
    {form.templateId==="basic"&&<p>{t("Uses the explicit Subject-based settings above; no additional fields.","使用上方明确填写的 Subject 配置，不添加其他字段。")}</p>}
    {form.templateId==="priority"&&<>
      <label htmlFor="create-template-priority">{t("Maximum priority (1–255)","最高优先级（1–255）")}</label>
      <input id="create-template-priority" inputMode="numeric" required value={form.templatePriority??""} onChange={event=>change("templatePriority",event.target.value)}/>
      <p>{t("Adds spec.maxPriority. Priority generates multiple Consumers; it is not a client processing-order guarantee. Review the generated plan and resource cost.","添加 spec.maxPriority。优先级会生成多个 Consumer，不保证客户端处理顺序；请审阅生成计划及资源开销。")}</p>
    </>}
    {form.templateId==="dead-letter"&&<>
      <label htmlFor="create-template-dlq">{t("Existing DLQ Queue name","已有 DLQ Queue 名称")}</label>
      <input id="create-template-dlq" required value={form.templateDLQ??""} onChange={event=>change("templateDLQ",event.target.value)}/>
      <p>{t("Adds spec.deadLetter.queue, not the target itself. Server preview must verify the target and dependency chain. Delivery defaults are resolved by the server; this is not transfer evidence.","添加 spec.deadLetter.queue，不创建目标。服务端预览须核验目标及依赖链。投递默认值由服务端解析；模板不构成转移证据。")}</p>
    </>}
    {form.templateId&&<details><summary>{t("Review generated template document","审阅生成的模板文档")}</summary>
      {document?<pre className="declaration-json">{stringifyJSON(document)}</pre>:<p>{t("Complete valid explicit settings and load the creation schema to display this document. No defaults are substituted.","请完整填写有效配置并读取创建 Schema 后查看文档，不会代填默认值。")}</p>}
    </details>}
  </fieldset>;
}
