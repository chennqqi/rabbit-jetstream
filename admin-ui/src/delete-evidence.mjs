import {stringifyJSON} from "./api.mjs";
import {errorMutationEvidence} from "./mutation-evidence.mjs";

const failure=value=>value?{kind:value.kind,status:value.status,code:value.code,mutation:errorMutationEvidence(value)}:undefined;
const observation=value=>value?{status:value.status,readAt:value.readAt,etag:value.etag,body:value.body,
  error:failure(value.error),olderError:failure(value.olderError),
  windows:value.windows?.map(window=>({body:window.body,readAt:window.readAt}))}:undefined;
const inspection=value=>value?{declaration:observation(value.declaration),stream:observation(value.stream),audit:observation(value.audit)}:undefined;

// Allowlisted local evidence; never serialize transport/session or raw errors.
// Observed resource and audit bodies are intentionally preserved and sensitive.
export function deleteEvidence(state,exportedAt=new Date().toISOString()){
  return stringifyJSON({schema:"rjs.queue-delete-evidence.v1",exportedAt,
    limitations:["Local evidence only, not a server outcome certificate or complete audit history.",
      "Request IDs are correlation identifiers, not idempotency keys. Do not replay unknown deletions.",
      "Mutation evidence describes one receiving attempt only, excludes audit/lock metadata and cannot rule out earlier replays.",
      "Counts and readback are non-atomic observations; missing resources do not establish causality.",
      "Contains configuration and audit data. Session clearing does not delete downloaded files."],
    queue:state.name,phase:state.phase,returnPhase:state.returnPhase,originalETag:state.etag,
    force:state.force,preview:state.preview,capabilities:state.capabilities,requestId:state.requestId,responseRequestId:state.responseRequestId,
    result:state.result,error:failure(state.error),inspection:inspection(state.inspection),
    inspectionHistory:state.inspectionHistory?.map(inspection)});
}
