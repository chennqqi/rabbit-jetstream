import {parseJSON,stringifyJSON} from "./api.mjs";
import {validateBatchResult} from "./batch-import.mjs";
import {canArchiveEditor} from "./editor-handoff.mjs";

const editable=new Set(["editing","review","blocked","preview-error","conflict","denied"]);
const pending=new Set(["loading","previewing","reading-conflict","reading-next","submitting","uncertain","inspecting"]);
const copy=value=>parseJSON(stringifyJSON(value));

// Session-owned coordination only. The retained single-Queue controller owns
// all API operations, preconditions and uncertain-write evidence. No batch PUT,
// automatic retry, rollback, or authorization is inferred from a plan.
export function createBatchExecution(planning){
  if(planning?.phase!=="ready"||!planning.result||!planning.binding)throw Error("Planning evidence required");
  const files=copy(planning.files),input=copy(planning.result);
  const plan=validateBatchResult({scope:"declaration-only-not-apply-authorization",plan:input,external_declarations:input.external},files);
  const entries=plan.items.map(item=>({...item,filename:files[item.index].name,document:files[item.index].document,model:null,view:null}));
  const ordered=new Set(plan.review_order??plan.order),listeners=new Set(),subscriptions=[];let selected=null,failure=null,state,archived=false;
  const canArchive=()=>!archived&&entries.every(entry=>!entry.model||entry.model.snapshot().phase==="archived"||canArchiveEditor(entry.model.snapshot()));
  const blockedByRequest=()=>entries.some(entry=>entry.model&&pending.has(entry.model.snapshot().phase));
  function reason(index){
    if(archived)return "archived";
    const entry=entries[index];
    if(!entry||!ordered.has(index))return "planning-blocked";
    if(blockedByRequest())return "request-pending";
    if(entry.dependencies.some(name=>{const prerequisite=entries.find(value=>value.queue===name);return prerequisite&&prerequisite.model?.snapshot().phase!=="accepted";}))return "prerequisite-not-accepted";
    const draft=entry.model?.snapshot().draft;
    if(draft){
      const dependency=draft.spec?.deadLetter?.queue;
      if(draft.metadata?.name!==entry.queue||stringifyJSON(dependency?[dependency]:[])!==stringifyJSON(entry.dependencies))return "dependency-changed";
    }
    return null;
  }
  function emit(){
    state={selected,failure,archived,canArchive:canArchive(),order:[...plan.order],items:entries.map(entry=>({index:entry.index,queue:entry.queue,filename:entry.filename,problems:copy(entry.problems),phase:entry.model?.snapshot().phase??"not-prepared",blocked:reason(entry.index)}))};
    for(const fn of listeners)fn();
  }
  function allowed(index,requireSelected=false){const blocked=requireSelected&&selected!==index?"not-selected":reason(index);if(blocked){failure=blocked;emit();return false;}failure=null;return true;}
  function select(index){
    if(archived)return false;
    if(!entries[index]?.model){failure="not-prepared";emit();return false;}
    if(selected!==index&&blockedByRequest()){failure="request-pending";emit();return false;}
    if(selected!==index){
      // Switching away and back can never reuse an old preview approval.
      for(const pos of [selected,index]){
        const model=entries[pos]?.model,snapshot=model?.snapshot();
        if(snapshot&&editable.has(snapshot.phase))model.edit(snapshot.raw);
      }
    }
    selected=index;failure=null;emit();return true;
  }
  emit();
  return {
    snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    select,
    selectedModel:()=>selected===null?null:entries[selected].view,
    archive({confirmed=false}={}){
      if(!confirmed||!canArchive())return null;
      // Copy before changing approvals; historical approval is evidence only.
      const record=copy({schema:"rjs.batch-execution-evidence.v1",scope:"archived-batch-not-write-authorization",archivedAt:new Date().toISOString(),
        limitations:["Local session snapshot, not a server outcome certificate or complete history.",
          "Each item was handled separately; this batch was never atomic and has no automatic rollback or retry.",
          "Request IDs are correlation identifiers, not idempotency keys. Do not replay unknown writes.",
          "Contains complete imported declarations and outcome evidence, including labels. Inspect and protect downloaded files.",
          "Declaration portability excludes messages, Consumer state and backup recovery."],
        plan,items:entries.map(entry=>({index:entry.index,queue:entry.queue,filename:entry.filename,document:entry.document,outcome:entry.model?.snapshot()??{phase:"not-prepared"}}))});
      archived=true;selected=null;failure=null;
      for(const unsubscribe of subscriptions)unsubscribe();
      for(const entry of entries)entry.model?.confirmApply(false);
      // Keep underlying controllers in the global name registry. Existing
      // deletion handoff/session clearing still owns their eventual release.
      emit();return record;
    },
    prepare(index,factory){
      if(!allowed(index))return false;
      const entry=entries[index];
      if(entry.model)return select(index);
      try{
        // The factory must register this draft in the same session name registry
        // used by ordinary creation and deletion handoff, not a private map.
        const model=factory(copy(entry.document)),snapshot=model.snapshot();
        if(!snapshot.create||snapshot.name!==entry.queue||snapshot.phase!=="editing")throw Error("Invalid creation controller");
        entry.model=model;
        // Revoke every command after archival, including form reads, local
        // edits and discard (which clears the API token). Keep observation
        // methods available; global registry owners still hold the real model.
        const guarded=Object.fromEntries(Object.entries(model).map(([key,value])=>[key,
          typeof value!=="function"||key==="snapshot"||key==="subscribe"?value:
            (...args)=>archived?Promise.resolve():value(...args)
        ]));
        entry.view={...guarded,
          async preview(){if(allowed(index,true))await model.preview();},
          confirmApply(value){if(!archived&&(!value||allowed(index,true)))model.confirmApply(value);},
          async apply(){if(allowed(index,true))await model.apply();},
        };
        subscriptions.push(model.subscribe(emit));
        return select(index);
      }catch(error){failure=error.code==="retained-name"?"retained-name":"preparation-failed";emit();return false;}
    },
  };
}
