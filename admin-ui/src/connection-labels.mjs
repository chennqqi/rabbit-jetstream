const labels=Object.freeze({
  en:Object.freeze({view:"Node connections",rows:"Connection rows",pagination:"Connection pagination",detail:"Connection detail"}),
  zh:Object.freeze({view:"节点连接",rows:"连接数据行",pagination:"连接分页",detail:"连接详情"}),
});

export function connectionLabels(language){
  return language==="zh"?labels.zh:labels.en;
}
