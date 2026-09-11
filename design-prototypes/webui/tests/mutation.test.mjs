import test from "node:test";
import assert from "node:assert/strict";
import { initialConfig } from "../src/model.js";
import {
  simulateMutation,
  inspectMutation,
  rebaseDraft,
  unresolvedMutation,
} from "../src/mutation.js";

const input = {
  base: initialConfig,
  draft: { ...initialConfig, ackWait: "45s", retention: "48h" },
  revision: 12,
  id: "mock-test",
};
test("accepted fixture preserves immutable request and original revision", () => {
  const result = simulateMutation({ ...input, scenario: "success" });
  assert.equal(result.status, "accepted");
  assert.equal(result.baseRevision, 12);
  assert.equal(initialConfig.ackWait, "30s");
  assert.notEqual(result.request, input.draft);
});
test("rejected permission and expiry never provide committed resources", () => {
  for (const scenario of ["forbidden", "expired"]) {
    const result = simulateMutation({ ...input, scenario });
    assert.equal(result.status, scenario);
    assert.equal(result.current, undefined);
    assert.deepEqual(result.request, input.draft);
  }
});
test("conflict retains base local current and requires explicit rebase", () => {
  const result = simulateMutation({ ...input, scenario: "conflict" });
  assert.equal(result.current.ackWait, "90s");
  assert.equal(result.base.ackWait, "30s");
  assert.equal(result.request.ackWait, "45s");
  const rebased = rebaseDraft(
    input.base,
    { ...input.base, retention: "48h" },
    result.current,
  );
  assert.equal(rebased.retention, "48h");
  assert.equal(rebased.ackWait, "90s");
  assert.equal(
    rebaseDraft(input.base, input.draft, result.current).ackWait,
    "45s",
  );
});
test("unknown result requires inspection, not another write", () => {
  const result = simulateMutation({ ...input, scenario: "unknown" });
  assert.equal(unresolvedMutation(result), true);
  const inspected = inspectMutation(result, initialConfig);
  assert.equal(inspected.status, "verified");
  assert.equal(inspected.currentRevision, 13);
  assert.equal(inspected.id, result.id);
  assert.equal(result.status, "unknown");
});
test("audit missing remains independent of confirmed resource outcome", () => {
  const result = inspectMutation(
    simulateMutation({ ...input, scenario: "audit-failed" }),
    initialConfig,
  );
  assert.equal(result.status, "verified");
  assert.equal(result.auditMissing, true);
});
test("partial outcome does not invent rollback or declaration advancement", () => {
  const result = inspectMutation(
    simulateMutation({ ...input, scenario: "partial" }),
    initialConfig,
  );
  assert.equal(result.observed.retention, "48h");
  assert.equal(result.observed.ackWait, "30s");
  assert.equal(result.currentRevision, 12);
  assert.equal(result.current.retention, "24h");
  assert.equal(unresolvedMutation(result), true);
});
test("invalid and equivalent changes cannot be submitted", () => {
  for (const draft of [
    initialConfig,
    { ...initialConfig, ackWait: "0.5m" },
    { ...initialConfig, ackWait: "bad" },
  ])
    assert.throws(() =>
      simulateMutation({ ...input, draft, scenario: "success" }),
    );
  assert.throws(() => simulateMutation({ ...input, scenario: "unexpected" }));
  assert.throws(() => inspectMutation({ status: "accepted" }, initialConfig));
});
