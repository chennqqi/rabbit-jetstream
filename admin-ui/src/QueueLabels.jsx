import React,{useId,useLayoutEffect,useRef,useState} from "react";
import {editQueueLabel} from "./queue-labels.mjs";
import {stringifyJSON} from "./api.mjs";

export function QueueLabels({model,state,language,editable,rules}) {
  const zh=language==="zh",t=(en,cn)=>zh?cn:en,id=useId(),inputs=useRef(new Map()),add=useRef(null),focus=useRef(null);
  const [error,setError]=useState(null);
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
  function name(kind,key=""){
    const value=window.prompt(t("Label key (exact text; existing keys cannot be overwritten)","标签键（精确文本；不能覆盖已有键）"),key);
    if(value!==null)change(kind==="add"?{kind,key:value}:{kind,key,value});
  }
  return <fieldset className="queue-structured-fields" disabled={!editable}><legend>{t("Queue labels","Queue 标签")}</legend>
    {rules&&<p>{t("Schema label values: text. Length/count rules are server-validated; invalid drafts remain editable.","Schema 标签值：文本。长度/数量规则由服务端验证；无效草稿仍可编辑。")}
      {rules.value.minLength!==undefined&&<> {t("Minimum value length: ","值最小长度：")}{rules.value.minLength}.</>}
      {rules.value.maxLength!==undefined&&<> {t("Maximum value length: ","值最大长度：")}{rules.value.maxLength}.</>}
      {rules.minimum!==undefined&&<> {t("Minimum labels: ","最少标签数：")}{rules.minimum}.</>}
      {rules.maximum!==undefined&&<> {t("Maximum labels: ","最多标签数：")}{rules.maximum}.</>}
    </p>}
    {rules&&<details><summary>{t("Schema label field rules","Schema 标签字段规则")}</summary><pre className="declaration-json">{stringifyJSON(rules)}</pre></details>}
    <p>{t("Keys are exact and case-sensitive. Empty values and multiline text are preserved. Add/rename refuses duplicate keys; removal requires confirmation. Changes affect only this draft until preview and confirmed apply.","键精确匹配且区分大小写，保留空值及多行文本。新增/重命名拒绝重复键，删除须确认。重新预览并确认提交前，仅改变此草稿。")}</p>
    {error?.raw===state.raw&&<p role="alert">{error.code==="duplicate-label"?t("That label key already exists; no label was overwritten.","该标签键已存在，未覆盖任何标签。"):t("Label edit was rejected; the draft is unchanged.","标签编辑被拒绝，草稿未改变。")}</p>}
    {Object.entries(state.draft.metadata.labels??{}).map(([key,value],index)=><div key={key}>
      <label htmlFor={`${id}-${index}`}>{t("Label value:","标签值：")} <code>{key===""?t("(empty key)","（空键）"):key}</code></label>
      <textarea id={`${id}-${index}`} ref={element=>{if(element)inputs.current.set(key,element);else inputs.current.delete(key);}} value={value} spellCheck="false" onChange={event=>change({kind:"value",key,value:event.target.value})}/>
      <button type="button" onClick={()=>name("rename",key)}>{t("Rename label:","重命名标签：")} {key||t("(empty key)","（空键）")}</button>
      <button type="button" onClick={()=>{if(window.confirm(t(`Remove label ${JSON.stringify(key)} from this draft?`,`从草稿移除标签 ${JSON.stringify(key)}？`)))change({kind:"remove",key},true);}}>{t("Remove label:","移除标签：")} {key||t("(empty key)","（空键）")}</button>
    </div>)}
    <button type="button" ref={add} onClick={()=>name("add")}>{t("Add label","添加标签")}</button>
  </fieldset>;
}
