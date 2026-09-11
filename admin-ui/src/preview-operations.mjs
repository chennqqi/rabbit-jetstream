// Presentation adapter only. Never infer mutation permissions from impact labels.
export function previewOperations(result) {
  if(!Array.isArray(result?.operations))return null;
  const names=new Set();
  for(const operation of result.operations){
    if(!operation||typeof operation.resource!=="string"||!operation.resource||typeof operation.name!=="string"||!operation.name||
      typeof operation.action!=="string"||!operation.action||typeof operation.impact!=="string"||!operation.impact||
      typeof operation.blocked!=="boolean"||!Array.isArray(operation.changes)||
      (operation.reason!==undefined&&typeof operation.reason!=="string"))return null;
    const identity=JSON.stringify([operation.resource,operation.name]);
    if(names.has(identity))return null;names.add(identity);
    for(const change of operation.changes){
      if(!change||typeof change.path!=="string"||!change.path||typeof change.impact!=="string"||!change.impact||
        (change.from!==undefined&&typeof change.from!=="string")||(change.to!==undefined&&typeof change.to!=="string"))return null;
    }
  }
  return result.operations;
}
