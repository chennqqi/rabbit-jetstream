import {newQueueDocument} from "./queue-create.mjs";

export const queueTemplates=Object.freeze([
  Object.freeze({id:"basic",en:"Subject-based Queue",zh:"Subject Queue"}),
  Object.freeze({id:"priority",en:"Priority Queue",zh:"优先级 Queue"}),
  Object.freeze({id:"dead-letter",en:"Queue with a DLQ target",zh:"带 DLQ 目标的 Queue"}),
]);

// Templates are authoring conveniences, never deployment defaults or preview
// authorization. Each invocation creates a fresh document from explicit inputs.
export function templateQueueDocument(form,controls){
  const id=form.templateId||"basic";
  if(!queueTemplates.some(template=>template.id===id))throw Error("template");
  const document=newQueueDocument(form,controls);
  if(id==="priority"){
    if(typeof form.templatePriority!=="string"||!/^\d+$/.test(form.templatePriority)||BigInt(form.templatePriority)<1n||BigInt(form.templatePriority)>255n)throw Error("template-priority");
    document.spec.maxPriority=Number(form.templatePriority);
  }
  if(id==="dead-letter"){
    if(typeof form.templateDLQ!=="string"||!/^[A-Za-z0-9_-]{1,256}$/.test(form.templateDLQ)||form.templateDLQ===document.metadata.name)throw Error("template-dlq");
    document.spec.deadLetter={queue:form.templateDLQ};
  }
  return document;
}
