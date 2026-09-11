import React from "react";
import {routingMode,routingNeedsConfirmation,editQueueRouting} from "./queue-routing.mjs";
import {stringifyJSON} from "./api.mjs";

export function QueueRouting({model,state,language,editable,types=["direct","topic","fanout"],rules}) {
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,mode=routingMode(state.draft),spec=state.draft.spec;
  function change(action){
    if(!editable)return;
    const needs=routingNeedsConfirmation(state.draft,action);
    if(needs&&!window.confirm(t("This removes routing content from the local draft. It does not change the server until a new preview and confirmed apply. Continue?","这会从本地草稿移除路由内容；重新预览并确认提交前不会改变服务端。继续？")))return;
    model.edit(editQueueRouting(state.draft,action,{confirmed:needs}));
  }
  const button=(label,action)=><button type="button" onClick={()=>change(action)}>{label}</button>;
  return <fieldset className="queue-structured-fields" disabled={!editable}><legend>{t("Queue routing","Queue 路由")}</legend>
    {rules&&<p>{t("Schema routing: choose one collection. Minimum Subjects: ","Schema 路由：二选一。最少 Subject 数：")}{rules.subjects.minimum}
      {t("; minimum Bindings: ","；最少 Binding 数：")}{rules.bindings.minimum}
      {t(". Subject and key duplicates are rejected by server preview.", "。重复 Subject 和路由键由服务端预览拒绝。")}</p>}
    {rules&&<details><summary>{t("Schema routing field rules","Schema 路由字段规则")}</summary><pre className="declaration-json">{stringifyJSON(rules)}</pre></details>}
    <p>{t("Choose Subjects OR Bindings. Blank rows and duplicates are preserved for correction; server preview validates routing semantics. Mode changes clear the old routing after confirmation, without converting wildcard syntax.","选择 Subjects 或 Bindings。空行及重复项保留供修正，路由语义由服务端预览验证。模式切换经确认后清除旧路由，不自动转换通配符语法。")}</p>
    <div><label htmlFor="queue-routing-mode">{t("Routing mode","路由模式")}</label><select id="queue-routing-mode" value={mode} onChange={event=>change({kind:"mode",value:event.target.value})}>
      {mode==="mixed"&&<option value="mixed">{t("Conflicting modes — choose one","模式冲突——请选择一种")}</option>}
      <option value="subjects">Subjects</option><option value="bindings">Bindings</option>
    </select></div>
    {mode==="subjects"&&<>
      {(spec.subjects??[]).map((subject,index)=><div key={index}><label htmlFor={`queue-subject-${index}`}>{t("Subject","Subject")} {index+1}</label>
        <input id={`queue-subject-${index}`} value={subject} spellCheck="false" onChange={event=>change({kind:"subject",index,value:event.target.value})}/>
        {button(t(`Remove Subject ${index+1}`,`移除 Subject ${index+1}`),{kind:"remove-subject",index})}</div>)}
      {button(t("Add Subject","添加 Subject"),{kind:"add-subject"})}
    </>}
    {mode==="bindings"&&<>
      {(spec.bindings??[]).map((binding,index)=><fieldset className="queue-structured-fields" key={index}><legend>{t("Binding","绑定")} {index+1}</legend>
        {rules?.bindings.keys[binding.type]&&<p>{rules.bindings.keys[binding.type].maximum===0?t("Schema key count: 0.","Schema 路由键数量：0。"):
          t("Schema minimum key count: ","Schema 最少路由键数：")+rules.bindings.keys[binding.type].minimum}</p>}
        <div><label htmlFor={`binding-exchange-${index}`}>{t("Exchange","Exchange")} {index+1}</label><input id={`binding-exchange-${index}`} value={binding.exchange} spellCheck="false" onChange={event=>change({kind:"exchange",index,value:event.target.value})}/></div>
        <div><label htmlFor={`binding-type-${index}`}>{t("Binding type","绑定类型")} {index+1}</label><select id={`binding-type-${index}`} value={binding.type} onChange={event=>change({kind:"type",index,value:event.target.value})}>
          <option value="">{t("Choose explicitly","请明确选择")}</option>
          {binding.type&&!types.includes(binding.type)&&<option value={binding.type}>{binding.type}</option>}
          {types.map(type=><option key={type} value={type}>{type}</option>)}
        </select></div>
        {binding.type==="fanout"&&<p>{t("Fanout requires no routing keys. Remove any invalid keys retained from expert JSON.","Fanout 不允许路由键；请移除专家 JSON 中保留的无效键。")}</p>}
        {(binding.keys??[]).map((key,keyIndex)=><div key={keyIndex}><label htmlFor={`binding-key-${index}-${keyIndex}`}>{t("Routing key","路由键")} {index+1}.{keyIndex+1}</label>
          <input id={`binding-key-${index}-${keyIndex}`} value={key} disabled={binding.type==="fanout"} spellCheck="false" onChange={event=>change({kind:"key",index,keyIndex,value:event.target.value})}/>
          {button(t(`Remove key ${index+1}.${keyIndex+1}`,`移除路由键 ${index+1}.${keyIndex+1}`),{kind:"remove-key",index,keyIndex})}</div>)}
        {binding.type!=="fanout"&&button(t(`Add key to Binding ${index+1}`,`为绑定 ${index+1} 添加路由键`),{kind:"add-key",index})}
        {button(t(`Remove Binding ${index+1}`,`移除绑定 ${index+1}`),{kind:"remove-binding",index})}
      </fieldset>)}
      {button(t("Add Binding","添加绑定"),{kind:"add-binding"})}
    </>}
  </fieldset>;
}
