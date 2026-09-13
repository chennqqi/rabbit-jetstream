import test from "node:test";
import assert from "node:assert/strict";
import {nodeLabels} from "../src/node-labels.mjs";

test("node labels cover accessible names, states and metric fields",()=>{
  assert.deepEqual({view:nodeLabels("en").view,collection:nodeLabels("en").collection,metrics:nodeLabels("en").metrics},{view:"Nodes",collection:"Node collection",metrics:"JetStream metrics"});
  assert.deepEqual({view:nodeLabels("zh").view,collection:nodeLabels("zh").collection,metrics:nodeLabels("zh").metrics},{view:"节点",collection:"节点集合",metrics:"JetStream 指标"});
  for(const language of ["en","zh"]){const labels=nodeLabels(language);assert.deepEqual(Object.keys(labels.statuses),["available","degraded","unavailable"]);assert.deepEqual(Object.keys(labels.failures),["denied","disabled","fallback"]);assert.equal(Object.keys(labels.jsFields).length,7);}
  assert.equal(nodeLabels("unexpected"),nodeLabels("en"));
  assert.equal(Object.isFrozen(nodeLabels("zh")),true);
});
