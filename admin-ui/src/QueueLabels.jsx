import React,{useId,useLayoutEffect,useRef,useState} from "react";
import {editQueueLabel} from "./queue-labels.mjs";
import {stringifyJSON} from "./api.mjs";
import {useDialog,DialogHost} from "./dialog.jsx";
import {removeLabelPrompt} from "./queue-labels-formatters.mjs";
import {queueLabelsLabels} from "./queue-labels-labels.mjs";

export function QueueLabels({model,state,language,editable,rules}) {
  const zh=language==="zh",text=queueLabelsLabels(language),id=useId(),inputs=useRef(new Map()),add=useRef(null),focus=useRef(null);
  const [error,setError]=useState(null);
  const dialog=useDialog();
  useLayoutEffect(()=>{
    if(focus.current){const target=focus.current;focus.current=null;(target.add?add.current:inputs.current.get(target.key))?.focus();}
  },[state]);
  function change(action,confirmed=false){
    if(!editable)return;
    try {
      const raw=editQueueLabel(state.draft,action,{confirmed});
      setError(null);
      if(action.kind!=="value")focus.current=action.kind==="remove"?{add:true}:{key:action.kind==="rename"?action.value:action.key};
      model.edit(raw);
    }catch(reason){setError({raw:state.raw,code:reason.code});}
  }
  async function name(kind,key=""){
    const value=await dialog.prompt(text.label_key_exact_text_existing_keys_cannot,{defaultValue:key,inputLabel:text.label_key});
    if(value!==null)change(kind==="add"?{kind,key:value}:{kind,key,value});
  }
  return <><fieldset className="queue-structured-fields" disabled={!editable}><legend>{text.queue_labels}</legend>
    {rules&&<p>{text.schema_label_values_text_length_count_rules}
      {rules.value.minLength!==undefined&&<> {text.minimum_value_length}{rules.value.minLength}.</>}
      {rules.value.maxLength!==undefined&&<> {text.maximum_value_length}{rules.value.maxLength}.</>}
      {rules.minimum!==undefined&&<> {text.minimum_labels}{rules.minimum}.</>}
      {rules.maximum!==undefined&&<> {text.maximum_labels}{rules.maximum}.</>}
    </p>}
    {rules&&<details><summary>{text.schema_label_field_rules}</summary><pre className="declaration-json">{stringifyJSON(rules)}</pre></details>}
    <p>{text.keys_are_exact_and_case_sensitive_empty}</p>
    {error?.raw===state.raw&&<p role="alert">{error.code==="duplicate-label"?text.that_label_key_already_exists_no_label:text.label_edit_was_rejected_the_draft_is}</p>}
    {Object.entries(state.draft.metadata.labels??{}).map(([key,value],index)=><div key={key}>
      <label htmlFor={`${id}-${index}`}>{text.label_value} <code>{key===""?text.empty_key:key}</code></label>
      <textarea id={`${id}-${index}`} ref={element=>{if(element)inputs.current.set(key,element);else inputs.current.delete(key);}} value={value} spellCheck="false" onChange={event=>change({kind:"value",key,value:event.target.value})}/>
      <button type="button" onClick={()=>void name("rename",key)}>{text.rename_label} {key||text.empty_key}</button>
      <button type="button" onClick={()=>{void dialog.confirm(removeLabelPrompt(language,key),{tone:"danger"}).then(ok=>{if(ok)change({kind:"remove",key},true);});}}>{text.remove_label} {key||text.empty_key}</button>
    </div>)}
    <button type="button" ref={add} onClick={()=>void name("add")}>{text.add_label}</button>
  </fieldset><DialogHost dialog={dialog.dialog} language={language}/></>;
}
