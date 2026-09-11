import test from "node:test";
import assert from "node:assert/strict";
import {createStreamRefresh} from "../src/stream-refresh.mjs";
import {createConsumerCollectionRefresh} from "../src/consumer-collection-refresh.mjs";
import {createAPI} from "../src/api.mjs";
const body={name:"S",subjects:[],messages:9007199254740993n};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(name="S"){
  const timers=new Map(),calls=[];let id=0,reply=async()=>({body});
  const api={request:(path,options)=>{calls.push({path,options});return reply(path);}};
  const options={schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:id=>timers.delete(id)};
  const view=createStreamRefresh(api,name,options);
  return {...view,api,options,timers,calls,setReply:fn=>reply=fn,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}
test("Stream periodic reads retain exact historical values/time, back off and recover",async()=>{
  const f=fixture();f.refresh.start();await settle();const before=f.model.snapshot();
  f.setReply(async()=>{throw {status:503};});await f.tick();assert.equal(f.model.snapshot().resource,before.resource);assert.equal(f.model.snapshot().readAt,before.readAt);assert.equal(f.model.snapshot().resource.messages,9007199254740993n);assert.equal([...f.timers.values()][0].ms,20000);
  f.setReply(async()=>({body:{...body,messages:0}}));await f.tick();assert.equal(f.model.snapshot().resource.messages,0);assert.equal([...f.timers.values()][0].ms,10000);assert.ok(f.calls.every(v=>v.path==="/api/v1/streams/S"&&!v.options.method));f.clear();
});
test("Stream missing, denied, disabled and invalid responses clear configuration and original time",async()=>{
  for(const error of [{status:404,code:"not_found"},{status:401},{status:403},{status:404,code:"read_api_disabled"},{status:200,kind:"invalid-response"},"identity","subjects"]){
    const f=fixture();f.refresh.start();await settle();f.setReply(async()=>{if(error==="identity")return {body:{...body,name:"other"}};if(error==="subjects")return {body:{...body,subjects:[{}]}};throw error;});await f.tick();assert.equal(f.model.snapshot().resource,null);assert.equal(f.model.snapshot().readAt,null);f.clear();
  }
});
test("Stream hidden/manual/single-flight and navigation fence late successes and failures",async()=>{
  for(const fail of [false,true]){
    const f=fixture();f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
    let finish,reject;f.setReply(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));const pending=f.refresh.refresh();await f.refresh.refresh();assert.equal(f.calls.length,2);f.clear();assert.equal(f.calls[1].options.signal.aborted,true);
    const next=fixture("other");next.setReply(async()=>({body:{...body,name:"other"}}));next.refresh.start();await settle();if(fail)reject({status:503});else finish({body});await pending;
    assert.equal(f.model.snapshot().resource,null);assert.equal(f.timers.size,0);assert.equal(next.model.snapshot().resource.name,"other");next.clear();
  }
});
test("Consumer query replacement does not restart or clear independent Stream observation",async()=>{
  const f=fixture();f.refresh.start();await settle();const snapshot=f.model.snapshot(),timer=[...f.timers.keys()][0];
  f.setReply(async()=>({body:{items:[],total:0,offset:0,limit:50}}));
  for(const q of ["first","second"]){const collection=createConsumerCollectionRefresh(f.api,"stream","S",{q,limit:50},f.options);collection.refresh.start();await settle();collection.clear();assert.equal(f.model.snapshot(),snapshot);assert.equal(f.timers.has(timer),true);}
  assert.equal(f.calls.filter(v=>v.path==="/api/v1/streams/S").length,1);f.clear();
});
test("unparseable JSON clears retained Stream and Stream collection through the real API decoder",async()=>{
  for(const collection of [false,true]){
    let malformed=false;
    const api=createAPI({origin:"http://localhost",fetch:async()=>new Response(malformed?'{} trailing':JSON.stringify(collection?{items:[],total:0,offset:0,limit:50}:{...body,messages:0}),{status:200})});
    const view=collection?createConsumerCollectionRefresh(api,"stream","S",{limit:50}):createStreamRefresh(api,"S");
    view.refresh.setInterval(0);view.refresh.start();await settle();assert.equal(view.model.snapshot().phase,"ready");
    malformed=true;await view.refresh.refresh();assert.equal(view.model.snapshot().failure,"invalid");assert.equal(view.model.snapshot().resource,null);assert.equal(view.model.snapshot().readAt,null);view.clear();
  }
});
