import React from "react";
import {ConsoleCapabilities} from "./ConsoleCapabilities.jsx";
import {refreshChoices} from "./refresh-preference.mjs";
import {settingsLabels} from "./settings-labels.mjs";

export function Settings({api,identity,language,onLanguage,onClear,refreshSeconds,onRefresh,refreshSaved}) {
  const text=settingsLabels(language);
  return <section aria-labelledby="settings-heading">
    <h2 id="settings-heading">{text.access_and_settings}</h2>
    <h3>{text.verified_session}</h3>
    <dl>
      <dt>{text.actor}</dt><dd>{identity.actor}</dd>
      <dt>{text.role}</dt><dd>{identity.role}</dd>
      <dt>{text.verified_expiry}</dt><dd>{identity.expires_at??text.unknown_not_proof_of_an_unlimited_credential}</dd>
      <dt>{text.resource_read_policy}</dt><dd>{identity.resource_read_policy}</dd>
    </dl>
    <p>{text.this_is_the_identity_verified_at_sign}</p>
    <h3>{text.reported_permissions}</h3>
    {identity.permissions.length?<ul aria-label={text.reported_permissions}>{identity.permissions.map((permission,index)=><li key={`${permission}-${index}`}><code>{permission}</code></li>)}</ul>:<p>{text.no_permissions_reported}</p>}
    <p>{text.permissions_do_not_prove_feature_availability_resource}</p>
    <h3>{text.local_preferences_and_session}</h3>
    <p>{text.language_english_only_language_and_read_page}</p>
    <button onClick={onLanguage}>{text.switch_interface_language}</button>
    <label htmlFor="overview-refresh-interval">{text.read_page_refresh_interval}</label>
    <select id="overview-refresh-interval" value={refreshSeconds} onChange={event=>onRefresh(Number(event.target.value))}>{refreshChoices.map(seconds=><option key={seconds} value={seconds}>{seconds===0?text.manual_only:`${seconds} ${text.seconds}`}</option>)}</select>
    <p>{text.applies_to_overview_queue_summaries_queue_stream}</p>
    {!refreshSaved&&<p role="status">{text.local_storage_is_unavailable_refresh_preference_applies}</p>}
    <p>{text.credentials_drafts_and_request_evidence_stay_in}</p>
    <button onClick={onClear}>{text.clear_this_session}</button>
    <ConsoleCapabilities api={api} language={language}/>
    <h3>{text.not_yet_provided}</h3>
    <p>{text.production_qualification_evidence_is_not_supplied_here}</p>
  </section>;
}
