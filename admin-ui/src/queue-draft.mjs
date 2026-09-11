import {createQueueEditor} from "./queue-editor.mjs";
import {parseJSON,stringifyJSON} from "./api.mjs";
import {readMutationCapabilities,observedCapabilityChange} from "./mutation-capabilities.mjs";
import {compileQueueForm} from "./queue-form-schema.mjs";

// Session-owned draft controller. Retained across route changes, never stored.
export function createQueueDraft(api,name,{document: initialDocument,requireCapabilities=api.requireMutationCapabilities===true}={}) {
  const editor=createQueueEditor(api,{requireCapabilities}), listeners=new Set();
  let form={phase:requireCapabilities?"idle":"legacy"},formGeneration=0;
  const formEditable=()=>["editing","review","blocked","preview-error","conflict","denied"].includes(editor.snapshot().phase);
  const cancelFormLoad=()=>{if(form.phase==="loading"){formGeneration++;form={phase:"error"};}};
  let raw="", modified=false, validation=null, mergeRaw="", mergeValidation=null, mergeConfirmed=false, applyConfirmed=false, state;
  const resetMerge=()=>{mergeRaw="";mergeValidation=null;mergeConfirmed=false;applyConfirmed=false;};
  const emit=()=>{state={...editor.snapshot(),raw,modified,validation,mergeRaw,mergeValidation,mergeConfirmed,applyConfirmed,form};for(const fn of listeners)fn();};
  if(initialDocument){
    if(initialDocument.metadata?.name!==name)throw new Error("Create identity mismatch");
    editor.create(initialDocument);raw=stringifyJSON(editor.snapshot().draft);modified=true;
  }
  emit();
  const unsubscribeCapabilities=api.subscribeCapabilityReads?.(observation=>{
    let changed=false;
    if(form.phase==="ready"&&(formEditable()||["previewing","reading-conflict","reading-next"].includes(editor.snapshot().phase))&&observedCapabilityChange(form.binding,observation)){
      formGeneration++;form={phase:"error"};changed=true;
    }
    if(editor.invalidateCapabilities(observation)){applyConfirmed=false;changed=true;}
    if(changed)emit();
  });
  async function loadForm({refresh=false}={}){
    if(!requireCapabilities||!formEditable()||form.phase==="loading"||(!refresh&&form.phase!=="idle"))return;
    const current=++formGeneration;form={phase:"loading"};emit();
    try{
      const binding=await readMutationCapabilities(api,["queue-schema"]);
      const compiled=compileQueueForm(binding);
      if(current!==formGeneration)return;
      form={phase:"ready",binding,...compiled};emit();
    }catch(error){if(current===formGeneration){form={phase:"error",status:error.status};emit();}}
  }
  async function readInitial(retry=false){
    if(retry ? !["load-error","uneditable"].includes(state.phase)||state.draft||state.requestId : state.phase!=="idle")return;
    const pending=editor.load(name);emit();await pending;
    raw=editor.snapshot().draft ? stringifyJSON(editor.snapshot().draft) : "";emit();
  }
  return {
    snapshot:()=>state,
    subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    load:()=>readInitial(),
    retryLoad:async()=>{await readInitial(true);await loadForm();},
    loadForm,
    edit(value){
      if(!["editing","review","blocked","preview-error","conflict","denied"].includes(state.phase))return;
      raw=value;modified=true;validation=null;resetMerge();
      // Even invalid JSON invalidates the previous preview immediately.
      editor.edit(editor.snapshot().draft);
      try {editor.edit(parseJSON(raw));}catch {validation="invalid-draft";}
      emit();
    },
    async preview(){
      if(validation || !["editing","review","blocked","preview-error","conflict","denied"].includes(state.phase))return;
      cancelFormLoad();
      resetMerge();const pending=editor.preview();emit();await pending;emit();
      const binding=editor.snapshot().capabilities;
      if(requireCapabilities&&binding?.schema&&["review","blocked"].includes(editor.snapshot().phase)){
        try{form={phase:"ready",binding,...compileQueueForm(binding)};}catch{form={phase:"error"};}
        emit();
      }
    },
    async readConflict(){
      if(state.phase!=="conflict"||state.create)return;
      resetMerge();const pending=editor.readConflict();emit();await pending;
      if(editor.snapshot().comparison)mergeRaw=raw;
      emit();
    },
    editMerge(value){
      if(state.phase!=="conflict"||!state.comparison)return;
      mergeRaw=value;mergeConfirmed=false;mergeValidation=null;modified=true;
      try{if(parseJSON(value)?.metadata?.name!==name)throw new Error("identity");}catch{mergeValidation="invalid-merge";}
      emit();
    },
    confirmMerge(value){
      mergeConfirmed=!!value && state.phase==="conflict" && !!state.comparison && !mergeValidation;
      emit();
    },
    rebase(){
      if(!mergeConfirmed||mergeValidation||state.phase!=="conflict"||!state.comparison||state.create)return;
      editor.rebase(parseJSON(mergeRaw),{confirmed:true});raw=mergeRaw;modified=true;validation=null;resetMerge();emit();
    },
    confirmApply(value){applyConfirmed=!!value&&state.phase==="review"&&!validation&&state.preview?.result.status!=="noop";emit();},
    async apply(){
      if(!applyConfirmed||state.phase!=="review"||validation)return;
      cancelFormLoad();
      applyConfirmed=false;modified=true;const pending=editor.apply();emit();await pending;
      if(editor.snapshot().phase==="accepted")modified=false;
      emit();
    },
    async editNext(){
      if(state.phase!=="accepted"||state.create)return;
      const pending=editor.editNext();emit();await pending;
      if(editor.snapshot().phase==="editing"){
        raw=stringifyJSON(editor.snapshot().draft);modified=false;validation=null;resetMerge();
        if(requireCapabilities)form={phase:"idle"};
      }
      emit();
      if(editor.snapshot().phase==="editing")await loadForm();
    },
    async inspect(){
      if(state.phase!=="uncertain")return;
      const pending=editor.inspectUncertain();emit();await pending;emit();
    },
    async inspectOlderAudit(){
      if(state.phase!=="uncertain")return;
      const pending=editor.inspectOlderAudit();emit();await pending;emit();
    },
    archive(){
      editor.archive();formGeneration++;unsubscribeCapabilities?.();resetMerge();modified=false;emit();
    },
    discard(){
      if(state.phase==="submitting")throw new Error("Cannot discard a pending submission");
      formGeneration++;
      unsubscribeCapabilities?.();
      editor.clear({confirmed:true});raw="";modified=false;validation=null;resetMerge();emit();
    },
  };
}
