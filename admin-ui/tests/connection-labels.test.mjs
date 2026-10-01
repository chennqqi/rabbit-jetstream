import test from "node:test";
import assert from "node:assert/strict";
import {connectionLabels} from "../src/connection-labels.mjs";

test("connection accessible names follow the visible language",()=>{
  assert.deepEqual(connectionLabels("en"),{view:"Node connections",rows:"Connection rows",pagination:"Connection pagination",detail:"Connection detail"});
  assert.deepEqual(connectionLabels("zh"),{view:"节点连接",rows:"连接数据行",pagination:"连接分页",detail:"连接详情"});
  assert.equal(connectionLabels("unexpected"),connectionLabels("en"));
  assert.equal(Object.isFrozen(connectionLabels("zh")),true);
});
