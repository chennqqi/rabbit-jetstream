import React, {useEffect, useState} from "react";
import {deleteEvidence} from "./delete-evidence.mjs";
import {evidenceDownloadLabels} from "./evidence-download-labels.mjs";
export function DeleteEvidence({state, language}) {
  const [download, setDownload] = useState(null), text = evidenceDownloadLabels("deletion", language);
  useEffect(() => { try { const url = URL.createObjectURL(new Blob([deleteEvidence(state)], {type: "application/json"})); setDownload({state, url}); return () => URL.revokeObjectURL(url); } catch { setDownload({state, error: true}); } }, [state]);
  return <section aria-label={text.label}><p>{text.description}</p>{download?.state === state && (download.error ? <p role="alert">{text.error}</p> : <a href={download.url} download="rjs-queue-delete-evidence.json">{text.download}</a>)}</section>;
}
