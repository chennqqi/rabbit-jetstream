import React from "react";
import {previewOperations} from "./preview-operations.mjs";
import {previewOperationsLabels} from "./preview-operations-labels.mjs";

export function PreviewOperations({preview,language}) {
  const zh=language==="zh",text=previewOperationsLabels(language),operations=previewOperations(preview.result);
  const value=text=>text===undefined?<span>{text.not_supplied_by_server}</span>:<pre className="declaration-json">{text===""?'""':text}</pre>;
  return <section aria-label={text.observed_to_desired_resource_changes}>
    <h4>{text.resource_operations_and_field_changes}</h4>
    <p>{text.these_are_server_comparisons_of_observed_resources}</p>
    {operations===null?<p role="alert">{text.structured_operation_data_is_unavailable_or_invalid}</p>:operations.length===0?<p>{text.the_server_supplied_no_operation_rows_this}</p>:operations.map(operation=><section key={JSON.stringify([operation.resource,operation.name])} className="preview-operation">
      <h5><code>{operation.resource}</code> · <code>{operation.name}</code></h5>
      <p>{text.action}: <code>{operation.action}</code> · {text.reported_impact}: <code>{operation.impact}</code> · {operation.blocked?text.blocked:text.not_blocked_by_this_preview}</p>
      {operation.reason&&<p>{text.server_reason}: {operation.reason}</p>}
      {operation.changes.length===0?<p>{text.no_field_level_rows_supplied_for_this}</p>:<div className="table-scroll" role="region" tabIndex="0" aria-label={`${operation.resource} ${operation.name} ${text.field_changes}`}><table>
        <thead><tr><th>{text.field}</th><th>{text.observed_value}</th><th>{text.proposed_value}</th><th>{text.reported_impact}</th></tr></thead>
        <tbody>{operation.changes.map((change,index)=><tr key={index}><th scope="row"><code>{change.path}</code></th><td>{value(change.from)}</td><td>{value(change.to)}</td><td><code>{change.impact}</code></td></tr>)}</tbody>
      </table></div>}
    </section>)}
  </section>;
}
