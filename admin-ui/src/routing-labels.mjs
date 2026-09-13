const labels=Object.freeze({
  en:Object.freeze({exchange:"Exchange",type:"Type",keys:"Keys",subjects:"Subjects",generatedSubjects:"Generated Subjects",matchedSubjects:"Matched Subjects"}),
  zh:Object.freeze({exchange:"交换机",type:"类型",keys:"路由键",subjects:"Subjects",generatedSubjects:"生成的 Subjects",matchedSubjects:"匹配的 Subjects"}),
});

export function routingLabels(language){return labels[language]??labels.en;}
