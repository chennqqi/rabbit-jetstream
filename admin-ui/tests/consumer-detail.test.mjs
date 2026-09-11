import {test} from "node:test";
import assert from "node:assert/strict";
import {createConsumerDetail} from "../src/consumer-detail.mjs";
import {consumerDetailURL,queueConsumersURL,readRoute} from "../src/routes.mjs";
const body={stream:"S",name:"same",mode:"pull",filter_subjects:[],pending:9007199254740993n};

test("exact lookup uses full identity and no list enumeration",async()=>{
  const calls=[];
  const model=createConsumerDetail({request:async path=>{calls.push(path);return {body};}},"S","same");
  await model.load();assert.deepEqual(calls,["/api/v1/streams/S/consumers/same"]);
  assert.equal(model.snapshot().resource.pending,9007199254740993n);
  const wrong=createConsumerDetail({request:async()=>({body})},"other","same");
  await wrong.load();assert.equal(wrong.snapshot().failure,"invalid");
});
test("refresh reflects mode change and disappearance without stale metrics",async()=>{
  let next=body;
  const api={request:async()=>{if(next instanceof Error)throw next;return {body:next};}};
  const model=createConsumerDetail(api,"S","same");await model.load();
  next={...body,mode:"push"};await model.load();assert.equal(model.snapshot().resource.mode,"push");
  next=Object.assign(new Error("missing"),{status:404,code:"not_found"});await model.load();
  assert.equal(model.snapshot().failure,"missing");assert.equal(model.snapshot().resource,null);assert.equal(model.snapshot().readAt,null);
});
test("authorization and unavailability do not imply missing",async()=>{
  for(const [status,code,failure] of [[401,"","denied"],[403,"","denied"],[404,"read_api_disabled","disabled"],[503,"","unavailable"]]){
    const model=createConsumerDetail({request:async()=>{throw Object.assign(new Error("private"),{status,code});}},"S","same");await model.load();
    assert.equal(model.snapshot().failure,failure);assert.equal(model.snapshot().resource,null);
  }
});
test("clearing suppresses late response",async()=>{
  let resolve;const model=createConsumerDetail({request:()=>new Promise(done=>resolve=done)},"S","same");
  const pending=model.load();model.clear();resolve({body});await pending;assert.equal(model.snapshot().phase,"idle");
});

test("invalid display-field types cannot replace a retained Consumer with an unrenderable object",async()=>{
  for(const field of ["durable","ack_policy"]){
    for(const value of [{},[],null,42,true]){
      let response=body;const model=createConsumerDetail({request:async()=>({body:response})},"S","same",{retainOnRefresh:true});
      await model.load();response={...body,[field]:value};await model.load();
      assert.equal(model.snapshot().failure,"invalid");assert.equal(model.snapshot().resource,null);assert.equal(model.snapshot().readAt,null);
    }
  }
});
test("Consumer routes and collection query preserve identity and paging",()=>{
  assert.deepEqual(readRoute(new URL(consumerDetailURL("S","same"),"http://localhost")),{kind:"consumer",stream:"S",name:"same"});
  const query={q:"subject.*",mode:"push",order:"desc",offset:150,limit:50};
  assert.deepEqual(readRoute(new URL(queueConsumersURL("new",query),"http://localhost")),{kind:"queue",name:"new",consumerQuery:query});
  for(const path of ["/admin/streams/S/consumers/%2F","/admin/streams/S/consumers/%","/admin/queues/by-name/new?cmode=other","/admin/queues/by-name/new?cq=a&cq=b","/admin/streams/S/consumers/c?token=secret"])assert.equal(readRoute(new URL(path,"http://localhost")).kind,"invalid");
});
