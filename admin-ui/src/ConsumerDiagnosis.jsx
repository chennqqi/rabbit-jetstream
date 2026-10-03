import React from "react";
import {consumerDiagnosis,consumerNodeEvidence} from "./consumer-diagnosis.mjs";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";
import {consumerDiagnosisLabels} from "./consumer-diagnosis-labels.mjs";

export function ConsumerDiagnosis({state,nodeState,language,now,paused}) {
  const result=consumerDiagnosis(state,{now,paused}),text=consumerDiagnosisLabels(language);
  const nodes=consumerNodeEvidence(result.replicas,nodeState,{now,paused});
  const labels={pending:text.pending_messages,ack_pending:text.ack_pending_messages,redelivered:text.reported_redelivery_count,waiting:text.waiting_pull_requests,max_ack_pending:text.configured_ack_pending_limit_only_positive_values};
  const replicaMessages={
    "leader-unreported":text.no_consumer_leader_was_reported_refresh_the,
    "follower-offline":text.follower_reported_offline_inspect_that_member_before,
    "follower-not-current":text.follower_reported_not_current_compare_later_observations,
    "follower-lag":text.follower_reported_lag_operations_lag_can_be,
  };
  const messages={
    "pending-observed":text.messages_are_waiting_for_delivery_compare_processing,
    "ack-limit-reached":text.ack_pending_count_meets_or_exceeds_the,
    "no-waiting-pull":text.pending_messages_coexist_with_zero_waiting_pull,
    "redelivery-observed":text.the_observation_includes_redelivery_review_ack_timing,
  };
  return <section aria-label={text.consumer_backlog_observations}>
    <h3>{text.backlog_investigation_hints}</h3>
    <p>{text.one_consumer_observation_only_not_queue_totals}</p>
    {result.status!=="current"?<p>{text.current_diagnosis_unavailable_refresh_this_observation_historical}</p>:<>
      <p>{text.evidence_read_completed}: {result.readAt}</p>
      <dl>{Object.entries(result.facts).map(([key,value])=><React.Fragment key={key}><dt>{labels[key]}</dt><dd>{value}</dd></React.Fragment>)}</dl>
      {result.missing.length>0&&<p>{text.unavailable_or_unsupported_evidence_fields}: {result.missing.join(", ")}</p>}
      {result.findings.length?<ul>{result.findings.map(code=><li key={code}>{messages[code]}</li>)}</ul>:<p>{text.no_counter_based_hint_was_triggered_by}</p>}
      <p>{text.the_following_replica_evidence_belongs_to_this}</p>
      <ReplicaEvidence stream={{cluster:state.resource.cluster}} language={language}/>
      {result.replicaFindings.length>0&&<ul aria-label={text.consumer_replica_investigation_hints}>{result.replicaFindings.map((finding,index)=><li key={index}>{finding.peer&&<><code>{finding.peer}</code> — </>}{replicaMessages[finding.code]}{finding.lag!==undefined&&<> {text.reported_lag_operations}: {finding.lag}</>}</li>)}</ul>}
      <h4>{text.configured_node_correlation}</h4>
      <p>{text.this_is_an_independently_timestamped_correlation_with}</p>
      {nodes.status!=="current"?<p>{nodes.status==="stale"?text.node_monitoring_evidence_is_stale_or_its : text.node_monitoring_evidence_is_unavailable_unresolved_peers}</p>:<>
        <p>{text.node_evidence_read_completed}: <span>{nodes.readAt}</span></p>
        <div className="table-scroll" role="region" tabIndex="0" aria-label={text.consumer_peer_node_correlation}><table><thead><tr><th>{text.consumer_peer}</th><th>{text.association}</th><th>Node ID</th><th>{text.monitoring_status}</th></tr></thead><tbody>{nodes.rows.map(row=><tr key={row.peer}><th scope="row"><code>{row.peer}</code></th><td>{row.association==="matched"?text.matched_configured_endpoint:row.association==="ambiguous"?text.ambiguous_name:text.unresolved_absence_not_established}</td><td>{row.nodeID??text.unknown}</td><td>{row.monitoringStatus??text.unknown}</td></tr>)}</tbody></table></div>
      </>}
    </>}
  </section>;
}
