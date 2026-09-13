import React from "react";
import {errorMutationEvidence} from "./mutation-evidence.mjs";
import {mutationEvidenceLabels} from "./mutation-evidence-labels.mjs";

export function MutationEvidence({state,language}){
  const evidence=errorMutationEvidence(state.error),text=mutationEvidenceLabels(language);
  if(!state.requestId||!["uncertain","inspecting"].includes(state.phase)||!evidence)return null;
  return <section aria-label={text.receiving_attempt_evidence}>
    <h3>{text.receiving_attempt_evidence}</h3>
    <p>{text.this_describes_one_receiving_attempt_not_every}</p>
    <dl><dt>{text.reported_phase}</dt><dd>{evidence.phase}</dd>
      <dt>{text.queue_resource_effects_in_this_attempt}</dt>
      <dd>{evidence.resourceEffects==="none"?text.none_reported_audit_lock_metadata_excluded:text.possible_partial_effects_or_completion_are_not}</dd>
      {evidence.intentId&&<><dt>{text.intent_identifier_persistence_not_guaranteed}</dt><dd><code>{evidence.intentId}</code></dd></>}
    </dl>
  </section>;
}
