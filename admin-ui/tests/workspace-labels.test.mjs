import test from "node:test";
import assert from "node:assert/strict";
import {streamLabels} from "../src/stream-labels.mjs";
import {summaryLabels} from "../src/summary-labels.mjs";

test("Stream and Queue summary accessible names follow the visible language",()=>{
  assert.deepEqual({detail:streamLabels("en").detail,consumers:streamLabels("en").consumers,pagination:streamLabels("en").pagination},{detail:"Stream detail",consumers:"Stream Consumers",pagination:"Stream Consumer pagination"});
  assert.deepEqual({detail:streamLabels("zh").detail,consumers:streamLabels("zh").consumers,pagination:streamLabels("zh").pagination},{detail:"Stream 详情",consumers:"Stream Consumer 列表",pagination:"Stream Consumer 分页"});
  assert.deepEqual(summaryLabels("en"),{metrics:"Scoped summary metrics",diagnostics:"Consumer diagnostics"});
  assert.deepEqual(summaryLabels("zh"),{metrics:"范围内摘要指标",diagnostics:"Consumer 排查入口"});
  for(const language of ["en","zh"]){const labels=streamLabels(language);assert.equal(Object.keys(labels.failures).length,7);assert.ok(labels.unknown);}
  assert.equal(streamLabels("other"),streamLabels("en"));assert.equal(summaryLabels("other"),summaryLabels("en"));
  assert.equal(Object.isFrozen(streamLabels("zh")),true);assert.equal(Object.isFrozen(summaryLabels("zh")),true);
});
