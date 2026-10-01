import test from "node:test";
import assert from "node:assert/strict";
import {normalizationReview} from "../src/normalization-review.mjs";
test("server-added defaults and omitted fields retain presence distinctions and exact values",()=>{
  const from={spec:{maxPriority:0,limit:9223372036854775807n,empty:"",nil:null}},to={spec:{limit:9223372036854775807n,delivery:{ackWait:"30s",maxDeliver:5},empty:"",nil:null}};
  const copy=structuredClone(from),result=normalizationReview(from,to);
  assert.deepEqual(result.rows,[{path:"/spec/delivery/ackWait",kind:"added",before:undefined,after:'"30s"'},{path:"/spec/delivery/maxDeliver",kind:"added",before:undefined,after:"5"},{path:"/spec/maxPriority",kind:"omitted",before:"0",after:undefined}]);
  assert.deepEqual(from,copy);assert.equal(result.truncated,false);
  assert.equal(normalizationReview({n:9007199254740993n},{n:9007199254740994n}).rows[0].before,"9007199254740993");
});
test("pointer escaping and representation changes do not guess semantic equivalence",()=>{
  const result=normalizationReview({"a/b~":null,duration:"60s",subjects:["b","a"]},{"a/b~":"",duration:"1m0s",subjects:["a","b"]});
  assert.equal(result.rows[0].path,"/a~1b~0");assert.equal(result.rows[0].before,"null");assert.equal(result.rows[0].after,'""');
  assert.equal(result.rows[1].kind,"different");assert.equal(result.rows[2].before,'["b","a"]');
  assert.deepEqual(normalizationReview({b:2,a:1},{a:1,b:2}).rows,[]);
});
test("large field sets report a display limit rather than silently claiming complete comparison",()=>{
  const result=normalizationReview({},Object.fromEntries(Array.from({length:300},(_,i)=>[`k${i}`,i])));
  assert.equal(result.rows.length,256);assert.equal(result.truncated,true);
});
