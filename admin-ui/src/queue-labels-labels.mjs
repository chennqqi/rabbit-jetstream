const catalogs = Object.freeze({
  en: Object.freeze({
    "label_key_exact_text_existing_keys_cannot": "Label key (exact text; existing keys cannot be overwritten)",
    "label_key": "Label key",
    "queue_labels": "Queue labels",
    "schema_label_values_text_length_count_rules": "Schema label values: text. Length/count rules are server-validated; invalid drafts remain editable.",
    "minimum_value_length": "Minimum value length: ",
    "maximum_value_length": "Maximum value length: ",
    "minimum_labels": "Minimum labels: ",
    "maximum_labels": "Maximum labels: ",
    "schema_label_field_rules": "Schema label field rules",
    "keys_are_exact_and_case_sensitive_empty": "Keys are exact and case-sensitive. Empty values and multiline text are preserved. Add/rename refuses duplicate keys; removal requires confirmation. Changes affect only this draft until preview and confirmed apply.",
    "that_label_key_already_exists_no_label": "That label key already exists; no label was overwritten.",
    "label_edit_was_rejected_the_draft_is": "Label edit was rejected; the draft is unchanged.",
    "label_value": "Label value:",
    "empty_key": "(empty key)",
    "rename_label": "Rename label:",
    "remove_label": "Remove label:",
    "add_label": "Add label",
  }),
  zh: Object.freeze({
    "label_key_exact_text_existing_keys_cannot": "标签键（精确文本；不能覆盖已有键）",
    "label_key": "标签键",
    "queue_labels": "Queue 标签",
    "schema_label_values_text_length_count_rules": "Schema 标签值：文本。长度/数量规则由服务端验证；无效草稿仍可编辑。",
    "minimum_value_length": "值最小长度：",
    "maximum_value_length": "值最大长度：",
    "minimum_labels": "最少标签数：",
    "maximum_labels": "最多标签数：",
    "schema_label_field_rules": "Schema 标签字段规则",
    "keys_are_exact_and_case_sensitive_empty": "键精确匹配且区分大小写，保留空值及多行文本。新增/重命名拒绝重复键，删除须确认。重新预览并确认提交前，仅改变此草稿。",
    "that_label_key_already_exists_no_label": "该标签键已存在，未覆盖任何标签。",
    "label_edit_was_rejected_the_draft_is": "标签编辑被拒绝，草稿未改变。",
    "label_value": "标签值：",
    "empty_key": "（空键）",
    "rename_label": "重命名标签：",
    "remove_label": "移除标签：",
    "add_label": "添加标签",
  }),
});

export function queueLabelsLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
