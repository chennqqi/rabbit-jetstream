const object=value=>value!==null&&typeof value==="object"&&!Array.isArray(value);
const only=(value,keys)=>object(value)&&Object.keys(value).every(key=>keys.includes(key));
const count=value=>Number.isSafeInteger(value)&&value>=0;
const required=(node,key)=>Array.isArray(node?.required)&&node.required.length===1&&node.required[0]===key;
function textRule(node){
  if(!only(node,["type","minLength","maxLength","pattern","format","description","title","examples","default"])||node.type!=="string")throw new Error("Unsupported collection value type");
  for(const key of ["minLength","maxLength"])if(node[key]!==undefined&&!count(node[key]))throw new Error("Unsafe text length");
  if(node.minLength!==undefined&&node.maxLength!==undefined&&node.minLength>node.maxLength)throw new Error("Reversed text lengths");
  if(node.pattern!==undefined&&(typeof node.pattern!=="string"||node.pattern.length>1024))throw new Error("Unsupported pattern annotation");
  if(node.enum!==undefined||node.oneOf!==undefined||node.anyOf!==undefined||node.$ref!==undefined)throw new Error("Unsupported collection value alternatives");
  return {type:node.type,minLength:node.minLength,maxLength:node.maxLength,pattern:node.pattern,format:node.format};
}

export function compileQueueCollections(schema){
  const metadata=schema.properties.metadata.properties,specNode=schema.properties.spec,spec=specNode.properties;
  const labels=metadata.labels;
  if(!only(labels,["type","additionalProperties","description","minProperties","maxProperties"])||labels.type!=="object")throw new Error("Unsupported label map");
  for(const key of ["minProperties","maxProperties"])if(labels[key]!==undefined&&!count(labels[key]))throw new Error("Unsafe label count");
  if(labels.minProperties!==undefined&&labels.maxProperties!==undefined&&labels.minProperties>labels.maxProperties)throw new Error("Reversed label count");
  const labelValue=textRule(labels.additionalProperties);
  const subjects=spec.subjects,bindings=spec.bindings,binding=bindings.items;
  if(!only(subjects,["type","uniqueItems","items","description"])||!only(bindings,["type","items"])||
    !only(binding.properties.keys,["type","uniqueItems","items"]))throw new Error("Unsupported base collection constraints");
  if(!Array.isArray(subjects.type)||subjects.type.length!==2||!subjects.type.includes("array")||!subjects.type.includes("null")||
    subjects.uniqueItems!==true||bindings.type!=="array"||binding.properties.keys.type!=="array"||binding.properties.keys.uniqueItems!==true)
    throw new Error("Unsupported routing collection");
  const modes=specNode.oneOf;
  if(!Array.isArray(modes)||modes.length!==2)throw new Error("Missing exclusive routing branches");
  const subjectMode=modes.find(branch=>required(branch,"subjects"));
  const bindingMode=modes.find(branch=>required(branch,"bindings"));
  for(const mode of [subjectMode,bindingMode])if(!only(mode,["required","properties"])||!only(mode.properties,["subjects","bindings"]))throw new Error("Unsupported routing branch");
  const activeSubjects=subjectMode.properties.subjects,inactiveBindings=subjectMode.properties.bindings;
  const activeBindings=bindingMode.properties.bindings,inactiveSubjects=bindingMode.properties.subjects;
  if(!only(activeSubjects,["type","minItems"])||activeSubjects.type!=="array"||!count(activeSubjects.minItems)||activeSubjects.minItems<1||
    !only(activeBindings,["minItems"])||!count(activeBindings.minItems)||activeBindings.minItems<1||
    !only(inactiveBindings,["maxItems"])||inactiveBindings.maxItems!==0||
    !only(inactiveSubjects,["maxItems"])||inactiveSubjects.maxItems!==0)throw new Error("Unsupported routing exclusivity");
  const branches=binding.oneOf;
  if(!Array.isArray(branches)||branches.length!==3)throw new Error("Missing binding key rules");
  const keys={};
  for(const type of ["direct","topic","fanout"]){
    const branch=branches.find(branch=>branch.properties?.type?.const===type);
    if(!only(branch,["properties","required"])||!only(branch.properties,["type","keys"])||
      !only(branch.properties.type,["const"]))throw new Error("Unsupported binding rule");
    const rule=branch.properties.keys;
    if(type==="fanout"){
      if(branch.required!==undefined||!only(rule,["maxItems"])||rule.maxItems!==0)throw new Error("Unsupported fanout keys");
      keys[type]={maximum:0};
    }else{
      if(!required(branch,"keys")||!only(rule,["minItems"])||!count(rule.minItems)||rule.minItems<1)throw new Error("Unsupported keyed binding");
      keys[type]={minimum:rule.minItems};
    }
  }
  return {
    labels:{value:labelValue,minimum:labels.minProperties,maximum:labels.maxProperties},
    routing:{subjects:{item:textRule(subjects.items),minimum:activeSubjects.minItems,unique:true},
      bindings:{minimum:activeBindings.minItems,exchange:textRule(binding.properties.exchange),
        key:textRule(binding.properties.keys.items),keys,uniqueKeys:true}},
  };
}
