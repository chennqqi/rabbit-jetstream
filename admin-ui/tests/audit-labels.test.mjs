import test from "node:test";
import assert from "node:assert/strict";
import {auditLabels} from "../src/audit-labels.mjs";

test("audit labels cover accessible, filter, export and evidence states",()=>{
  assert.deepEqual({events:auditLabels("en").events,export:auditLabels("en").export,window:auditLabels("en").window},{events:"Audit events",export:"Audit export",window:"Audit window"});
  assert.deepEqual({events:auditLabels("zh").events,export:auditLabels("zh").export,window:auditLabels("zh").window},{events:"审计事件",export:"审计导出",window:"审计窗口"});
  for(const language of ["en","zh"]){const labels=auditLabels(language);assert.equal(Object.keys(labels.fields).length,8);assert.match(labels.exportHelp,/256|16 MiB/);assert.ok(labels.lowerBoundary);}
  assert.equal(auditLabels("unexpected"),auditLabels("en"));
  assert.equal(Object.isFrozen(auditLabels("zh")),true);
});
