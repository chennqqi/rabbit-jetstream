import React, {useState} from "react";
import {commonDisplayLabels} from "./common-display-labels.mjs";
import {shortRevision} from "./format.mjs";
export function CopyValue({value, language, short}) {
  const [copied, setCopied] = useState(false), text = commonDisplayLabels(language);
  async function copy() { try { await navigator.clipboard.writeText(value); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch {/* value remains selectable */} }
  // short renders the truncated form (full value in the tooltip) while the
  // copy button always places the complete value on the clipboard.
  return <span className="copy-value"><code title={short ? value : undefined}>{short ? shortRevision(value) : value}</code><button type="button" onClick={() => void copy()}>{copied ? text.copied : text.copy}</button>{copied && <span role="status" className="visually-hidden">{text.copiedStatus}</span>}</span>;
}
