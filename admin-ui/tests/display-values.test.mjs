import {test} from "node:test";
import assert from "node:assert/strict";
import {durationDisplay,readTimeDisplay} from "../src/display-values.mjs";

test("Durations are compact but exact from nanoseconds through signed int64",()=>{
  for(const [value,text] of [[0,"0s"],[1,"1ns"],[1500,"1.5µs"],[1500000,"1.5ms"],[30000000000,"30s"],[60000000000,"1m"],[86400000000000,"24h"],[30000000001,"30.000000001s"],[9223372036854775807n,"9223372036.854775807s"]])assert.equal(durationDisplay(value),text);
  for(const value of [undefined,null,"30s",-1,1.1,NaN,Infinity,Number.MAX_SAFE_INTEGER+1,9223372036854775808n])assert.equal(durationDisplay(value),null);
});
test("Reader timestamps expose timezone, milliseconds and day boundaries",()=>{
  assert.equal(readTimeDisplay("2026-09-09T08:20:00.123Z","Asia/Shanghai"),"2026-09-09 16:20:00.123 (Asia/Shanghai, GMT+08:00)");
  assert.equal(readTimeDisplay("2026-09-09T20:00:00.000Z","Asia/Shanghai"),"2026-09-10 04:00:00.000 (Asia/Shanghai, GMT+08:00)");
  assert.equal(readTimeDisplay("2026-01-01T00:00:00.000Z","UTC"),"2026-01-01 00:00:00.000 (UTC, GMT+00:00)");
  assert.equal(readTimeDisplay("2026-11-01T05:30:00.000Z","America/New_York"),"2026-11-01 01:30:00.000 (America/New_York, GMT-04:00)");
  assert.equal(readTimeDisplay("2026-11-01T06:30:00.000Z","America/New_York"),"2026-11-01 01:30:00.000 (America/New_York, GMT-05:00)");
});
test("Malformed, impossible and extra-precision times are not silently normalized",()=>{
  for(const value of [null,"","2026-02-30T00:00:00.000Z","2026-09-09","2026-09-09T08:20:00.123456789Z"])assert.equal(readTimeDisplay(value,"UTC"),null);
  assert.equal(readTimeDisplay("2026-09-09T08:20:00.000Z","not/a-zone"),null);
});
