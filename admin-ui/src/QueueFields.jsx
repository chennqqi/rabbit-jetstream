import React from "react";
import {queueFields, queueFieldValue, editQueueField, supportsQueueFields} from "./queue-fields.mjs";
import {QueueRouting} from "./QueueRouting.jsx";
import {QueueLabels} from "./QueueLabels.jsx";
import {queueFieldsLabels, queueFieldLabel} from "./queue-fields-labels.mjs";

export function QueueFields({model, state, language, editable}) {
  const text = queueFieldsLabels(language);
  const schemaRequired = state.form && state.form.phase !== "legacy";
  const formReady = !schemaRequired || state.form.phase === "ready";
  const fields = schemaRequired ? (state.form.fields ?? []) : queueFields;
  const supported = formReady && !state.validation && supportsQueueFields(state.draft);
  return <details><summary>{text.title}</summary>
    {schemaRequired && <>
      <button type="button" disabled={!editable || state.form.phase === "loading"} onClick={() => void model.loadForm({refresh: true})}>{text.reload}</button>
      {state.form.phase === "loading" && <p role="status">{text.loading}</p>}
      {state.form.phase === "error" && <p role="status">{text.error}</p>}
      {formReady && <p>{text.ready}</p>}
    </>}
    <p>{text.guide}</p><p>{text.immutable}</p>
    {supported && <QueueLabels model={model} state={state} language={language} editable={editable} rules={state.form?.labels}/>}
    {supported && <QueueRouting model={model} state={state} language={language} editable={editable} types={state.form?.routingTypes} rules={state.form?.routing}/>}
    {!supported ? <p role="status" hidden={!formReady}>{text.unsupported}</p> :
      <fieldset className="queue-structured-fields" disabled={!editable}><legend>{text.legend}</legend>
        {fields.map(field => {
          const value = queueFieldValue(state.draft, field), id = `queue-field-${field.key}`;
          const change = event => model.edit(editQueueField(state.draft, field.key, event.target.value, fields));
          const hints = [field.required ? text.required : null, field.minimum !== undefined ? text.minimum + String(field.minimum) : null, field.maximum !== undefined ? text.maximum + String(field.maximum) : null, field.defaultValue !== undefined ? text.defaultValue + String(field.defaultValue) : null].filter(Boolean);
          return <div key={field.key}><label htmlFor={id}>{queueFieldLabel(field, language)}</label>
            {field.options ? <select id={id} value={value} onChange={change}><option value="">{text.omitted}</option>{value && !field.options.includes(value) && <option value={value}>{value}</option>}{field.options.map(option => <option key={option} value={option}>{option}</option>)}</select> : <input id={id} value={value} onChange={change} spellCheck="false"/>}
            {schemaRequired && <small>{hints.join(text.separator)}</small>}
          </div>;
        })}
      </fieldset>}
  </details>;
}
