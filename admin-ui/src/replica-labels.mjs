const catalogs = Object.freeze({
  en: Object.freeze({
    evidence: "Replica evidence", observations: "Replica observations", leader: "Leader", follower: "Follower", unknown: "Unknown", yes: "Yes", no: "No",
    declared: "Declared replicas", configured: "Observed configured replicas",
    configMismatch: "Declared and observed configured replica counts differ; these are separate reads. Refresh to verify.", singleReplica: "A single-replica configuration has no replica fault tolerance.",
    unreported: "Replica topology was not reported; this implies neither health nor absence of replicas.", invalid: "Invalid replica identity or topology response; no partial topology shown.",
    noLeader: "No Leader reported in this observation; the cause is not established.", incomplete: "Reported member rows differ from observed configured replicas; missing members are not fabricated.",
    columns: Object.freeze(["Node name", "Role", "Reported current", "Reported offline", "Lag operations", "Since last activity (ns)"]), guide: "Replica observations are not health — metric guide",
    guideCounts: "Configured counts do not establish online membership, quorum or release qualification. Unreported Leader sync/offline/lag metrics remain unknown; its role does not imply zero lag.", guideNames: "Node names are not stable Node IDs; no potentially incorrect detail links are generated here.",
  }),
  zh: Object.freeze({
    evidence: "副本观测证据", observations: "副本观测", leader: "Leader（主节点）", follower: "Follower（副本节点）", unknown: "未知", yes: "是", no: "否",
    declared: "声明副本数", configured: "观测配置副本数", configMismatch: "声明与观测配置副本数不同；两者独立读取，请刷新核实。", singleReplica: "单副本配置没有副本故障容错。",
    unreported: "此响应未报告副本拓扑，不推断为健康或没有副本。", invalid: "副本身份或拓扑响应无效，不展示部分拓扑。", noLeader: "本次未报告 Leader，不能推断具体原因。", incomplete: "报告的成员行数与观测配置副本数不一致，不补造缺失成员。",
    columns: Object.freeze(["节点名称", "角色", "报告已同步", "报告离线", "落后操作数", "距最近活动（纳秒）"]), guide: "副本观测不是健康结论 — 指标说明",
    guideCounts: "配置数量不代表在线数量、法定人数或发布资格。Leader 未提供的同步、离线及落后指标保持未知；不根据角色补零。", guideNames: "节点名称不是稳定 Node ID；此处不生成可能指错节点的详情链接。",
  }),
});
export function replicaEvidenceLabels(language) { return catalogs[language] ?? catalogs.en; }
const publicCatalogs = Object.freeze({
  en: Object.freeze({evidence: catalogs.en.evidence, observations: catalogs.en.observations, leader: catalogs.en.leader, follower: catalogs.en.follower}),
  zh: Object.freeze({evidence: catalogs.zh.evidence, observations: catalogs.zh.observations, leader: catalogs.zh.leader, follower: catalogs.zh.follower}),
});
export function replicaLabels(language) {
  return publicCatalogs[language] ?? publicCatalogs.en;
}
export function replicaRole(role, language) { const localized = replicaLabels(language); return Object.freeze({leader: localized.leader, follower: localized.follower})[role]; }
