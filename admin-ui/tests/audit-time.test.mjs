import {test} from "node:test";
import assert from "node:assert/strict";
import {auditTimeNanos} from "../src/audit-time.mjs";
import {readAuditQuery} from "../src/audit-query.mjs";
test("Audit times preserve nanosecond and timezone boundaries",()=>{
  const start=auditTimeNanos("2026-09-10T00:00:00.000000001Z");
  assert.equal(auditTimeNanos("2026-09-10T08:00:00.000000001+08:00"),start);
  assert.equal(auditTimeNanos("2026-09-10T00:00:00.000000002Z")-start,1n);
  assert.doesNotThrow(()=>readAuditQuery("?from=2026-09-10T00:00:00.000000001Z&until=2026-09-10T00:00:00.000000002Z"));
  for(const value of ["2026-02-30T00:00:00Z","2026-09-10T24:00:00Z","2026-09-10T00:00:00","2026-09-10T00:00:00.1234567890Z","2026-09-10T00:00:00+24:00"])assert.throws(()=>auditTimeNanos(value));
  assert.throws(()=>readAuditQuery("?from=2026-09-10T00:00:00Z&until=2026-09-10T00:00:00Z"));
});
