const catalogs = Object.freeze({
  en: Object.freeze({
    title: "Metric history",
    description: "Source: server-configured Prometheus. Gaps are not connected; decreasing uptime marks a process reset. No samples does not mean zero.",
    metric: "Metric", window: "Time window", refresh: "Refresh history", querying: "Querying history…",
    errors: Object.freeze({disabled: "History backend is not configured.", denied: "History read was denied.", unavailable: "History backend is unavailable or incompatible."}),
    noSamples: "No samples in this window; state is unknown.", samples: "samples", defaultSeries: "Default series",
    exactSamples: "View exact samples", time: "Time", value: "Value", evidence: "Evidence", gap: "gap", reset: "reset", none: "—",
    defaults: Object.freeze([
      Object.freeze({id: "jetstream-storage-bytes", label: "JetStream storage bytes"}),
      Object.freeze({id: "jetstream-memory-bytes", label: "JetStream memory bytes"}),
      Object.freeze({id: "management-uptime-seconds", label: "Management uptime seconds"}),
    ]),
  }),
  zh: Object.freeze({
    title: "历史指标",
    description: "来源为服务端配置的 Prometheus。缺口不会用连线填补；运行时间下降表示进程重置。没有样本不表示数值为零。",
    metric: "指标", window: "时间范围", refresh: "刷新历史", querying: "正在查询历史数据…",
    errors: Object.freeze({disabled: "服务端未配置历史后端。", denied: "历史读取被拒绝。", unavailable: "历史后端不可用或响应不兼容。"}),
    noSamples: "该范围没有样本；状态未知。", samples: "个样本", defaultSeries: "默认序列",
    exactSamples: "查看精确样本", time: "时间", value: "值", evidence: "证据", gap: "采样缺口", reset: "进程重置", none: "—",
    defaults: Object.freeze([
      Object.freeze({id: "jetstream-storage-bytes", label: "JetStream 存储字节"}),
      Object.freeze({id: "jetstream-memory-bytes", label: "JetStream 内存字节"}),
      Object.freeze({id: "management-uptime-seconds", label: "管理服务运行秒数"}),
    ]),
  }),
});

export function metricHistoryLabels(language) {
  return catalogs[language] ?? catalogs.en;
}

export function metricOptionLabel(option, language) {
  const key = Object.freeze({en: "en", zh: "zh"})[language] ?? "en";
  return (option[key] ?? option.en ?? option.zh) || option.id;
}
