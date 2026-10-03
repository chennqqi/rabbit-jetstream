import {test} from "node:test";
import assert from "node:assert/strict";
import {createMetricHistory,metricHistory} from "../src/metric-history.mjs";

const base={schema:"rjs.metric-history.v1",source:"prometheus",metric:"management-uptime-seconds",window:"1h",start:"2026-09-11T00:00:00Z",end:"2026-09-11T01:00:00Z",stepSeconds:60,series:[{labels:{instance:"management:8223"},samples:[{time:"2026-09-11T00:00:00Z",value:"100.5"},{time:"2026-09-11T00:02:00Z",value:"2.5",gapBefore:true,reset:true}]}]};

test("metric history retains source values, gaps and reset evidence",()=>{
  const value=metricHistory(base,{metric:"management-uptime-seconds",window:"1h"});assert.equal(value.series[0].samples[1].value,"2.5");assert.equal(value.series[0].samples[1].gapBefore,true);assert.equal(value.series[0].samples[1].reset,true);
});

test("metric history rejects mismatches, fabricated resets and duplicate projected series",()=>{
  for(const value of [{...base,source:"browser"},{...base,stepSeconds:15},{...base,series:[{labels:{secret:"x"},samples:[]}]},{...base,metric:"jetstream-storage-bytes"},{...base,series:[{labels:{},samples:[{time:base.start,value:"1",reset:true}]}]},{...base,series:[{labels:{instance:"x"},samples:[]},{labels:{instance:"x"},samples:[]}]}])assert.throws(()=>metricHistory(value,{metric:value.metric,window:"1h"}));
});

test("history model sends only enumerated URL state and clears failed evidence",async()=>{
  const calls=[];let fail=false;const api={request:async path=>{calls.push(path);if(fail)throw{status:503};return{body:{...base,metric:"jetstream-storage-bytes",window:"15m",stepSeconds:15,start:"2026-09-11T00:45:00Z",series:[]}};}};
  const model=createMetricHistory(api);assert.equal(model.snapshot(),model.snapshot(),"external-store snapshot must be referentially stable");await model.load();assert.equal(model.snapshot().phase,"ready");assert.equal(model.snapshot(),model.snapshot());assert.equal(calls[0],"/api/v1/history?metric=jetstream-storage-bytes&window=15m");fail=true;await model.load();assert.equal(model.snapshot().value,null);assert.equal(model.snapshot().error,"unavailable");
  assert.throws(()=>model.select({metric:"up or vector(1)",window:"15m"}),TypeError);model.clear();
});

test("Queue history encodes exact validated identity and fences late reads",async()=>{
  const pending=[];const api={request:path=>new Promise(resolve=>pending.push({path,resolve}))};const model=createMetricHistory(api,{metric:"queue-messages",queue:"orders",window:"15m"});const read=model.load();assert.equal(pending[0].path,"/api/v1/history?metric=queue-messages&window=15m&queue=orders");model.clear();pending[0].resolve({body:{...base,metric:"queue-messages",window:"15m",stepSeconds:15,start:"2026-09-11T00:45:00Z",series:[]}});await read;assert.equal(model.snapshot().phase,"idle");
});
