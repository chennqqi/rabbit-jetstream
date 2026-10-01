import {test} from "node:test";
import assert from "node:assert/strict";
import {summaryConsumerScope,summaryConsumerCounts} from "../src/summary-consumer.mjs";

test("Summary selects declared primary identity, never a priority or first collection member",()=>{
  const scope=summaryConsumerScope({stream:{name:"S"},consumer:{stream:"S",name:"primary"},priorityConsumers:[{stream:"S",name:"first"}]});
  assert.deepEqual(scope,{stream:"S",name:"primary",url:"/admin/streams/S/consumers/primary"});
  for(const plan of [{},{stream:{name:"S"},consumer:{stream:"other",name:"primary"}},{stream:{name:"S"},consumer:{stream:"S",name:"../bad"}},{stream:{name:"S"},priorityConsumers:[{stream:"S",name:"first"}]}])assert.equal(summaryConsumerScope(plan),null);
});
test("Summary keeps exact scoped counters and never converts stale/missing evidence to zero",()=>{
  const scope={stream:"S",name:"primary"},resource={...scope,pending:9007199254740993n,ack_pending:0};
  assert.deepEqual(summaryConsumerCounts(scope,{phase:"ready",resource}),{pending:"9007199254740993",ackPending:"0"});
  for(const state of [{phase:"loading",resource},{phase:"error",resource},{phase:"ready",resource:{...resource,name:"other"}},{phase:"ready",resource:{...resource,stream:"other"}},{phase:"ready",resource:null}])assert.deepEqual(summaryConsumerCounts(scope,state),{pending:null,ackPending:null});
  assert.deepEqual(summaryConsumerCounts(scope,{phase:"ready",resource:{...scope,pending:-1,ack_pending:undefined}}),{pending:null,ackPending:null});
});
