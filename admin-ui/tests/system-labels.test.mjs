import test from "node:test";
import assert from "node:assert/strict";
import {systemLabels} from "../src/system-labels.mjs";

test("system status accessible names follow the visible language",()=>{
  assert.deepEqual(systemLabels("en"),{overview:"Overview",monitoring:"Monitoring summary",account:"Management and account",queues:"Declared Queues",compatibility:"Compatibility",build:"Management build",sdk:"Native SDK contract",capabilities:"Server capabilities"});
  assert.deepEqual(systemLabels("zh"),{overview:"总览",monitoring:"监控摘要",account:"管理服务与账户",queues:"已声明 Queue",compatibility:"兼容性",build:"管理进程构建",sdk:"原生 SDK 契约",capabilities:"服务端能力"});
  assert.equal(systemLabels("other"),systemLabels("en"));assert.equal(Object.isFrozen(systemLabels("zh")),true);
});
