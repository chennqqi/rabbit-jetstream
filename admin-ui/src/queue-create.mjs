export const emptyCreationForm = () => ({name:"",subjects:"",replicas:"",storage:"",maxMessages:""});

export function archiveCreationBatch(setup,{confirmed=false}={}) {
  const record=setup.batch?.archive({confirmed});
  if(!record)return false;
  setup.batchHistory=[...(setup.batchHistory??[]),record];
  setup.batch=null;setup.batchVisible=false;
  return true;
}

export function canParkCreation(state) {
  return !!state?.create&&!state.requestId&&["editing","review","blocked","preview-error","conflict","denied"].includes(state.phase);
}

export function parkCreation(setup) {
  if(!canParkCreation(setup.model?.snapshot()))return false;
  setup.parked=[...(setup.parked??[]),setup.model];
  setup.model=null;setup.form=emptyCreationForm();
  return true;
}

export function startAnotherCreation(setup) {
  const state=setup.model?.snapshot();
  if(state?.phase!=="accepted"||!state.create)return false;
  setup.completed=[...(setup.completed??[]),setup.model];
  setup.model=null;setup.form=emptyCreationForm();
  return true;
}

export function resumeCreation(setup,model,{discardForm=false}={}) {
  if(setup.model||!setup.parked?.includes(model)||!canParkCreation(model.snapshot()))return false;
  if(Object.values(setup.form).some(Boolean)&&!discardForm)return false;
  model.edit(model.snapshot().raw); // Preserve even invalid text; invalidate preview and approval.
  setup.parked=setup.parked.filter(value=>value!==model);
  setup.model=model;setup.form=emptyCreationForm();
  return true;
}

export function retainCreationDraft(drafts,document,create) {
  const key=`create:${document.metadata.name}`;
  if(drafts.has(key))throw Object.assign(new Error("Creation name already retained in this session"),{code:"retained-name"});
  const model=create(document);drafts.set(key,model);return model;
}

export function newQueueDocument({name,subjects,replicas,storage,maxMessages},controls) {
  const field=key=>controls?.fields.find(field=>field.key===key);
  const replicaChoices=controls?field("replicas")?.options:["1","3","5"];
  const storageChoices=controls?field("storage")?.options:["file","memory"];
  const limit=controls?field("maxMessages"): {minimum:0,maximum:9223372036854775807n};
  if(!replicaChoices||!storageChoices||!limit||limit.maximum===undefined)throw new Error("Unsupported creation controls");
  if(typeof name!=="string"||!/^[A-Za-z0-9_-]+$/.test(name))throw new Error("name");
  if(!replicaChoices.includes(replicas)||!Number.isSafeInteger(Number(replicas)))throw new Error("replicas");
  if(!storageChoices.includes(storage))throw new Error("storage");
  if(typeof subjects!=="string"||!subjects.trim())throw new Error("subjects");
  const entries=subjects.split(/\r?\n/).map(value=>value.trim()).filter(Boolean);
  if(new Set(entries).size!==entries.length)throw new Error("subjects");
  if(typeof maxMessages!=="string"||!/^\d+$/.test(maxMessages)||BigInt(maxMessages)<1n||
    BigInt(maxMessages)<BigInt(limit.minimum??0)||BigInt(maxMessages)>BigInt(limit.maximum))throw new Error("maxMessages");
  // Validate Subject semantics/defaults using server preview, not a second
  // divergent topology implementation in the browser.
  return {apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:entries,replicas:Number(replicas),storage,retention:{maxMessages:BigInt(maxMessages)}}};
}
