import test from "node:test";
import assert from "node:assert/strict";
import {globalConsumerLabels} from "../src/global-consumer-labels.mjs";

test("global Consumer labels cover collection and query failures in both languages", () => {
  for (const language of ["en", "zh"]) {
    const labels = globalConsumerLabels(language);
    for (const kind of ["unavailable", "collecting", "generation-changed", "invalid-response", "invalid-query", "credentials-rejected", "role-denied", "collection-failed", "fallback"]) {
      assert.equal(typeof labels.failures[kind], "string", `${language} is missing ${kind}`);
      assert.ok(labels.failures[kind].length > 0);
    }
  }
});

test("global Consumer labels fall back to English for an unsupported locale", () => {
  assert.equal(globalConsumerLabels("fr"), globalConsumerLabels("en"));
});
