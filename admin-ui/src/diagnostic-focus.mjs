import {queueFields} from "./queue-fields.mjs";

// Explicit mappings only; never interpolate a server pointer into a selector.
export function diagnosticControlID(path){
  const field=queueFields.find(field=>"/"+field.path.join("/")===path);
  if(field)return "queue-field-"+field.key;
  const subject=/^\/spec\/subjects\/(0|[1-9][0-9]{0,8})$/.exec(path);
  if(subject)return "queue-subject-"+subject[1];
  const binding=/^\/spec\/bindings\/(0|[1-9][0-9]{0,8})\/(exchange|type)$/.exec(path);
  if(binding)return "binding-"+binding[2]+"-"+binding[1];
  const key=/^\/spec\/bindings\/(0|[1-9][0-9]{0,8})\/keys\/(0|[1-9][0-9]{0,8})$/.exec(path);
  if(key)return "binding-key-"+key[1]+"-"+key[2];
  return "queue-draft";
}

export function focusDiagnostic(root,path){
  if(!root)return false;
  let target=root.querySelector("#"+diagnosticControlID(path));
  if(!target||target.matches(":disabled"))target=root.querySelector("#queue-draft");
  if(!target||target.matches(":disabled"))return false;
  for(let parent=target.parentElement;parent&&parent!==root;parent=parent.parentElement){
    if(parent.tagName==="DETAILS")parent.open=true;
  }
  target.focus();
  return true;
}
