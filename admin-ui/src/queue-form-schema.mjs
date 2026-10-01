import {queueFields} from "./queue-fields.mjs";
import {compatibleQueueSchema} from "./queue-schema.mjs";
import {compileQueueCollections} from "./queue-collection-schema.mjs";

// Presentation labels/order are local; value types, choices and annotations
// come from the authenticated schema. Never coerce unsafe numbers or add defaults.
export function compileQueueForm(binding){
  const schema=binding?.schema;
  if(!compatibleQueueSchema(schema,binding?.body))throw new Error("Unsupported form schema");
  const fields=queueFields.map(presentation=>{
    let node=schema,required=true;
    for(const part of presentation.path){
      required=required&&(node.required??[]).includes(part);
      node=node.properties?.[part];
      if(!node)throw new Error("Missing form field");
    }
    if(node.type!==(presentation.type??"string"))throw new Error("Unsupported field type");
    const integer=value=>typeof value==="bigint"||Number.isSafeInteger(value);
    for(const key of ["minimum","maximum"])if(node[key]!==undefined&&!integer(node[key]))throw new Error("Unsafe field bound");
    if(node.minimum!==undefined&&node.maximum!==undefined&&node.minimum>node.maximum)throw new Error("Reversed field bounds");
    if(node.enum!==undefined&&(!Array.isArray(node.enum)||!node.enum.length||
      node.enum.some(value=>node.type==="integer"?!integer(value):typeof value!=="string")||
      new Set(node.enum.map(String)).size!==node.enum.length))throw new Error("Invalid field choices");
    if(node.default!==undefined&&(node.type==="integer"?!integer(node.default):typeof node.default!=="string"))throw new Error("Invalid default annotation");
    return {key:presentation.key,path:[...presentation.path],en:presentation.en,zh:presentation.zh,
      type:node.type,required,options:node.enum?.map(String),minimum:node.minimum,maximum:node.maximum,
      defaultValue:node.default,format:node.format};
  });
  const routingTypes=schema.properties.spec.properties.bindings.items.properties.type.enum;
  if(!Array.isArray(routingTypes)||routingTypes.length!==3||!["direct","topic","fanout"].every(value=>routingTypes.includes(value)))
    throw new Error("Unsupported routing editor");
  return {fields,routingTypes:[...routingTypes],...compileQueueCollections(schema),revision:binding.body.queue.schema.etag};
}
