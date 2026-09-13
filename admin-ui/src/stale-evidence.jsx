import React from "react";

// Shared console freshness rule: a read that is paused, failed or older than
// 30 seconds is labeled historical, never presented as current health
// evidence. The note text stays page-specific on purpose.
export function StaleEvidence({readAt, paused, failure, clock, note}) {
  if (!readAt) return null;
  const parsed = Date.parse(readAt);
  if (!Number.isFinite(parsed)) return null;
  if (!(paused || failure || clock - parsed > 30000 || clock < parsed)) return null;
  return <p role="status">{note}</p>;
}
