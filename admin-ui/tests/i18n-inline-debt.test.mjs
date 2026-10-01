import test from "node:test";
import assert from "node:assert/strict";
import {readdir, readFile} from "node:fs/promises";

// Completion gate: user-visible bilingual selection belongs in feature
// catalogs, never inline in components or helpers.
const baseline = Object.freeze({
  "Audit.jsx": 0, "batch-phase.mjs": 0, "BatchHistory.jsx": 0,
  "ConsoleNavigation.jsx": 0, "copy-value.jsx": 0,
  "DeleteEvidence.jsx": 0, "DisplayValues.jsx": 0, "DLQDiagnostics.jsx": 0,
  "DLQEvidence.jsx": 0, "EditorHandoff.jsx": 0, "evidence-indicator.jsx": 0,
  "GlobalConsumers.jsx": 0, "MetricHistory.jsx": 0, "Nodes.jsx": 0,
  "NormalizationReview.jsx": 0, "Overview.jsx": 0, "QueueCreate.jsx": 0,
  "QueueDeclaration.jsx": 0, "QueueEvidence.jsx": 0, "QueueFields.jsx": 0,
  "QueueList.jsx": 0, "QueuePanels.jsx": 0, "QueueTemplates.jsx": 0,
  "ReplicaEvidence.jsx": 0, "StreamDetail.jsx": 0, "SummaryMetrics.jsx": 0,
});

test("inline bilingual selection debt cannot increase", async () => {
  const sourceDirectory = new URL("../src/", import.meta.url);
  const names = (await readdir(sourceDirectory)).filter(name => /\.(jsx|mjs)$/.test(name));
  for (const name of names) {
    const source = await readFile(new URL(name, sourceDirectory), "utf8");
    const count = (source.match(/language\s*===\s*["']zh["']\s*\?|\bzh\s*\?/g) ?? []).length;
    const maximum = 0;
    assert.ok(count <= maximum, `${name} has ${count} inline language selections; maximum is ${maximum}`);
  }
});
