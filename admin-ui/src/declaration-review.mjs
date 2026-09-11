export function declarationReview(state) {
  const preview=state.preview,review=preview?.declaration_review;
  if(review===undefined)return {status:"missing"};
  const invalid=()=>({status:"invalid"});
  if(!review||typeof review!=="object"||Array.isArray(review)||preview.create_only!==state.create||
    preview.plan?.queue!==state.name||preview.result?.queue!==state.name||
    (state.create?preview.base_revision!=="":preview.base_revision!==state.etag))return invalid();
  const document=review.document;
  if(document!==undefined&&(!document||document.apiVersion!=="rabbit-jetstream.io/v1alpha1"||document.kind!=="Queue"||document.metadata?.name!==state.name||!document.spec||typeof document.spec!=="object"||Array.isArray(document.spec)))return invalid();
  if(review.status==="unavailable"){
    if(!["target_unrepresentable","base_unrepresentable","base_identity_mismatch"].includes(review.reason)||review.diff!==undefined)return invalid();
    if(review.reason==="target_unrepresentable"?document!==undefined:!document||state.create)return invalid();
    return {status:"unavailable",reason:review.reason,document};
  }
  if(review.reason!==undefined||!document)return invalid();
  if(review.status==="create")return state.create&&review.diff===undefined?{status:"create",document}:invalid();
  if(review.status!=="available"||state.create||review.diff?.queue!==state.name||!Array.isArray(review.diff.changes))return invalid();
  const seen=new Set();
  for(const change of review.diff.changes){
    if(!change||typeof change.path!=="string"||!change.path||seen.has(change.path)||typeof change.impact!=="string"||!change.impact||
      (change.from!==undefined&&typeof change.from!=="string")||(change.to!==undefined&&typeof change.to!=="string"))return invalid();
    seen.add(change.path);
  }
  return {status:"available",document,changes:review.diff.changes};
}
