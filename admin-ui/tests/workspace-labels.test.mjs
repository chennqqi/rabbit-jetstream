import test from "node:test";
import assert from "node:assert/strict";
import {streamLabels} from "../src/stream-labels.mjs";
import {summaryLabels} from "../src/summary-labels.mjs";

test("Stream and Queue summary accessible names follow the visible language",()=>{
  assert.deepEqual(streamLabels("en"),{detail:"Stream detail",consumers:"Stream Consumers",pagination:"Stream Consumer pagination",subjects:"Subjects",retention:"Retention",discard:"Discard",bytes:"Bytes",consumerCount:"Consumers",firstSequence:"First sequence",lastSequence:"Last sequence",mode:"Mode",pending:"Pending",ackPending:"Ack pending"});
  assert.deepEqual(streamLabels("zh"),{detail:"Stream 详情",consumers:"Stream Consumer 列表",pagination:"Stream Consumer 分页",subjects:"Subjects",retention:"保留策略",discard:"丢弃策略",bytes:"存储字节",consumerCount:"Consumer 数",firstSequence:"首序列号",lastSequence:"末序列号",mode:"模式",pending:"待投递",ackPending:"待确认"});
  assert.deepEqual(summaryLabels("en"),{metrics:"Scoped summary metrics",diagnostics:"Consumer diagnostics"});
  assert.deepEqual(summaryLabels("zh"),{metrics:"范围内摘要指标",diagnostics:"Consumer 排查入口"});
  assert.equal(streamLabels("other"),streamLabels("en"));assert.equal(summaryLabels("other"),summaryLabels("en"));
  assert.equal(Object.isFrozen(streamLabels("zh")),true);assert.equal(Object.isFrozen(summaryLabels("zh")),true);
});
