import test from "node:test";
import assert from "node:assert/strict";
import {deletionLabels} from "../src/deletion-labels.mjs";

test("deletion workflow accessible names follow the visible language",()=>{
  assert.deepEqual(deletionLabels("en"),{deletion:"Queue deletion",handoff:"Editor to deletion handoff"});
  assert.deepEqual(deletionLabels("zh"),{deletion:"Queue 删除",handoff:"编辑器转删除交接"});
  assert.equal(deletionLabels("other"),deletionLabels("en"));
  assert.equal(Object.isFrozen(deletionLabels("zh")),true);
});
