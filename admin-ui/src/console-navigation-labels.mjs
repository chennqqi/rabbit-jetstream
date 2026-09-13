const catalogs = Object.freeze({
  en: Object.freeze({primary: "Primary navigation", menu: "Navigation menu", navigation: "Navigation", page: "Page", entries: Object.freeze({overview: "Overview", queues: "Queue list", streams: "Stream list", consumers: "Consumer list", nodes: "Node list", audit: "Audit", create: "Create Queue", bulk: "Bulk changes", diagnostics: "Diagnostics", alerts: "Operational alerts", settings: "Access and settings", access: "Tenant access", compatibility: "Compatibility"})}),
  zh: Object.freeze({primary: "主导航", menu: "导航菜单", navigation: "导航", page: "页面", entries: Object.freeze({overview: "总览", queues: "Queue 列表", streams: "Stream 列表", consumers: "Consumer 列表", nodes: "节点列表", audit: "审计", create: "创建 Queue", bulk: "批量变更", diagnostics: "诊断包", alerts: "运维告警", settings: "访问与设置", access: "租户访问管理", compatibility: "兼容性"})}),
});
export function consoleNavigationLabels(language) { return catalogs[language] ?? catalogs.en; }
