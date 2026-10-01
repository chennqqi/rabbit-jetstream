import test from "node:test";
import assert from "node:assert/strict";
import {routingLabels} from "../src/routing-labels.mjs";

test("routing labels use one frozen bilingual vocabulary",()=>{
  assert.deepEqual(routingLabels("en"),{exchange:"Exchange",type:"Type",keys:"Keys",subjects:"Subjects",generatedSubjects:"Generated Subjects",matchedSubjects:"Matched Subjects"});
  assert.deepEqual(routingLabels("zh"),{exchange:"交换机",type:"类型",keys:"路由键",subjects:"Subjects",generatedSubjects:"生成的 Subjects",matchedSubjects:"匹配的 Subjects"});
  assert.equal(routingLabels("other"),routingLabels("en"));
  assert.equal(Object.isFrozen(routingLabels("zh")),true);
});
