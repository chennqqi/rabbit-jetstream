const labels=Object.freeze({
  en:Object.freeze({events:"Audit events",export:"Audit export",window:"Audit window"}),
  zh:Object.freeze({events:"审计事件",export:"审计导出",window:"审计窗口"}),
});

export function auditLabels(language){
  return language==="zh"?labels.zh:labels.en;
}
