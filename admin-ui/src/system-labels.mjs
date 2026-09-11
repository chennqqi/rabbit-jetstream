const labels=Object.freeze({
  en:Object.freeze({overview:"Overview",monitoring:"Monitoring summary",account:"Management and account",queues:"Declared Queues",compatibility:"Compatibility",build:"Management build",sdk:"Native SDK contract",capabilities:"Server capabilities"}),
  zh:Object.freeze({overview:"总览",monitoring:"监控摘要",account:"管理服务与账户",queues:"已声明 Queue",compatibility:"兼容性",build:"管理进程构建",sdk:"原生 SDK 契约",capabilities:"服务端能力"}),
});
export function systemLabels(language){return language==="zh"?labels.zh:labels.en;}
