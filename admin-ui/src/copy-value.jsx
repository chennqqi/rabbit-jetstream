import React, {useState} from "react";
import {commonDisplayLabels} from "./common-display-labels.mjs";
export function CopyValue({value, language}) {
  const [copied, setCopied] = useState(false), text = commonDisplayLabels(language);
  async function copy() { try { await navigator.clipboard.writeText(value); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch {/* value remains selectable */} }
  return <span className="copy-value"><code>{value}</code><button type="button" onClick={() => void copy()}>{copied ? text.copied : text.copy}</button>{copied && <span role="status" className="visually-hidden">{text.copiedStatus}</span>}</span>;
}
