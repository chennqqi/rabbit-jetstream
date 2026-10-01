const catalogs = Object.freeze({
  en: Object.freeze({
    metrics: "Scoped summary metrics", diagnostics: "Consumer diagnostics", invalidScope: "Declaration has no valid primary Consumer identity; its metrics cannot be read.", primary: "Declared primary Consumer", refresh: "Refresh summary Consumer", loading: "Loading primary Consumer…",
    missing: "Declared primary Consumer was not found in this read; this is not zero backlog.", failed: "Primary Consumer read failed, was denied or was invalid. Historical metrics are retained only when explicitly labeled below.", historical: "Primary Consumer metrics are from the last successful read, not current observations.", stale: "Stale primary Consumer observation; the 30-second threshold is not health evidence.",
    values: Object.freeze({stored: "Stored messages (Stream)", pending: "Pending (primary Consumer)", ackPending: "Ack pending (primary Consumer)"}), unknown: "Unknown", readCompleted: "Consumer read completed", guide: "This Consumer only; not additive — metric guide", guideText: "Pending/ack-pending belong only to this Consumer, not the whole Queue or all priorities. Stream and Consumer are read separately; these are not additive message categories or proof of ownership markers/configuration agreement.",
  }),
  zh: Object.freeze({
    metrics: "范围内摘要指标", diagnostics: "Consumer 排查入口", invalidScope: "声明未提供有效的主 Consumer 身份，无法读取其指标。", primary: "声明指定的主 Consumer", refresh: "刷新摘要 Consumer", loading: "正在读取主 Consumer…",
    missing: "本次未找到声明指定的主 Consumer；这不表示积压为零。", failed: "主 Consumer 读取失败、被拒绝或响应无效。仅在下方明确标记时保留历史指标。", historical: "主 Consumer 指标来自上次成功读取，不代表当前观测。", stale: "主 Consumer 为旧观测；30 秒阈值不是健康证明。",
    values: Object.freeze({stored: "存储消息数（Stream）", pending: "待投递（主 Consumer）", ackPending: "待确认（主 Consumer）"}), unknown: "未知", readCompleted: "Consumer 读取完成", guide: "仅此 Consumer；不可汇总 — 指标说明", guideText: "待投递、待确认仅属于此 Consumer，不是整个 Queue 或全部优先级的汇总。Stream 与 Consumer 分别读取，三项数值不构成可相加的消息分类，也不证明归属标记或配置一致。",
  }),
});
export function summaryMetricsLabels(language) { return catalogs[language] ?? catalogs.en; }
const publicCatalogs = Object.freeze({
  en: Object.freeze({metrics: catalogs.en.metrics, diagnostics: catalogs.en.diagnostics}),
  zh: Object.freeze({metrics: catalogs.zh.metrics, diagnostics: catalogs.zh.diagnostics}),
});
export function summaryLabels(language) {
  return publicCatalogs[language] ?? publicCatalogs.en;
}
