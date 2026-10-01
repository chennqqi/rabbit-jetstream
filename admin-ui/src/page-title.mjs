const names=Object.freeze({
  en:Object.freeze({login:"Management console",expired:"Session expired",overview:"Overview",queues:"Queue list","create-queue":"Create Queue","bulk-change":"Bulk Queue changes",queue:"Queue","edit-queue":"Edit Queue","delete-queue":"Delete Queue",streams:"Stream list",stream:"Stream",consumers:"Consumer list",consumer:"Consumer",nodes:"Nodes",node:"Node","node-connections":"Connections","node-connection":"Connection",audit:"Audit",diagnostics:"Diagnostics",settings:"Access and settings",compatibility:"Compatibility",unavailable:"Page unavailable"}),
  zh:Object.freeze({login:"管理控制台",expired:"会话已过期",overview:"总览",queues:"Queue 列表","create-queue":"创建 Queue","bulk-change":"批量 Queue 变更",queue:"Queue","edit-queue":"编辑 Queue","delete-queue":"删除 Queue",streams:"Stream 列表",stream:"Stream",consumers:"Consumer 列表",consumer:"Consumer",nodes:"节点",node:"节点","node-connections":"连接","node-connection":"连接",audit:"审计",diagnostics:"诊断包",settings:"访问与设置",compatibility:"兼容性",unavailable:"页面不可用"}),
});

export function consolePageTitle({phase,authenticated=false,route={},language="en"}={}){
  const text=names[language]??names.en;
  if(!authenticated)return `${phase==="expired"?text.expired:text.login} — Rabbit JetStream`;
  const label=text[route.kind]??text.unavailable;
  const identity=route.kind==="consumer"?[route.stream,route.name]:["node","node-connections"].includes(route.kind)?[route.id]:route.kind==="node-connection"?[route.id,route.cid]:["queue","edit-queue","delete-queue","stream"].includes(route.kind)?[route.name]:[];
  const suffix=identity.filter(value=>typeof value==="string"&&value).join(" / ");
  return `${label}${suffix?`: ${suffix}`:""} — Rabbit JetStream`;
}
