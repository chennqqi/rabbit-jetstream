import test from "node:test";
import assert from "node:assert/strict";
import {nodeLabels} from "../src/node-labels.mjs";

test("node accessible names follow the visible language",()=>{
  assert.deepEqual(nodeLabels("en"),{view:"Nodes",collection:"Node collection",metrics:"JetStream metrics"});
  assert.deepEqual(nodeLabels("zh"),{view:"节点",collection:"节点集合",metrics:"JetStream 指标"});
  assert.equal(nodeLabels("unexpected"),nodeLabels("en"));assert.equal(Object.isFrozen(nodeLabels("zh")),true);
});
