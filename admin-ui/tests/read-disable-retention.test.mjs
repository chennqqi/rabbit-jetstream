import test from "node:test";
import assert from "node:assert/strict";
import {createOverview} from "../src/overview.mjs";
import {createNodes} from "../src/nodes.mjs";

test("explicit read API disablement clears Overview evidence; unrelated failures retain it",async()=>{
  for(const [error,cleared] of [[{status:404,code:"read_api_disabled"},true],[{status:404,code:"not_found"},false],[{status:503,code:"read_api_disabled"},false]]){
    let failed=false;
    const model=createOverview({request:async path=>{
      if(failed&&path==="/api/v1/info")throw error;
      return {body:path==="/api/v1/info"?{name:"private",version:"v",jetstream:{}}:{items:[],total:0,offset:0,limit:1}};
    }});
    await model.load();const before=model.snapshot().info;failed=true;await model.load();
    const state=model.snapshot();assert.equal(state.info.value,cleared?null:before.value);assert.equal(state.info.readAt,cleared?null:before.readAt);
    assert.equal(state.info.failure,cleared?"disabled":"unavailable");assert.equal(state.queues.phase,"ready");assert.notEqual(state.queues.value,null);
  }
});
test("explicit read API disablement clears retained node identity and time, but generic 404 does not",async()=>{
  for(const [error,cleared] of [[{status:404,code:"read_api_disabled"},true],[{status:404,code:"not_found"},false],[{status:503,code:"read_api_disabled"},false]]){
    let failed=false;const model=createNodes({request:async()=>{if(failed)throw error;return {body:{nodes:[{endpoint:"private",id:"N1",status:"available"}],total:1}};}},{retainOnRefresh:true});
    await model.load();const before=model.snapshot();failed=true;await model.load();const state=model.snapshot();
    assert.equal(state.snapshot,cleared?null:before.snapshot);assert.equal(state.readAt,cleared?null:before.readAt);assert.equal(state.failure,cleared?"disabled":"unavailable");
  }
});
