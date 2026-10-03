// Presentation formatting for values the API returns as raw numbers or full
// identifiers. Formatting is display-only: never pass formatted strings back
// to the API, and never use them to make ordering decisions.
const byteUnits = ["B", "KB", "MB", "GB", "TB", "PB"];

export function humanBytes(value) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return null;
  let scaled = value, unit = 0;
  while (scaled >= 1024 && unit < byteUnits.length - 1) { scaled /= 1024; unit += 1; }
  const rounded = unit === 0 ? String(scaled) : scaled.toFixed(scaled >= 100 ? 0 : 1);
  return `${rounded} ${byteUnits[unit]}`;
}

export function shortRevision(revision) {
  return typeof revision === "string" && revision.length > 8 ? `${revision.slice(0, 8)}…` : revision;
}
