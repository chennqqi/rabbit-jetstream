import test from "node:test";
import assert from "node:assert/strict";
import {validObservationTime} from "../src/observation-time.mjs";
import {createConnections} from "../src/connections.mjs";

const invalid=["2026-02-30T00:00:00Z","2025-02-29T00:00:00Z","2026-04-31T00:00:00Z","2026-09-10T24:00:00Z","2026-09-10T00:60:00Z","2026-09-10T00:00:60Z","2026-09-10T00:00:00","2026-09-10T00:00:00.1234567890Z","2026-09-10T00:00:00+24:00","2026-09-10T00:00:00+00:60","2026-09-10T00:00:00z","0001-01-01T00:00:00Z","",null,123,{}];
test("observation timestamps require real calendar dates and RFC3339 nanosecond precision",()=>{
  for(const value of invalid)assert.equal(validObservationTime(value),false,String(value));
  for(const value of ["2024-02-29T23:59:59Z","2026-09-10T00:00:00.1Z","2026-09-10T00:00:00.123456789Z","2026-09-10T08:00:00.123456789+08:00","2026-09-09T23:30:00-00:30"])assert.equal(validObservationTime(value),true,value);
});
test("invalid source or management timestamps clear connection evidence and recover without rounding",async()=>{
  const time="2026-09-10T08:00:00.123456789+08:00";
  for(const key of ["observed_at","read_at"]){
    for(const value of invalid){
      const body={node_id:"N",offset:0,limit:50,total:1,items:[{cid:1}],observed_at:time,read_at:time};let reply=body;
      const model=createConnections({request:async()=>({body:reply})},"N",{offset:0,limit:50});model.refresh.setInterval(0);model.refresh.start();await new Promise(resolve=>setImmediate(resolve));assert.equal(model.snapshot().phase,"ready");
      reply={...body,[key]:value};await model.refresh.refresh();assert.equal(model.snapshot().failure,"invalid");assert.equal(model.snapshot().page,null);assert.equal(model.snapshot().readAt,null);
      reply=body;await model.refresh.refresh();assert.equal(model.snapshot().page[key],time);model.clear();
    }
  }
});
