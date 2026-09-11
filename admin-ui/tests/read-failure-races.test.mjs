import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueDeclaration} from "../src/queue-declaration.mjs";
import {createConsumerDetail} from "../src/consumer-detail.mjs";
import {createStreamRead} from "../src/stream-detail.mjs";
import {createNodes} from "../src/nodes.mjs";
import {createOverview} from "../src/overview.mjs";

const cases=[
  ["Queue declaration",api=>createQueueDeclaration(api,"Q"),{queue:"Q",revision:"r",plan:{queue:"Q",revision:"r",stream:{name:"S"}}}],
  ["Consumer detail",api=>createConsumerDetail(api,"S","C"),{stream:"S",name:"C",mode:"pull",filter_subjects:null}],
  ["Stream detail",api=>createStreamRead(api,"S"),{name:"S",subjects:["events"]}],
  ["Stream Consumer collection",api=>createStreamRead(api,"S",true),{items:[],total:0,offset:0,limit:50}],
  ["Node collection",createNodes,{nodes:[],total:0}],
];
for(const [name,create,body] of cases){
  test(`${name}: obsolete failures cannot replace loading, ready or cleared states`,async()=>{
    for(const status of [401,404,503])for(const phase of ["loading","ready","idle"]){
      const pending=[];
      const api={request(path,options){assert.equal(options.method,undefined);return new Promise((resolve,reject)=>pending.push({resolve,reject,signal:options.signal}));}};
      const model=create(api),old=model.load();
      let latest;
      if(phase==="idle")model.clear();else latest=model.load();
      assert.equal(pending[0].signal.aborted,true);
      if(phase==="ready"){
        pending[1].resolve({body,headers:new Headers({ETag:'"7"'})});await latest;
      }
      const before=model.snapshot();assert.equal(before.phase,phase);
      let updates=0;const unsubscribe=model.subscribe(()=>updates++);
      // Simulate a transport that ignores abort and fails after a route change.
      pending[0].reject(Object.assign(new Error("obsolete failure"),{status,code:"not_found"}));await old;
      assert.equal(model.snapshot(),before);assert.equal(updates,0);
      if(phase==="loading"){
        pending[1].resolve({body,headers:new Headers({ETag:'"8"'})});await latest;
        assert.equal(model.snapshot().phase,"ready");assert.equal(updates,1);
      }
      unsubscribe();
    }
  });
}

test("Overview independent sources suppress both late authorization failures after replacement or clear",async()=>{
  for(const cleared of [false,true]){
    const pending=[];
    const model=createOverview({request:()=>new Promise((resolve,reject)=>pending.push({resolve,reject}))});
    const old=model.load();
    if(cleared)model.clear();else{
      const latest=model.load();
      pending[2].resolve({body:{name:"management",version:"test",jetstream:{}}});
      pending[3].resolve({body:{items:[],total:0,offset:0,limit:1}});
      await latest;
    }
    const before=model.snapshot();let updates=0;const unsubscribe=model.subscribe(()=>updates++);
    pending[0].reject(Object.assign(new Error("old denial"),{status:401}));
    pending[1].reject(Object.assign(new Error("old denial"),{status:403}));
    await old;assert.equal(model.snapshot(),before);assert.equal(updates,0);
    assert.equal(before.info.phase,cleared?"idle":"ready");assert.equal(before.queues.phase,cleared?"idle":"ready");
    unsubscribe();
  }
});
