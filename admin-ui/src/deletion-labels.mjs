const labels=Object.freeze({
  en:Object.freeze({deletion:"Queue deletion",handoff:"Editor to deletion handoff"}),
  zh:Object.freeze({deletion:"Queue 删除",handoff:"编辑器转删除交接"}),
});

export function deletionLabels(language){return language==="zh"?labels.zh:labels.en;}
