import {stringifyJSON} from "./api.mjs";
import {dlqTarget} from "./dlq-diagnostics.mjs";
const fields=(value,keys)=>Object.fromEntries(keys.filter(key=>value?.[key]!==undefined).map(key=>[key,value[key]]));
function observation(value,keys){
  if(!value)return {phase:"unobserved"};
  return {phase:value.phase,readAt:value.readAt??null,...(value.phase==="available"?{etag:value.etag??null,value:fields(value.value,keys)}:{})};
}
export function dlqEvidence({plan,state,declarationETag,declarationReadAt},exportedAt=new Date().toISOString()){
  return stringifyJSON({schema:"rjs.dlq-diagnostic-evidence.v1",exportedAt,
    limitations:["Local read-only snapshot, not a complete history, atomic observation or transfer outcome certificate.",
      "Controller counters cover all Queues in one process, can reset and do not prove this Queue transferred messages.",
      "Ignored counts invalid advisory JSON or no matching declared source, not proof of message loss or completed acknowledgment. Missing counters are unreported, not zero.",
      "Counters describe processing attempts, not unique messages; redelivered advisories can be counted again. Moved can include an already-absent source message. Last successful run does not prove every transfer succeeded.",
      "Existence does not prove ownership or readiness. Missing declaration can leave the Stream unobserved.",
      "Contains operational identifiers and configuration metadata. Session clearing does not remove downloaded files."],
    source:{queue:plan.queue,planRevision:plan.revision,declarationETag:declarationETag??null,readAt:declarationReadAt??null,target:dlqTarget(plan)},
    target:observation(state.target,["queue","revision","stream"]),stream:observation(state.stream,["name"]),
    controller:observation(state.controller,["instanceId","enabled","leader","lastRun","lastSuccess","errorReported","dlqProcessed","dlqMoved","dlqFailed","dlqIgnored"])});
}
