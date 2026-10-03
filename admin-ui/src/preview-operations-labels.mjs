const catalogs = Object.freeze({
  en: Object.freeze({
    "not_supplied_by_server": "Not supplied by server",
    "observed_to_desired_resource_changes": "Observed-to-desired resource changes",
    "resource_operations_and_field_changes": "Resource operations and field changes",
    "these_are_server_comparisons_of_observed_resources": "These are server comparisons of observed resources with the proposed generated configuration, not a declaration-to-declaration diff. Impact labels are advisory, not approval, health or qualification. Missing old/new values are not interpreted as zero or absence. Preview may become stale; apply rechecks safety.",
    "structured_operation_data_is_unavailable_or_invalid": "Structured operation data is unavailable or invalid. Inspect the raw preview; no change summary can be established.",
    "the_server_supplied_no_operation_rows_this": "The server supplied no operation rows; this alone is not proof that nothing changes.",
    "action": "Action",
    "reported_impact": "Reported impact",
    "blocked": "Blocked",
    "not_blocked_by_this_preview": "Not blocked by this preview",
    "server_reason": "Server reason",
    "no_field_level_rows_supplied_for_this": "No field-level rows supplied for this operation. Create/ensure actions may still propose work; consult the generated plan.",
    "field_changes": "field changes",
    "field": "Field",
    "observed_value": "Observed value",
    "proposed_value": "Proposed value",
  }),
  zh: Object.freeze({
    "not_supplied_by_server": "服务端未提供",
    "observed_to_desired_resource_changes": "观测资源到目标资源的变化",
    "resource_operations_and_field_changes": "资源操作与字段变化",
    "these_are_server_comparisons_of_observed_resources": "以下是服务端对观测资源与拟生成配置的比较，不是两份声明之间的差异。影响等级仅供参考，不代表批准、健康或资格。未提供的旧/新值不解释为零或不存在。预览可能过时，提交时会重新检查安全条件。",
    "structured_operation_data_is_unavailable_or_invalid": "结构化操作数据缺失或无效。请检查原始预览，无法建立变更摘要。",
    "the_server_supplied_no_operation_rows_this": "服务端未提供操作行，仅凭此不能证明没有变化。",
    "action": "操作",
    "reported_impact": "报告的影响",
    "blocked": "已阻止",
    "not_blocked_by_this_preview": "本次预览未阻止",
    "server_reason": "服务端原因",
    "no_field_level_rows_supplied_for_this": "此操作未提供逐字段变化行。Create/ensure 操作仍可能需要执行，请查看生成计划。",
    "field_changes": "字段变化",
    "field": "字段",
    "observed_value": "观测值",
    "proposed_value": "目标值",
  }),
});

export function previewOperationsLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
