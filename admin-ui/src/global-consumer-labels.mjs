const catalogs = Object.freeze({
  en: Object.freeze({
    unknown: "Unknown", title: "Consumers",
    description: "The page starts from one complete collected generation, then filters and pages on the server. Routine queries never trigger a full broker scan; Stream plus name is the identity.",
    contains: "Queue, Stream, Consumer or durable contains", filter: "Filter", mode: "Mode", all: "All", state: "State",
    identityOrder: "Identity order", ascending: "Ascending", descending: "Descending", pageSize: "Page size",
    collectingAction: "Collecting…", collectNewGeneration: "Collect new generation",
    auditorCollection: "Auditors can query an existing generation; only operators can start collection.", loading: "Loading Consumers…",
    resetFirst: "Reset to first page", filteredTotal: "Filtered total", generation: "Generation",
    stale: "Showing an older complete generation; the latest collection failed and no partial rows were mixed in.",
    list: "Consumer list", caption: "Global Consumer observation list", ownership: "Ownership", pending: "Pending", ackPending: "Ack pending",
    noMatches: "No matching Consumers.", emptyPage: "This page is empty; return to the previous page.", pagination: "Consumer pagination", previous: "Previous", next: "Next",
    failures: Object.freeze({
      unavailable: "Consumer index unavailable; this is not an empty list.", collecting: "The first complete generation is collecting; retry the query shortly.",
      "generation-changed": "Consumer generation changed; pagination must restart from the first page.",
      "invalid-response": "The server returned an invalid Consumer page; partial data is hidden.", "invalid-query": "Invalid Consumer filters.",
      "credentials-rejected": "Credentials rejected.", "role-denied": "Current role cannot perform this operation.",
      "collection-failed": "Collection failed; an older generation is retained as stale when available.", fallback: "Consumer query failed.",
    }),
  }),
  zh: Object.freeze({
    unknown: "未知", title: "Consumer 列表",
    description: "页面先展示一个完整采集代次，再由服务端筛选和分页。日常查询不会触发 broker 全量扫描；同名 Consumer 以 Stream + 名称区分。",
    contains: "Queue、Stream、Consumer 或 durable 包含", filter: "筛选", mode: "模式", all: "全部", state: "状态",
    identityOrder: "身份排序", ascending: "升序", descending: "降序", pageSize: "每页条数",
    collectingAction: "正在采集…", collectNewGeneration: "采集新代次",
    auditorCollection: "当前 auditor 可查询已有代次；只有 operator 可以启动新采集。", loading: "正在读取 Consumer 列表…",
    resetFirst: "重置到第一页", filteredTotal: "筛选后总数", generation: "代次",
    stale: "显示的是旧完整代次；最近一次采集失败，没有混入部分结果。",
    list: "Consumer 列表", caption: "全局 Consumer 观测列表", ownership: "归属", pending: "待投递", ackPending: "待确认",
    noMatches: "没有匹配的 Consumer。", emptyPage: "当前页没有数据，请返回上一页。", pagination: "Consumer 分页", previous: "上一页", next: "下一页",
    failures: Object.freeze({
      unavailable: "Consumer 索引不可用；这不是空列表。", collecting: "正在采集首个完整代次，请稍后重试查询。",
      "generation-changed": "Consumer 代次已更换，分页必须从第一页重新开始。",
      "invalid-response": "服务端返回了无效 Consumer 页面，不显示部分数据。", "invalid-query": "Consumer 筛选条件无效。",
      "credentials-rejected": "凭据被拒绝。", "role-denied": "当前角色无权执行此操作。",
      "collection-failed": "采集失败；若有旧代次会保留为 stale。", fallback: "Consumer 查询失败。",
    }),
  }),
});

export function globalConsumerLabels(language) {
  return catalogs[language] ?? catalogs.en;
}
