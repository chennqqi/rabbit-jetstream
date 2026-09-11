import React from "react";
import {consumerDiagnosis,consumerNodeEvidence} from "./consumer-diagnosis.mjs";
import {ReplicaEvidence} from "./ReplicaEvidence.jsx";

export function ConsumerDiagnosis({state,nodeState,language,now,paused}) {
  const result=consumerDiagnosis(state,{now,paused}),t=(en,zh)=>language==="zh"?zh:en;
  const nodes=consumerNodeEvidence(result.replicas,nodeState,{now,paused});
  const labels={pending:t("Pending messages","待投递消息数"),ack_pending:t("ACK-pending messages","待确认消息数"),redelivered:t("Reported redelivery count","报告的重投计数"),waiting:t("Waiting pull requests","等待 Pull 请求数"),max_ack_pending:t("Configured ACK-pending limit (only positive values are finite)","配置的待确认上限（仅正数为有限上限）")};
  const replicaMessages={
    "leader-unreported":t("No Consumer Leader was reported. Refresh the exact Consumer and inspect its cluster; this does not identify a cause.","未报告 Consumer Leader。请刷新精确 Consumer 观测并检查所属集群，当前无法确定原因。"),
    "follower-offline":t("Follower reported offline. Inspect that member before attributing backlog to processing capacity.","Follower 报告离线。请先检查该成员，不要直接将积压归因于处理能力。"),
    "follower-not-current":t("Follower reported not current. Compare later observations; one sample does not establish sustained replication failure.","Follower 报告未同步。请对照后续观测，单次样本不证明持续复制故障。"),
    "follower-lag":t("Follower reported lag operations. Lag can be transient; it does not alone explain backlog.","Follower 报告存在落后操作。落后可能是暂态，单凭此值不能解释积压。"),
  };
  const messages={
    "pending-observed":t("Messages are waiting for delivery. Compare processing capacity and later observations; this sample does not establish sustained backlog growth.","观测到待投递消息。请对照处理能力与后续观测；单次样本不能证明积压持续增长。"),
    "ack-limit-reached":t("ACK-pending count meets or exceeds the positive configured limit. Inspect processing and ACK completion before changing limits; do not infer that increasing the limit fixes the cause.","待确认数达到或超过配置的正数上限。请先检查处理与 ACK 完成情况，不要据此认定提高上限能解决原因。"),
    "no-waiting-pull":t("Pending messages coexist with zero waiting pull requests. Check the client's fetch loop and processing phase; zero waiting requests does not prove that clients are disconnected.","待投递消息与零等待 Pull 请求同时出现。请检查客户端拉取循环及处理阶段；零等待请求不证明客户端已断开。"),
    "redelivery-observed":t("The observation includes redelivery. Review ACK timing and application failures; this count is not a rate or proof of a particular failure cause.","观测包含重投。请核对 ACK 时序及应用错误；此计数不是速率，也不能证明具体故障原因。"),
  };
  return <section aria-label={t("Consumer backlog observations","Consumer 积压观测")}>
    <h3>{t("Backlog investigation hints","积压排查提示")}</h3>
    <p>{t("One Consumer observation only: not Queue totals, historical trends, node health or a root-cause verdict. No configuration changes or message operations are performed.","仅基于一个 Consumer 的观测，不是 Queue 汇总、历史趋势、节点健康或根因判定。不修改配置或操作消息。")}</p>
    {result.status!=="current"?<p>{t("Current diagnosis unavailable: refresh this observation. Historical or incomplete reads do not produce current hints.","当前诊断不可用，请刷新观测。历史数据或未完成读取不生成当前提示。")}</p>:<>
      <p>{t("Evidence read completed","证据读取完成")}: {result.readAt}</p>
      <dl>{Object.entries(result.facts).map(([key,value])=><React.Fragment key={key}><dt>{labels[key]}</dt><dd>{value}</dd></React.Fragment>)}</dl>
      {result.missing.length>0&&<p>{t("Unavailable or unsupported evidence fields","不可用或不支持的证据字段")}: {result.missing.join(", ")}</p>}
      {result.findings.length?<ul>{result.findings.map(code=><li key={code}>{messages[code]}</li>)}</ul>:<p>{t("No counter-based hint was triggered by the available fields. This is not a healthy-state verdict.","可用字段未触发计数类提示，不代表健康判定。")}</p>}
      <p>{t("The following replica evidence belongs to this Consumer, not its Stream or the entire node. Configured replica count is not provided by this response; membership coverage and quorum remain unknown.","以下副本证据属于本 Consumer，不是其 Stream 或整个节点。此响应未提供配置副本数，成员覆盖和法定人数保持未知。")}</p>
      <ReplicaEvidence stream={{cluster:state.resource.cluster}} language={language}/>
      {result.replicaFindings.length>0&&<ul aria-label={t("Consumer replica investigation hints","Consumer 副本排查提示")}>{result.replicaFindings.map((finding,index)=><li key={index}>{finding.peer&&<><code>{finding.peer}</code> — </>}{replicaMessages[finding.code]}{finding.lag!==undefined&&<> {t("Reported lag operations","报告的落后操作数")}: {finding.lag}</>}</li>)}</ul>}
      <h4>{t("Configured-node correlation","配置节点关联")}</h4>
      <p>{t("This is an independently timestamped correlation with configured monitoring endpoints by exact server name. It is not complete cluster membership, an atomic snapshot, or node health.","这是按精确服务器名称与已配置监控端点进行的独立时间戳关联，不代表完整集群成员、原子快照或节点健康。")}</p>
      {nodes.status!=="current"?<p>{nodes.status==="stale"?t("Node monitoring evidence is stale or its latest refresh failed; no current correlation is shown.","节点监控证据已过期或最近刷新失败，不展示当前关联。") : t("Node monitoring evidence is unavailable; unresolved peers are not treated as missing nodes.","节点监控证据不可用；未解析的成员不视为节点缺失。")}</p>:<>
        <p>{t("Node evidence read completed","节点证据读取完成")}: <span>{nodes.readAt}</span></p>
        <div className="table-scroll" role="region" tabIndex="0" aria-label={t("Consumer peer node correlation","Consumer 成员节点关联")}><table><thead><tr><th>{t("Consumer peer","Consumer 成员")}</th><th>{t("Association","关联状态")}</th><th>Node ID</th><th>{t("Monitoring status","监控状态")}</th></tr></thead><tbody>{nodes.rows.map(row=><tr key={row.peer}><th scope="row"><code>{row.peer}</code></th><td>{row.association==="matched"?t("Matched configured endpoint","已匹配配置端点"):row.association==="ambiguous"?t("Ambiguous name","名称有歧义"):t("Unresolved; absence not established","未解析；不能确定缺失")}</td><td>{row.nodeID??t("Unknown","未知")}</td><td>{row.monitoringStatus??t("Unknown","未知")}</td></tr>)}</tbody></table></div>
      </>}
    </>}
  </section>;
}
