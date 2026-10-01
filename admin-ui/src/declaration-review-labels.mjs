const catalogs = Object.freeze({
  en: Object.freeze({
    "not_supplied_by_server": "Not supplied by server",
    "declaration_change_review": "Declaration change review",
    "declaration_changes": "Declaration changes",
    "this_compares_the_original_accepted_declaration_with": "This compares the original accepted declaration with the normalized target. It is separate from observed-resource repair below. Impact labels do not authorize changes or establish health. The normalized document never replaces your draft or original ETag.",
    "original_etag_for_this_preview": "Original ETag for this preview",
    "this_server_did_not_supply_declaration_review": "This server did not supply declaration review. No declaration difference can be inferred from its absence; the resource preview remains separate.",
    "declaration_review_is_malformed_or_does_not": "Declaration review is malformed or does not match this draft identity/revision. Confirmation is unavailable; retry preview explicitly.",
    "a_lossless_declaration_comparison_is_unavailable_server": "A lossless declaration comparison is unavailable. Server reason:",
    "create_only_no_previous_declaration_or_old": "Create-only: no previous declaration or old-value diff is fabricated. Review the normalized target and proposed resource operations.",
    "no_normalized_declaration_changes_reported_observed_resources": "No normalized declaration changes reported. Observed resources may still need repair; inspect their operations separately.",
    "declaration_field_differences": "Declaration field differences",
    "field": "Field",
    "original_declaration": "Original declaration",
    "normalized_target": "Normalized target",
    "reported_impact": "Reported impact",
    "normalized_target_queue_document": "Normalized target Queue document",
    "includes_server_normalization_defaults_units_or_formatting": "Includes server normalization/defaults. Units or formatting may differ from the submitted JSON without changing generated configuration.",
  }),
  zh: Object.freeze({
    "not_supplied_by_server": "服务端未提供",
    "declaration_change_review": "声明变更审阅",
    "declaration_changes": "声明变化",
    "this_compares_the_original_accepted_declaration_with": "此处比较原始已接受声明与规范化目标，与下方观测资源修复分开。影响等级不授权变更，也不证明健康。规范化文档不会替换草稿或原始 ETag。",
    "original_etag_for_this_preview": "本次预览的原始 ETag",
    "this_server_did_not_supply_declaration_review": "服务端未提供声明审阅，不能从缺失推断声明无变化；资源预览是独立信息。",
    "declaration_review_is_malformed_or_does_not": "声明审阅格式无效或与草稿身份/版本不匹配。无法确认提交，请手动重新预览。",
    "a_lossless_declaration_comparison_is_unavailable_server": "无法进行无损声明比较。服务端原因：",
    "create_only_no_previous_declaration_or_old": "仅创建：不伪造旧声明或旧值差异。请审阅规范化目标及拟执行的资源操作。",
    "no_normalized_declaration_changes_reported_observed_resources": "未报告规范化声明变化。观测资源仍可能需要修复，请单独检查资源操作。",
    "declaration_field_differences": "声明字段差异",
    "field": "字段",
    "original_declaration": "原始声明",
    "normalized_target": "规范化目标",
    "reported_impact": "报告的影响",
    "normalized_target_queue_document": "规范化目标 Queue 文档",
    "includes_server_normalization_defaults_units_or_formatting": "包含服务端规范化/默认值。单位或格式可能与提交 JSON 不同，但不改变生成配置。",
  }),
});

export function declarationReviewLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
