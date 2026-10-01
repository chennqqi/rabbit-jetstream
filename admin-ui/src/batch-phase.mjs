const labels=Object.freeze({
  en:Object.freeze({"not-prepared":"Not prepared",idle:"Not started",loading:"Loading declaration","load-error":"Declaration read failed",uneditable:"Declaration cannot be edited",editing:"Editing",review:"Awaiting separate confirmation",accepted:"Apply accepted; not health proof",uncertain:"Write outcome unknown",submitting:"Submitting once",previewing:"Previewing",blocked:"Preview blocked",conflict:"Conflict",denied:"Denied","preview-error":"Preview failed",archived:"Archived, read-only",inspecting:"Inspecting unknown outcome","reading-conflict":"Reading conflict evidence","reading-next":"Reading next edit base"}),
  zh:Object.freeze({"not-prepared":"未准备",idle:"未开始",loading:"读取声明中","load-error":"声明读取失败",uneditable:"声明不可编辑",editing:"编辑中",review:"待独立确认",accepted:"提交已接受，非健康证明",uncertain:"写入结果未知",submitting:"单次提交中",previewing:"预览中",blocked:"预览被阻塞",conflict:"冲突",denied:"拒绝","preview-error":"预览失败",archived:"已归档，只读",inspecting:"检查未知结果中","reading-conflict":"读取冲突证据中","reading-next":"读取下一次编辑基线中"}),
});
const unknown=Object.freeze({en:"Unknown state",zh:"未知状态"});

// Presentation only: never replace the machine phase in retained evidence.
export function batchPhaseLabel(phase,language){
  const catalog=labels[language]??labels.en,label=typeof phase==="string"&&Object.hasOwn(catalog,phase)?catalog[phase]:undefined;
  if(label)return label;
  const fallback=unknown[language]??unknown.en;
  return typeof phase==="string"&&phase?`${fallback} (${phase})`:fallback;
}
