import React, {useEffect, useState} from "react";
import {queueDetailURL} from "./routes.mjs";
import {evidenceIndicatorLabels} from "./evidence-indicator-labels.mjs";

const severity = phase => ({uncertain: 0, submitting: 1, inspecting: 2}[phase] ?? 9);

export function EvidenceIndicator({drafts, deletions, router, language}) {
  const [, setTick] = useState(0), text = evidenceIndicatorLabels(language);
  useEffect(() => { const timer = setInterval(() => setTick(value => value + 1), 1000); return () => clearInterval(timer); }, []);
  const entries = [];
  for (const [name, model] of drafts) { const phase = model.snapshot().phase; if (phase && phase !== "idle" && phase !== "archived") entries.push({kind: "edit", name, phase}); }
  for (const [name, model] of deletions) { const phase = model.snapshot().phase; if (phase && phase !== "idle") entries.push({kind: "delete", name, phase}); }
  if (!entries.length) return null;
  entries.sort((a, b) => severity(a.phase) - severity(b.phase) || a.name.localeCompare(b.name));
  const items = entries.map(entry => {
    let href;
    try { href = queueDetailURL(entry.name) + (entry.kind === "delete" ? "/delete" : "/edit"); } catch { return null; }
    const visit = event => { if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) { event.preventDefault(); event.currentTarget.closest("details")?.removeAttribute("open"); router.navigate(href); } };
    return <li key={`${entry.kind}:${entry.name}`}><a href={href} onClick={visit}><code>{entry.name}</code>{text.separator}{text.phases[entry.phase] ?? entry.phase}</a></li>;
  }).filter(Boolean);
  const blocked = entries.some(entry => ["submitting", "uncertain", "inspecting"].includes(entry.phase));
  return <details className={`evidence-indicator${blocked ? " evidence-blocked" : ""}`}><summary><span>{text.title}</span><span className="evidence-count" aria-hidden="true">{entries.length}</span><span className="visually-hidden">{text.count(entries.length)}</span></summary><div className="identity-panel" role="region" aria-label={text.region}><p>{text.description}</p><ul>{items}</ul></div></details>;
}
