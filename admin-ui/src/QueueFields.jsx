import React from "react";
import {queueFields,queueFieldValue,editQueueField,supportsQueueFields} from "./queue-fields.mjs";
import {QueueRouting} from "./QueueRouting.jsx";
import {QueueLabels} from "./QueueLabels.jsx";

export function QueueFields({model,state,language,editable}) {
  const zh=language==="zh",schemaRequired=state.form&&state.form.phase!=="legacy";
  const formReady=!schemaRequired||state.form.phase==="ready";
  const fields=schemaRequired?(state.form.fields??[]):queueFields;
  const supported=formReady&&!state.validation&&supportsQueueFields(state.draft);
  return <details><summary>{zh?"结构化配置字段":"Structured configuration fields"}</summary>
    {schemaRequired&&<>
      <button type="button" disabled={!editable||state.form.phase==="loading"} onClick={()=>void model.loadForm({refresh:true})}>{zh?"重新读取表单 Schema":"Reload form schema"}</button>
      {state.form.phase==="loading"&&<p role="status">{zh?"正在读取表单规则；JSON 草稿保留。":"Reading form rules; JSON draft retained."}</p>}
      {state.form.phase==="error"&&<p role="status">{zh?"表单 Schema 不可用或已变化。结构化编辑已关闭；原始 JSON 保留，可明确重新读取。":"Form schema unavailable or changed. Structured editing is unavailable; original JSON is retained. Reload explicitly."}</p>}
      {formReady&&<p>{zh?"控件类型、选项与范围来自已核对的服务端 Schema。":"Control types, choices and bounds come from the verified server schema."}</p>}
    </>}
    <p>{zh?"与下方 JSON 共用草稿；修改后必须重新预览并确认。留空表示省略，不等于零；不在客户端填入默认值。优先级省略与显式 0 不同。保留策略的 0 表示不设置该上限。时长示例 30s，容量示例 10MiB。服务端预览验证格式、范围及依赖，不代表部署资格或死信目标可用。":"Shares the JSON draft below; edits require a fresh preview and confirmation. Blank means omitted, not zero; no client defaults are inserted. Omitted priority differs from explicit 0. A retention limit of 0 leaves that limit unset. Duration example: 30s; size example: 10MiB. Server preview validates format, range and dependencies, not deployment qualification or DLQ availability."}</p>
    <p>{zh?"Queue 名称保持不变。标签与路由字段和专家 JSON 共用同一草稿。":"Queue name remains immutable. Labels, routing fields and expert JSON share one draft."}</p>
    {supported&&<QueueLabels model={model} state={state} language={language} editable={editable} rules={state.form?.labels}/>}
    {supported&&<QueueRouting model={model} state={state} language={language} editable={editable} types={state.form?.routingTypes} rules={state.form?.routing}/>}
    {!supported?<p role="status" hidden={!formReady}>{zh?"JSON 无效、版本/字段未知或结构无法表示；结构化编辑已禁用，原始 JSON 未被转换或丢弃。":"Invalid JSON, unknown version/fields or an unrepresentable structure: structured editing is disabled; original JSON has not been converted or discarded."}</p>:
      <fieldset className="queue-structured-fields" disabled={!editable}><legend>{zh?"存储、保留与投递":"Storage, retention and delivery"}</legend>
        {fields.map(field=>{
          const value=queueFieldValue(state.draft,field),id=`queue-field-${field.key}`;
          const change=event=>model.edit(editQueueField(state.draft,field.key,event.target.value,fields));
          return <div key={field.key}><label htmlFor={id}>{zh?field.zh:field.en}</label>
            {field.options?<select id={id} value={value} onChange={change}>
              <option value="">{zh?"省略（由服务端验证）":"Omitted (server validated)"}</option>
              {value&&!field.options.includes(value)&&<option value={value}>{value}</option>}
              {field.options.map(option=><option key={option} value={option}>{option}</option>)}
            </select>:<input id={id} value={value} onChange={change} spellCheck="false"/>}
            {schemaRequired&&<small>{[
              field.required?(zh?"必填":"Required"):null,
              field.minimum!==undefined?(zh?"最小值：":"Minimum: ")+String(field.minimum):null,
              field.maximum!==undefined?(zh?"最大值：":"Maximum: ")+String(field.maximum):null,
              field.defaultValue!==undefined?(zh?"省略时默认值（不自动填充）：":"Omitted default (not inserted): ")+String(field.defaultValue):null
            ].filter(Boolean).join(" · ")}</small>}
          </div>;
        })}
      </fieldset>}
  </details>;
}
