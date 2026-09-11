const labels=Object.freeze({
  en:Object.freeze({view:"Nodes",collection:"Node collection",metrics:"JetStream metrics"}),
  zh:Object.freeze({view:"节点",collection:"节点集合",metrics:"JetStream 指标"}),
});

export function nodeLabels(language){
  return language==="zh"?labels.zh:labels.en;
}
