import React, {useSyncExternalStore} from "react";
import {canArchiveEditor} from "./editor-handoff.mjs";
import {QueueEvidence} from "./QueueEvidence.jsx";
import {stringifyJSON} from "./api.mjs";
import {deletionLabels} from "./deletion-labels.mjs";
import {useDialog, DialogHost} from "./dialog.jsx";
import {editorHandoffLabels} from "./editor-handoff-labels.mjs";

function HandoffEntry({model, language}) {
  const state = useSyncExternalStore(model.subscribe, model.snapshot), text = editorHandoffLabels(language);
  return <div><p>{text.state}: {state.phase}</p>{!canArchiveEditor(state) && <p role="alert">{text.blocked}</p>}<QueueEvidence state={state} language={language}/></div>;
}
export function EditorHandoff({entries, language, onArchive}) {
  const text = editorHandoffLabels(language), accessible = deletionLabels(language), dialog = useDialog();
  useSyncExternalStore(listener => { const stops = entries.map(({model}) => model.subscribe(listener)); return () => stops.forEach(stop => stop()); }, () => entries.map(({model}) => model.snapshot().phase).join(","));
  const blocked = entries.some(({model}) => !canArchiveEditor(model.snapshot()));
  if (!entries.length) return null;
  return <section className="queue-list" aria-label={accessible.handoff}><h3>{text.title}</h3><p>{text.description}</p>
    {entries.map(({key, model}) => <HandoffEntry key={key} model={model} language={language}/>)}
    <button disabled={blocked} onClick={() => { void dialog.confirm(text.confirm).then(ok => { if (ok) onArchive(); }); }}>{text.archive}</button>
    <DialogHost dialog={dialog.dialog} language={language}/>
  </section>;
}
export function ArchivedEditors({records, language}) {
  const text = editorHandoffLabels(language);
  if (!records?.length) return null;
  return <details className="queue-list"><summary>{text.archived}: {records.length}</summary>{records.map((record, index) => <section key={index}><p>{record.key}{text.separator}{record.archivedAt}</p><pre className="declaration-json">{stringifyJSON({phase: record.state.phase, raw: record.state.raw, mergeRaw: record.state.mergeRaw, requestId: record.state.requestId})}</pre><QueueEvidence state={record.state} language={language}/></section>)}</details>;
}
