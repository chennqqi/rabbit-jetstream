import React from "react";
import {declarationReview} from "./declaration-review.mjs";
import {stringifyJSON} from "./api.mjs";
import {NormalizationReview} from "./NormalizationReview.jsx";
import {declarationReviewLabels} from "./declaration-review-labels.mjs";

export function DeclarationReview({state,language}) {
  const zh=language==="zh",text=declarationReviewLabels(language),review=declarationReview(state);
  const value=text=>text===undefined?text.not_supplied_by_server:text===""?'""':text;
  return <section className="declaration-review" aria-label={text.declaration_change_review}>
    <h4>{text.declaration_changes}</h4>
    <p>{text.this_compares_the_original_accepted_declaration_with}</p>
    {!state.create&&<p>{text.original_etag_for_this_preview}: <code>{state.etag}</code></p>}
    {review.status==="missing"&&<p>{text.this_server_did_not_supply_declaration_review}</p>}
    {review.status==="invalid"&&<p role="alert">{text.declaration_review_is_malformed_or_does_not}</p>}
    {review.status==="unavailable"&&<p>{text.a_lossless_declaration_comparison_is_unavailable_server} <code>{review.reason}</code></p>}
    {review.status==="create"&&<p>{text.create_only_no_previous_declaration_or_old}</p>}
    {review.status==="available"&&(review.changes.length===0?<p>{text.no_normalized_declaration_changes_reported_observed_resources}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={text.declaration_field_differences}><table>
      <thead><tr><th>{text.field}</th><th>{text.original_declaration}</th><th>{text.normalized_target}</th><th>{text.reported_impact}</th></tr></thead>
      <tbody>{review.changes.map(change=><tr key={change.path}><th scope="row"><code>{change.path}</code></th><td><pre className="declaration-json">{value(change.from)}</pre></td><td><pre className="declaration-json">{value(change.to)}</pre></td><td><code>{change.impact}</code></td></tr>)}</tbody>
    </table></div>)}
    {review.document&&<>
      <NormalizationReview submitted={state.draft} normalized={review.document} language={language}/>
      <details><summary>{text.normalized_target_queue_document}</summary><p>{text.includes_server_normalization_defaults_units_or_formatting}</p><pre className="declaration-json">{stringifyJSON(review.document)}</pre></details>
    </>}
  </section>;
}
