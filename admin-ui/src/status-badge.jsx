import React from "react";

// Translates observation outcomes into visible priority. Tones map to the
// shipped alert severities: ok = healthy/consistent, warn = degraded or
// pending, bad = failing/firing, neutral = unavailable/inactive. The label is
// always the server's own short state word — badges never invent health.
export function StatusBadge({tone = "neutral", children}) {
  return <span className={`status-badge status-${tone}`}>{children}</span>;
}

export function observationTone(state) {
  if (state === "present") return "ok";
  if (state === "degraded") return "warn";
  if (state === "missing") return "bad";
  return "neutral";
}

export function nodeStatusTone(status) {
  if (status === "available") return "ok";
  if (status === "degraded") return "warn";
  if (status === "unavailable") return "bad";
  return "neutral";
}

export function alertStateTone(state) {
  if (state === "firing") return "bad";
  if (state === "pending") return "warn";
  if (state === "recovered") return "ok";
  return "neutral";
}
