import React from "react";
import {queueTemplates, templateQueueDocument} from "./queue-templates.mjs";
import {stringifyJSON} from "./api.mjs";
import {queueTemplatesLabels, queueTemplateName} from "./queue-templates-labels.mjs";

export function QueueTemplates({form, change, controls, language}) {
  const text = queueTemplatesLabels(language);
  let document = null;
  if (controls && form.templateId) { try { document = templateQueueDocument(form, controls); } catch {} }
  return <fieldset><legend>{text.legend}</legend><p>{text.description}</p>
    <label htmlFor="create-template">{text.template}</label>
    <select id="create-template" value={form.templateId ?? ""} onChange={event => change("templateId", event.target.value)}><option value="">{text.none}</option>{queueTemplates.map(item => <option key={item.id} value={item.id}>{queueTemplateName(item, language)}</option>)}</select>
    {form.templateId === "basic" && <p>{text.basic}</p>}
    {form.templateId === "priority" && <><label htmlFor="create-template-priority">{text.priority}</label><input id="create-template-priority" inputMode="numeric" required value={form.templatePriority ?? ""} onChange={event => change("templatePriority", event.target.value)}/><p>{text.priorityNote}</p></>}
    {form.templateId === "dead-letter" && <><label htmlFor="create-template-dlq">{text.dlq}</label><input id="create-template-dlq" required value={form.templateDLQ ?? ""} onChange={event => change("templateDLQ", event.target.value)}/><p>{text.dlqNote}</p></>}
    {form.templateId && <details><summary>{text.review}</summary>{document ? <pre className="declaration-json">{stringifyJSON(document)}</pre> : <p>{text.incomplete}</p>}</details>}
  </fieldset>;
}
