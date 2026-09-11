const labels=Object.freeze({
  en:Object.freeze({metrics:"Scoped summary metrics",diagnostics:"Consumer diagnostics"}),
  zh:Object.freeze({metrics:"范围内摘要指标",diagnostics:"Consumer 排查入口"}),
});
export function summaryLabels(language){return language==="zh"?labels.zh:labels.en;}
