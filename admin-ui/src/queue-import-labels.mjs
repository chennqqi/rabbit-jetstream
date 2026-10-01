const catalogs = Object.freeze({
  en: Object.freeze({
    "import_queue_declaration": "Import Queue declaration",
    "import_a_single_queue_json_file": "Import a single Queue JSON file",
    "reads_locally_up_to_1_mib_utf": "Reads locally, up to 1 MiB UTF-8 JSON. No request is sent by file selection. This prepares a create-only draft, not a restore or overwrite. Review labels and deployment settings; DLQ dependencies are not included and must be checked during server preview.",
    "queue_json_file": "Queue JSON file",
    "reading_local_file": "Reading local file…",
    "cancel_file_read": "Cancel file read",
    "choose_a_nonempty_json_file_no_larger": "Choose a nonempty JSON file no larger than 1 MiB.",
    "invalid_utf_8_json_or_unsupported_single": "Invalid UTF-8 JSON or unsupported single-Queue document/version. No draft was changed.",
    "loaded_file": "Loaded file: ",
    "bytes": "bytes",
    "labels": "Labels: ",
    "review_imported_document": "Review imported document",
    "i_reviewed_this_file_and_want_to": "I reviewed this file and want to prepare a new Queue draft; preview and apply are separate actions.",
    "prepare_imported_creation_draft": "Prepare imported creation draft",
    "reload_the_creation_schema_before_preparing_this": "Reload the creation schema before preparing this file.",
    "this_name_has_a_retained_request_in": "This name has a retained request in this session; it will not be overwritten.",
    "cannot_prepare_this_draft_check_current_schema": "Cannot prepare this draft. Check current schema and supported replicas/storage; the file is retained for review.",
  }),
  zh: Object.freeze({
    "import_queue_declaration": "导入 Queue 声明",
    "import_a_single_queue_json_file": "导入单个 Queue JSON 文件",
    "reads_locally_up_to_1_mib_utf": "在本地读取，最多 1 MiB UTF-8 JSON，选择文件不发送请求。仅准备创建草稿，不是恢复或覆盖。请审阅标签和部署设置；文件不包含 DLQ 依赖，须在服务端预览时检查。",
    "queue_json_file": "Queue JSON 文件",
    "reading_local_file": "正在读取本地文件…",
    "cancel_file_read": "取消文件读取",
    "choose_a_nonempty_json_file_no_larger": "请选择非空且不超过 1 MiB 的 JSON 文件。",
    "invalid_utf_8_json_or_unsupported_single": "UTF-8 JSON 无效，或不是受支持的单 Queue 文档／版本。未修改草稿。",
    "loaded_file": "已读取文件：",
    "bytes": "字节",
    "labels": "标签数：",
    "review_imported_document": "审阅导入文档",
    "i_reviewed_this_file_and_want_to": "我已审阅文件，希望准备新 Queue 草稿；预览和提交是独立操作。",
    "prepare_imported_creation_draft": "准备导入创建草稿",
    "reload_the_creation_schema_before_preparing_this": "准备此文件前，请重新读取创建 Schema。",
    "this_name_has_a_retained_request_in": "本会话已保留此名称的请求，不会覆盖。",
    "cannot_prepare_this_draft_check_current_schema": "无法准备草稿。请检查当前 Schema 和受支持的副本／存储设置；文件仍保留供审阅。",
  }),
});

export function queueImportLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
