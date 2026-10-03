const catalogs = Object.freeze({
  en: Object.freeze({
    tabs: Object.freeze(["Summary", "Configuration", "Routing", "Consumers", "Events"]),
    allQueues: "All Queues", refresh: "Refresh Queue page", edit: "Edit draft and preview", delete: "Review deletion impact",
    revision: "Plan revision", etag: "Declaration ETag (original)", unknown: "Unknown", readCompleted: "Declaration read completed",
    guide: "Read-only refresh and navigation guide",
    guideText: "Page refresh rereads the declaration and active panel; it changes neither server state nor editor drafts and is not an atomic snapshot. All Queues opens the unfiltered list; browser Back restores the prior list query.",
    tabLabel: "Queue detail tabs", loading: "Loading…",
    errors: Object.freeze({missing: "Queue declaration not found.", denied: "Read denied. Check credentials and permissions.", unavailable: "Declaration read unavailable; this is not an empty resource."}),
    metrics: Object.freeze({messages: "Stored Queue messages", bytes: "Stored Queue bytes"}),
    inputSubjects: "Declared input Subjects (distinct from priority storage Subjects)", configuration: "Declared configuration (not observed state)",
    notEditable: "This declaration cannot be losslessly converted to an editable document.", rawPlan: "Declared Plan (raw data)",
    routingMap: "Declared routing map", routingNote: "Read-only Plan mapping, not actual delivery records.",
    noBindings: "No declared Exchange bindings; Stream Subjects:", auditDenied: "This identity has no audit read permission.",
  }),
  zh: Object.freeze({
    tabs: Object.freeze(["摘要", "配置", "路由", "消费者", "事件"]),
    allQueues: "所有 Queue", refresh: "刷新 Queue 页面", edit: "编辑草稿与预览", delete: "审阅删除影响",
    revision: "Plan 版本", etag: "声明 ETag（原值）", unknown: "未知", readCompleted: "声明读取完成",
    guide: "只读刷新与返回说明",
    guideText: "整页刷新重新读取声明及当前面板；不修改服务端或编辑草稿，也不是原子快照。返回所有 Queue 会打开未筛选列表；浏览器后退可恢复原列表查询。",
    tabLabel: "Queue 详情标签", loading: "正在读取…",
    errors: Object.freeze({missing: "Queue 声明不存在。", denied: "读取被拒绝，请检查凭据与权限。", unavailable: "声明读取不可用，不代表资源为空。"}),
    metrics: Object.freeze({messages: "Queue 存储消息", bytes: "Queue 存储字节"}),
    inputSubjects: "声明输入 Subjects（与优先级存储 Subjects 区分）", configuration: "声明配置（不是观测状态）",
    notEditable: "此声明无法无损转换为可编辑文档。", rawPlan: "声明 Plan（原始数据）",
    routingMap: "声明路由映射", routingNote: "只读 Plan 映射，不是实际投递记录。",
    noBindings: "无声明的 Exchange 绑定；Stream Subjects 如下：", auditDenied: "当前身份没有审计读取权限。",
  }),
});

export function queueDeclarationLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
