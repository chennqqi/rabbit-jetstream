import test from "node:test";
import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";

test("generated OpenAPI types preserve lossless uint64 fields as bigint", async () => {
  const source = await readFile(new URL("../src/generated/openapi.d.ts", import.meta.url), "utf8");
  for (const declaration of ["pending: bigint | null;", "cid: bigint;", "lag: bigint;", "nextBefore: bigint | null;"]) {
    assert.match(source, new RegExp(declaration.replace(/[|]/g, "\\|")), declaration);
  }
  assert.doesNotMatch(source, /\n\s+pending: number \| null;/);
});
