import React,{useEffect,useMemo,useSyncExternalStore} from "react";
import {createCompatibility} from "./compatibility.mjs";
import {ConsoleCapabilities} from "./ConsoleCapabilities.jsx";
import {systemLabels} from "./system-labels.mjs";
import {compatibilityLabels} from "./compatibility-labels.mjs";

export function Compatibility({api,language}){
  const text=compatibilityLabels(language),model=useMemo(()=>createCompatibility(api),[api]),accessible=systemLabels(language);
  const state=useSyncExternalStore(model.subscribe,model.snapshot);
  useEffect(()=>{void model.load();return()=>model.clear();},[model]);
  const evidence=source=><>{source.phase==="loading"&&<p role="status">{text.reading_compatibility_metadata}</p>}{source.phase==="error"&&<p role="alert">{text.source_unavailable_denied_or_incompatible_previous_metadata}</p>}{source.readAt&&<p>{text.read_completed}: <time dateTime={source.readAt}>{source.readAt}</time></p>}</>;
  const build=state.build.value,sdk=state.sdk.value,unknown=text.unreported;
  return <section aria-label={accessible.compatibility}><h2>{text.compatibility}</h2>
    <p>{text.independent_reported_metadata_not_an_atomic_snapshot}</p>
    <button disabled={Object.values(state).some(s=>s.phase==="loading")} onClick={()=>void model.load()}>{text.refresh_compatibility_metadata}</button>
    <section aria-label={accessible.build}><h3>{text.management_process_build}</h3>{evidence(state.build)}{build&&<dl>
      {[[text.reported_version,build.version],[text.go_runtime,build.goVersion],[text.build_target,`${build.os}/${build.arch}`],[text.vcs_revision,build.revision??unknown],[text.revision_source,build.revisionSource??unknown],[text.modified_source_at_build,build.modified===undefined?unknown:String(build.modified)],[text.embedded_ui_identity,build.uiAssets?.digest??unknown],[text.embedded_ui_files,build.uiAssets?.fileCount??unknown]].map(([key,value])=><React.Fragment key={key}><dt>{key}</dt><dd><code>{value}</code></dd></React.Fragment>)}
    </dl>}<p>{text.the_ui_digest_identifies_the_exact_embedded}</p></section>
    <section aria-label={accessible.sdk}><h3>{text.published_native_sdk_contract}</h3>{evidence(state.sdk)}{sdk&&<>
      <dl>{[[text.schema,sdk.schema],[text.declared_availability,sdk.availability],[text.delivery_guarantee,sdk.delivery.delivery_guarantee],[text.publisher_confirmation,sdk.delivery.publisher_confirm],[text.acknowledgment_policy,sdk.delivery.ack_policy]].map(([key,value])=><React.Fragment key={key}><dt>{key}</dt><dd>{value}</dd></React.Fragment>)}</dl>
      <h4>{text.message_headers}</h4><ul>{sdk.headers.map((header,index)=><li key={index}><code>{header.name}</code> — {header.required?text.required:text.optional}</li>)}</ul>
    </>}<p>{text.a_published_contract_does_not_prove_an}</p></section>
    <ConsoleCapabilities api={api} language={language}/>
  </section>;
}
