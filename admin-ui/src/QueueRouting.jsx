import React from "react";
import {routingMode,routingNeedsConfirmation,editQueueRouting} from "./queue-routing.mjs";
import {stringifyJSON} from "./api.mjs";
import {useDialog,DialogHost} from "./dialog.jsx";
import {queueRoutingFormatters} from "./queue-routing-formatters.mjs";
import {queueRoutingLabels} from "./queue-routing-labels.mjs";

export function QueueRouting({model,state,language,editable,types=["direct","topic","fanout"],rules}) {
  const format=queueRoutingFormatters(language);
  const zh=language==="zh",text=queueRoutingLabels(language),mode=routingMode(state.draft),spec=state.draft.spec;
  const dialog=useDialog();
  async function change(action){
    if(!editable)return;
    const needs=routingNeedsConfirmation(state.draft,action);
    if(needs&&!await dialog.confirm(text.this_removes_routing_content_from_the_local,{tone:"danger"}))return;
    model.edit(editQueueRouting(state.draft,action,{confirmed:needs}));
  }
  const button=(label,action)=><button type="button" onClick={()=>void change(action)}>{label}</button>;
  const host=<DialogHost dialog={dialog.dialog} language={language}/>;
  return <><fieldset className="queue-structured-fields" disabled={!editable}><legend>{text.queue_routing}</legend>
    {rules&&<p>{text.schema_routing_choose_one_collection_minimum_subjects}{rules.subjects.minimum}
      {text.minimum_bindings}{rules.bindings.minimum}
      {text.subject_and_key_duplicates_are_rejected_by}</p>}
    {rules&&<details><summary>{text.schema_routing_field_rules}</summary><pre className="declaration-json">{stringifyJSON(rules)}</pre></details>}
    <p>{text.choose_subjects_or_bindings_blank_rows_and}</p>
    <div><label htmlFor="queue-routing-mode">{text.routing_mode}</label><select id="queue-routing-mode" value={mode} onChange={event=>change({kind:"mode",value:event.target.value})}>
      {mode==="mixed"&&<option value="mixed">{text.conflicting_modes_choose_one}</option>}
      <option value="subjects">Subjects</option><option value="bindings">Bindings</option>
    </select></div>
    {mode==="subjects"&&<>
      {(spec.subjects??[]).map((subject,index)=><div key={index}><label htmlFor={`queue-subject-${index}`}>{text.subject} {index+1}</label>
        <input id={`queue-subject-${index}`} value={subject} spellCheck="false" onChange={event=>change({kind:"subject",index,value:event.target.value})}/>
        {button(format.removeSubject(index+1),{kind:"remove-subject",index})}</div>)}
      {button(text.add_subject,{kind:"add-subject"})}
    </>}
    {mode==="bindings"&&<>
      {(spec.bindings??[]).map((binding,index)=><fieldset className="queue-structured-fields" key={index}><legend>{text.binding} {index+1}</legend>
        {rules?.bindings.keys[binding.type]&&<p>{rules.bindings.keys[binding.type].maximum===0?text.schema_key_count_0:
          text.schema_minimum_key_count+rules.bindings.keys[binding.type].minimum}</p>}
        <div><label htmlFor={`binding-exchange-${index}`}>{text.exchange} {index+1}</label><input id={`binding-exchange-${index}`} value={binding.exchange} spellCheck="false" onChange={event=>change({kind:"exchange",index,value:event.target.value})}/></div>
        <div><label htmlFor={`binding-type-${index}`}>{text.binding_type} {index+1}</label><select id={`binding-type-${index}`} value={binding.type} onChange={event=>change({kind:"type",index,value:event.target.value})}>
          <option value="">{text.choose_explicitly}</option>
          {binding.type&&!types.includes(binding.type)&&<option value={binding.type}>{binding.type}</option>}
          {types.map(type=><option key={type} value={type}>{type}</option>)}
        </select></div>
        {binding.type==="fanout"&&<p>{text.fanout_requires_no_routing_keys_remove_any}</p>}
        {(binding.keys??[]).map((key,keyIndex)=><div key={keyIndex}><label htmlFor={`binding-key-${index}-${keyIndex}`}>{text.routing_key} {index+1}.{keyIndex+1}</label>
          <input id={`binding-key-${index}-${keyIndex}`} value={key} disabled={binding.type==="fanout"} spellCheck="false" onChange={event=>change({kind:"key",index,keyIndex,value:event.target.value})}/>
          {button(format.removeKey(index+1,keyIndex+1),{kind:"remove-key",index,keyIndex})}</div>)}
        {binding.type!=="fanout"&&button(format.addKey(index+1),{kind:"add-key",index})}
        {button(format.removeBinding(index+1),{kind:"remove-binding",index})}
      </fieldset>)}
      {button(text.add_binding,{kind:"add-binding"})}
    </>}
  </fieldset>{host}</>;
}
