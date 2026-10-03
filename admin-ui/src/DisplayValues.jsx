import React from "react";
import {durationDisplay, readTimeDisplay} from "./display-values.mjs";
import {commonDisplayLabels} from "./common-display-labels.mjs";
export function ExactDuration({value, language}) { const display = durationDisplay(value); return display === null ? <>{commonDisplayLabels(language).unknown}</> : <span title={`${value} ns`}>{display}</span>; }
export function ReadTime({value, language}) { const display = readTimeDisplay(value); return display === null ? <>{commonDisplayLabels(language).unknown}</> : <time dateTime={value} title={value}>{display}</time>; }

// Collapses the per-page honesty notes (observation semantics, freshness
// thresholds) into one folded entry so page bodies stop opening with
// disclaimers while the semantics stay one click away.
export function PageNotes({language, children}) {
  const text = commonDisplayLabels(language);
  return <details className="page-notes"><summary>{text.aboutNumbers}</summary><div className="page-notes-body">{children}</div></details>;
}

// Compact refresh-state indicator replacing the "Automatic refresh: 10
// seconds after each read; failure backoff up to 60 seconds" sentence.
export function RefreshStatus({readAt, paused, seconds, language}) {
  const text = commonDisplayLabels(language);
  if (paused) return <p className="refresh-status" role="status">{text.refreshPaused} · <ReadTime value={readAt} language={language}/></p>;
  if (!seconds) return <p className="refresh-status" role="status">{text.manualOnly} · <ReadTime value={readAt} language={language}/></p>;
  return <p className="refresh-status" role="status">{text.refreshedAt} <ReadTime value={readAt} language={language}/> · {text.autoEvery.replace("{n}", String(seconds))}</p>;
}