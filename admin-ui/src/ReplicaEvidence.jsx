import React from "react";
import {replicaEvidence} from "./replicas.mjs";
import {replicaLabels,replicaRole} from "./replica-labels.mjs";
export function ReplicaEvidence({stream,desired,language}) {
  const zh=language==="zh",unknown=zh?"未知":"Unknown",evidence=replicaEvidence(stream,desired),accessible=replicaLabels(language);
  const boolean=value=>value===null?unknown:value?(zh?"是":"Yes"):(zh?"否":"No");
  return <section aria-label={accessible.evidence}><h3>{accessible.evidence}</h3>
    <dl className="replica-counts">{desired!==undefined&&<><dt>{zh?"声明副本数":"Declared replicas"}</dt><dd>{evidence.declared??unknown}</dd></>}<dt>{zh?"观测配置副本数":"Observed configured replicas"}</dt><dd>{evidence.configured??unknown}</dd></dl>
    {evidence.configMismatch&&<p role="alert">{zh?"声明与观测配置副本数不同；两者独立读取，请刷新核实。":"Declared and observed configured replica counts differ; these are separate reads. Refresh to verify."}</p>}
    {evidence.configured===1&&<p>{zh?"单副本配置没有副本故障容错。":"A single-replica configuration has no replica fault tolerance."}</p>}
    {evidence.phase==="unreported"&&<p>{zh?"此响应未报告副本拓扑，不推断为健康或没有副本。":"Replica topology was not reported; this implies neither health nor absence of replicas."}</p>}
    {evidence.phase==="invalid"&&<p role="alert">{zh?"副本身份或拓扑响应无效，不展示部分拓扑。":"Invalid replica identity or topology response; no partial topology shown."}</p>}
    {evidence.phase==="reported"&&<>
      {!evidence.leaderReported&&<p role="alert">{zh?"本次未报告 Leader，不能推断具体原因。":"No Leader reported in this observation; the cause is not established."}</p>}
      {evidence.coverage===false&&<p role="alert">{zh?"报告的成员行数与观测配置副本数不一致，不补造缺失成员。":"Reported member rows differ from observed configured replicas; missing members are not fabricated."}</p>}
      <div className="table-scroll replica-observations" role="region" tabIndex="0" aria-label={accessible.observations}><table><thead><tr><th>{zh?"节点名称":"Node name"}</th><th>{zh?"角色":"Role"}</th><th>{zh?"报告已同步":"Reported current"}</th><th>{zh?"报告离线":"Reported offline"}</th><th>{zh?"落后操作数":"Lag operations"}</th><th>{zh?"距最近活动（纳秒）":"Since last activity (ns)"}</th></tr></thead><tbody>{evidence.rows.map(row=><tr key={row.name}><th scope="row">{row.name}</th><td>{replicaRole(row.role,language)??unknown}</td><td>{boolean(row.current)}</td><td>{boolean(row.offline)}</td><td>{row.lag??unknown}</td><td>{row.active??unknown}</td></tr>)}</tbody></table></div>
    </>}
    <details className="reading-help"><summary>{zh?"副本观测不是健康结论 · 指标说明":"Replica observations are not health — metric guide"}</summary>
      <p>{zh?"配置数量不代表在线数量、法定人数或发布资格。Leader 未提供的同步、离线及落后指标保持未知；不根据角色补零。":"Configured counts do not establish online membership, quorum or release qualification. Unreported Leader sync/offline/lag metrics remain unknown; its role does not imply zero lag."}</p>
      <p>{zh?"节点名称不是稳定 Node ID，此处不生成可能指错节点的详情链接。":"Node names are not stable Node IDs; no potentially incorrect detail links are generated here."}</p>
    </details>
  </section>;
}
