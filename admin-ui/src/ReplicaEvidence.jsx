import React from "react";
import {replicaEvidence} from "./replicas.mjs";
import {replicaEvidenceLabels, replicaRole} from "./replica-labels.mjs";

export function ReplicaEvidence({stream, desired, language}) {
  const text = replicaEvidenceLabels(language);
  const evidence = replicaEvidence(stream, desired);
  const boolean = value => value === null ? text.unknown : value ? text.yes : text.no;
  return <section aria-label={text.evidence}><h3>{text.evidence}</h3>
    <dl className="replica-counts">{desired !== undefined && <><dt>{text.declared}</dt><dd>{evidence.declared ?? text.unknown}</dd></>}<dt>{text.configured}</dt><dd>{evidence.configured ?? text.unknown}</dd></dl>
    {evidence.configMismatch && <p role="alert">{text.configMismatch}</p>}
    {evidence.configured === 1 && <p>{text.singleReplica}</p>}
    {evidence.phase === "unreported" && <p>{text.unreported}</p>}
    {evidence.phase === "invalid" && <p role="alert">{text.invalid}</p>}
    {evidence.phase === "reported" && <>
      {!evidence.leaderReported && <p role="alert">{text.noLeader}</p>}
      {evidence.coverage === false && <p role="alert">{text.incomplete}</p>}
      <div className="table-scroll replica-observations" role="region" tabIndex="0" aria-label={text.observations}><table><thead><tr>{text.columns.map(column => <th key={column}>{column}</th>)}</tr></thead><tbody>{evidence.rows.map(row => <tr key={row.name}><th scope="row">{row.name}</th><td>{replicaRole(row.role, language) ?? text.unknown}</td><td>{boolean(row.current)}</td><td>{boolean(row.offline)}</td><td>{row.lag ?? text.unknown}</td><td>{row.active ?? text.unknown}</td></tr>)}</tbody></table></div>
    </>}
    <details className="reading-help"><summary>{text.guide}</summary><p>{text.guideCounts}</p><p>{text.guideNames}</p></details>
  </section>;
}
