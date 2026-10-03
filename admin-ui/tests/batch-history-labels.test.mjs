import test from "node:test";
import assert from "node:assert/strict";
import {batchHistoryLabels} from "../src/batch-history-labels.mjs";

test("batch history labels preserve evidence safety wording",()=>{
  assert.match(batchHistoryLabels("en").evidenceNote,/not a server outcome certificate/);
  assert.match(batchHistoryLabels("zh").evidenceNote,/不是服务端结果证书/);
  assert.equal(batchHistoryLabels("other"),batchHistoryLabels("en"));
});
