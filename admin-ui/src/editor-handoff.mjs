import {parseJSON,stringifyJSON} from "./api.mjs";
import {emptyCreationForm} from "./queue-create.mjs";

const releasable=new Set(["idle","load-error","uneditable","editing","review","blocked","preview-error","conflict","denied","accepted"]);
export const canArchiveEditor=state=>!!state&&releasable.has(state.phase);
export const queueEditors=(drafts,name)=>[name,`create:${name}`].filter(key=>drafts.has(key)).map(key=>({key,model:drafts.get(key)}));

// Synchronous preflight across both edit/create entries: no partial handoff
// when either is pending/unknown. Preserve snapshots before disabling models.
export function archiveQueueEditors({drafts,archives,creation,name,confirmed=false}){
  const entries=queueEditors(drafts,name);
  if(!confirmed||!entries.length||entries.some(({model})=>!canArchiveEditor(model.snapshot())))return false;
  const records=entries.map(({key,model})=>({key,archivedAt:new Date().toISOString(),state:parseJSON(stringifyJSON(model.snapshot()))}));
  for(const {model} of entries)model.archive();
  const released=new Set(entries.map(entry=>entry.model));
  for(const {key} of entries)drafts.delete(key);
  archives.set(name,[...(archives.get(name)??[]),...records]);
  if(released.has(creation.model)){creation.model=null;creation.form=emptyCreationForm();}
  for(const field of ["parked","completed"])if(creation[field])creation[field]=creation[field].filter(model=>!released.has(model));
  return true;
}
