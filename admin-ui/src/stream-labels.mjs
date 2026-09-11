const labels=Object.freeze({
  en:Object.freeze({detail:"Stream detail",consumers:"Stream Consumers",pagination:"Stream Consumer pagination",subjects:"Subjects",retention:"Retention",discard:"Discard",bytes:"Bytes",consumerCount:"Consumers",firstSequence:"First sequence",lastSequence:"Last sequence",mode:"Mode",pending:"Pending",ackPending:"Ack pending"}),
  zh:Object.freeze({detail:"Stream 详情",consumers:"Stream Consumer 列表",pagination:"Stream Consumer 分页",subjects:"Subjects",retention:"保留策略",discard:"丢弃策略",bytes:"存储字节",consumerCount:"Consumer 数",firstSequence:"首序列号",lastSequence:"末序列号",mode:"模式",pending:"待投递",ackPending:"待确认"}),
});

export function streamLabels(language){return language==="zh"?labels.zh:labels.en;}
