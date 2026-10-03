export function mutationEvidence(value){
  if(value?.schemaVersion!=="rjs.mutation-evidence.v1"||value.scope!=="receiving-attempt"||
    !["audit_intent","backend","audit_outcome"].includes(value.phase)||
    value.resourceEffects!==(value.phase==="audit_intent"?"none":"possible")||
    (value.intentId!==undefined&&(typeof value.intentId!=="string"||!/^[0-9a-f]{32}$/.test(value.intentId))))return undefined;
  return {schemaVersion:value.schemaVersion,scope:value.scope,phase:value.phase,resourceEffects:value.resourceEffects,
    ...(value.intentId?{intentId:value.intentId}:{})};
}
export const errorMutationEvidence=error=>mutationEvidence(error?.mutation??error?.body?.error?.mutation);
