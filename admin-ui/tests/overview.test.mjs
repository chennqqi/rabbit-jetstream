import {test} from "node:test";
import assert from "node:assert/strict";
import {createOverview} from "../src/overview.mjs";
import {readRoute} from "../src/routes.mjs";

test("Overview uses full server total and preserves exact account integers",async()=>{
  const model=createOverview({request:async path=>({body:path.includes("/info")?{name:"m",version:"v",jetstream:{memory_used:9007199254740993n}}:{items:[{queue:"first"}],offset:0,limit:1,total:201}})});
  await model.load();assert.equal(model.snapshot().queues.value.total,201);assert.equal(model.snapshot().info.value.jetstream.memory_used,9007199254740993n);
  assert.equal(readRoute(new URL("/admin/overview","http://localhost")).kind,"overview");
});
test("Independent source failure retains last success without advancing its time",async()=>{
  let fail=false;
  const model=createOverview({request:async path=>{
    if(path.includes("/info")){if(fail)throw Object.assign(new Error("private"),{status:503});return {body:{name:"m",version:"v",jetstream:{streams:1}}};}
    return {body:{items:[],offset:0,limit:1,total:0}};
  }});
  await model.load();const previous=model.snapshot().info;fail=true;await model.load();
  assert.equal(model.snapshot().info.failure,"unavailable");assert.equal(model.snapshot().info.readAt,previous.readAt);assert.equal(model.snapshot().info.value,previous.value);
  assert.equal(model.snapshot().queues.phase,"ready");assert.equal(model.snapshot().queues.value.total,0);
});

test("Denied overview refresh clears prior protected values",async()=>{
  let denied=false;
  const model=createOverview({request:async path=>{if(denied)throw {status:403};return {body:path.includes("/info")?{name:"m",version:"v",jetstream:{}}:{items:[],offset:0,limit:1,total:0}};}});
  await model.load();denied=true;await model.load();
  for(const source of Object.values(model.snapshot())){assert.equal(source.failure,"denied");assert.equal(source.readAt,null);assert.equal(source.value,null);}
});
test("Invalid page never masquerades as a complete total; late reads ignored",async()=>{
  const model=createOverview({request:async()=>({body:{items:[],offset:0,limit:1,total:999}})});await model.load();assert.equal(model.snapshot().queues.failure,"invalid");
  const pending=[];const delayed=createOverview({request:()=>new Promise(resolve=>pending.push(resolve))});const done=delayed.load();delayed.clear();
  for(const resolve of pending)resolve({body:{}});await done;assert.equal(delayed.snapshot().info.phase,"idle");assert.equal(delayed.snapshot().queues.phase,"idle");
});
