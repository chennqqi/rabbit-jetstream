import test from "node:test";
import assert from "node:assert/strict";
import {auditLabels} from "../src/audit-labels.mjs";

test("audit accessible names follow the visible language",()=>{
  assert.deepEqual(auditLabels("en"),{events:"Audit events",export:"Audit export",window:"Audit window"});
  assert.deepEqual(auditLabels("zh"),{events:"审计事件",export:"审计导出",window:"审计窗口"});
  assert.equal(auditLabels("unexpected"),auditLabels("en"));
  assert.equal(Object.isFrozen(auditLabels("zh")),true);
});
