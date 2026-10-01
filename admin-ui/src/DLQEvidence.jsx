import React, {useEffect, useMemo, useState} from "react";
import {dlqEvidence} from "./dlq-evidence.mjs";
import {evidenceDownloadLabels} from "./evidence-download-labels.mjs";
export function DLQEvidence({plan, state, declarationETag, declarationReadAt, language}) {
  const [download, setDownload] = useState(null), text = evidenceDownloadLabels("dlq", language);
  const content = useMemo(() => { try { return dlqEvidence({plan, state, declarationETag, declarationReadAt}); } catch { return null; } }, [plan, state, declarationETag, declarationReadAt]);
  useEffect(() => { if (content === null) { setDownload(null); return; } try { const url = URL.createObjectURL(new Blob([content], {type: "application/json"})); setDownload({content, url}); return () => URL.revokeObjectURL(url); } catch { setDownload({content, error: true}); } }, [content]);
  return <section aria-label={text.label}><p>{text.description}</p>{content === null || download?.content === content && download.error ? <p role="alert">{text.error}</p> : download?.content === content && <a href={download.url} download="rjs-dlq-diagnostic-evidence.json">{text.download}</a>}</section>;
}
