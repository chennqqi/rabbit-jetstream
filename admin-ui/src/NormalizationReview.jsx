import React from "react";
import {normalizationReview} from "./normalization-review.mjs";
import {normalizationReviewLabels} from "./normalization-review-labels.mjs";
export function NormalizationReview({submitted, normalized, language}) {
  const text = normalizationReviewLabels(language), {rows, truncated} = normalizationReview(submitted, normalized);
  return <details><summary>{text.title}</summary><p>{text.description}</p>
    {rows.length === 0 ? <p>{text.none}</p> : <div className="table-scroll" role="region" tabIndex="0" aria-label={text.region}><table><thead><tr>{text.columns.map(column => <th key={column}>{column}</th>)}</tr></thead><tbody>{rows.map(row => <tr key={row.path}><th scope="row"><code>{row.path}</code></th><td><pre className="declaration-json">{row.before === undefined ? text.notSupplied : row.before}</pre></td><td><pre className="declaration-json">{row.after === undefined ? text.notSupplied : row.after}</pre></td><td>{text.kinds[row.kind]}</td></tr>)}</tbody></table></div>}
    {truncated && <p role="status">{text.truncated}</p>}
  </details>;
}
