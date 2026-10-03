import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createCapabilities} from "./capabilities.mjs";
import {stringifyJSON} from "./api.mjs";
import {systemLabels} from "./system-labels.mjs";
import {consoleCapabilitiesLabels} from "./console-capabilities-labels.mjs";

export function ConsoleCapabilities({api,language}){
  const model=useMemo(()=>createCapabilities(api),[api]),state=useSyncExternalStore(model.subscribe,model.snapshot),zh=language==="zh",accessible=systemLabels(language);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const text=consoleCapabilitiesLabels(language),body=state.body;
  return <section aria-label={accessible.capabilities} className="queue-list console-capabilities">
    <h3>{text.server_declared_capabilities}</h3>
    <button disabled={state.phase==="loading"} onClick={()=>void model.load()}>{text.refresh_server_capabilities}</button>
    {state.phase==="loading"&&<p role="status">{text.reading_contract_metadata}</p>}
    {state.phase==="error"&&<p role="alert">{text.capabilities_unavailable_denied_or_incompatible_deployment_intent}</p>}
    {state.phase==="ready"&&<>
      <dl className="declaration-meta">
        <dt>{text.declared_deployment_profile}</dt><dd>{body.deployment.profile}</dd>
        <dt>{text.profile_source}</dt><dd>{body.deployment.source}</dd>
        <dt>{text.parser_supported_replicas}</dt><dd>{body.queue.supportedReplicas.join(", ")}</dd>
        <dt>{text.parser_supported_storage}</dt><dd>{body.queue.supportedStorage.join(", ")}</dd>
        <dt>{text.parser_supported_priority_range}</dt><dd>{body.queue.minimumPriority}–{body.queue.maximumPriority}</dd>
        <dt>{text.production_qualification}</dt><dd>{body.qualification.status==="reported"?text.manifest_statement_reported:text.unreported_by_this_api}</dd>
        {body.qualification.status==="reported"&&<><dt>{text.manifest_statement}</dt><dd>{body.qualification.statement}</dd><dt>{text.manifest_sha_256}</dt><dd><code>{body.qualification.manifestDigest}</code></dd></>}
        <dt>{text.read_completed}</dt><dd>{state.readAt}</dd>
        <dt>{text.queue_document_version}</dt><dd>{body.queue.apiVersion}</dd>
      </dl>
      <p>{text.deployment_profile_is_configuration_not_observed_topology}</p>
      <details><summary>{text.canonical_omitted_field_defaults}</summary><pre className="declaration-json">{stringifyJSON(body.queue.defaults)}</pre><p>{text.these_partial_defaults_are_not_a_complete}</p></details>
      <details><summary>{text.implemented_api_contract_identifiers}</summary><ul>{body.features.map(feature=><li key={feature}><code>{feature}</code></li>)}</ul><p>{text.contract_implementation_is_not_permission_backend_availability}</p></details>
      {state.schema&&<details><summary>{text.verified_queue_authoring_schema}</summary>
        <p>{text.all_document_fields_are_described_custom_formats}</p>
        <p><code>{body.queue.schema.id}</code></p>
        <pre className="declaration-json">{stringifyJSON(state.schema)}</pre>
      </details>}
    </>}
  </section>;
}
