import {stringifyJSON} from "./api.mjs";
import {errorMutationEvidence} from "./mutation-evidence.mjs";

const failure=value=>value?{status:value.status,code:value.code,requestId:value.requestId,mutation:errorMutationEvidence(value)}:undefined;
const receipt=value=>({requestId:value.requestId,responseRequestId:value.responseRequestId,
  originalETag:value.originalETag,returnedETag:value.returnedETag,document:value.document,
  submittedPlan:value.submittedPlan,result:value.result});
const observation=value=>value?{status:value.status,readAt:value.readAt,etag:value.etag,
  body:value.body,error:failure(value.error),olderError:failure(value.olderError),
  windows:value.windows?.map(window=>({readAt:window.readAt,body:window.body}))}:undefined;

// Explicit evidence fields only: never serialize the model, transport or session.
// Resource documents and audit bodies are intentionally preserved, not redacted.
export function queueEvidence(state,exportedAt=new Date().toISOString()) {
  return stringifyJSON({schema:"rjs.queue-editor-evidence.v1",exportedAt,
    limitations:["Local editor snapshot, not a server outcome certificate or complete history.",
      "Request IDs are correlation identifiers, not idempotency keys. Do not replay unknown writes.",
      "Mutation evidence describes one receiving attempt only, excludes audit/lock metadata and cannot rule out earlier replays.",
      "Observations may be partial and from different times; matching state does not establish causality.",
      "Contains configuration and audit data. Session clearing does not delete downloaded files."],
    queue:state.name,phase:state.phase,createOnly:state.create,
    originalETag:state.etag,base:state.base,raw:state.raw,document:state.draft,
    validation:state.validation,mergeRaw:state.mergeRaw,mergeValidation:state.mergeValidation,
    comparison:state.comparison?{etag:state.comparison.etag,document:state.comparison.document}:undefined,
    preview:state.preview,capabilities:state.capabilities,requestId:state.requestId,responseRequestId:state.responseRequestId,
    returnedETag:state.returnedETag,submittedPlan:state.submittedPlan,result:state.result,
    error:failure(state.error),nextEditError:failure(state.nextEditError),
    inspection:state.inspection?{declaration:observation(state.inspection.declaration),
      consumers:observation(state.inspection.consumers),audit:observation(state.inspection.audit)}:undefined,
    acceptedOperations:state.acceptedOperations?.map(receipt)});
}
