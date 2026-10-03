const catalogs = Object.freeze({
  en: Object.freeze({
    "receiving_attempt_evidence": "Receiving-attempt evidence",
    "this_describes_one_receiving_attempt_not_every": "This describes one receiving attempt, not every transport replay or the original request's durable outcome. No retry is enabled.",
    "reported_phase": "Reported phase",
    "queue_resource_effects_in_this_attempt": "Queue resource effects in this attempt",
    "none_reported_audit_lock_metadata_excluded": "None reported; audit/lock metadata excluded.",
    "possible_partial_effects_or_completion_are_not": "Possible; partial effects or completion are not resolved.",
    "intent_identifier_persistence_not_guaranteed": "Intent identifier (persistence not guaranteed)",
  }),
  zh: Object.freeze({
    "receiving_attempt_evidence": "接收端单次尝试证据",
    "this_describes_one_receiving_attempt_not_every": "仅描述接收端的一次尝试，不代表所有传输重放或原始请求的持久结果。不会解锁重试。",
    "reported_phase": "报告阶段",
    "queue_resource_effects_in_this_attempt": "本次尝试的 Queue 资源影响",
    "none_reported_audit_lock_metadata_excluded": "报告无资源影响，不包含审计/锁元数据。",
    "possible_partial_effects_or_completion_are_not": "可能有影响，尚不能判定部分执行或已完成。",
    "intent_identifier_persistence_not_guaranteed": "意图标识（不保证已持久化）",
  }),
});

export function mutationEvidenceLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
