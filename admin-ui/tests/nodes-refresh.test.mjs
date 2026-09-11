import test from "node:test";
import assert from "node:assert/strict";
import {createNodes} from "../src/nodes.mjs";

test("overview node retention preserves time on failure but clears denied evidence",async()=>{
  let error;const model=createNodes({request:async()=>{if(error)throw error;return {body:{nodes:[],total:0}};}},{retainOnRefresh:true});
  await model.load();const previous=model.snapshot();error={status:503};await model.load();
  assert.equal(model.snapshot().snapshot,previous.snapshot);assert.equal(model.snapshot().readAt,previous.readAt);assert.equal(model.snapshot().failure,"unavailable");
  error={status:401};await model.load();assert.equal(model.snapshot().snapshot,null);assert.equal(model.snapshot().readAt,null);
});
