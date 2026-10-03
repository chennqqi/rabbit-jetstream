import test from "node:test";
import assert from "node:assert/strict";
import {humanBytes, shortRevision} from "../src/format.mjs";

test("humanBytes keeps byte values exact and scales larger magnitudes", () => {
  assert.equal(humanBytes(0), "0 B");
  assert.equal(humanBytes(64713), "63.2 KB");
  assert.equal(humanBytes(26950), "26.3 KB");
  assert.equal(humanBytes(500 * 1024), "500 KB");
  assert.equal(humanBytes(3 * 1024 * 1024 * 1024), "3.0 GB");
});

test("humanBytes rejects non-numeric or negative input", () => {
  assert.equal(humanBytes(undefined), null);
  assert.equal(humanBytes(-1), null);
  assert.equal(humanBytes(Number.NaN), null);
  assert.equal(humanBytes("500"), null);
});

test("shortRevision truncates long identifiers and keeps short values", () => {
  assert.equal(shortRevision("955f19acc66b4eb04abedffd9e27081c"), "955f19ac…");
  assert.equal(shortRevision("short"), "short");
  assert.equal(shortRevision(undefined), undefined);
});
