import test from "node:test";
import assert from "node:assert/strict";
import {createConnectionDetail} from "../src/connection-detail.mjs";
import {nodeConnectionURL,readRoute} from "../src/routes.mjs";
const cid="18446744073709551615",time="2026-09-10T08:00:00.123456789+08:00";
const body={node_id:"N",observed_at:time,read_at:time,item:{cid:BigInt(cid),in_msgs:9007199254740993n,name:"private"},total:200};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(){let reply=async()=>({body});const calls=[];const model=createConnectionDetail({request:(path,options)=>{calls.push({path,options});return reply();}},"N",cid);model.refresh.setInterval(0);return {model,calls,reply(fn){reply=fn;},async start(){model.refresh.start();await settle();}};}

test("exact connection route preserves list context and never rounds CID",()=>{
  const url=nodeConnectionURL("N",cid,{offset:200,limit:25});
  assert.deepEqual(readRoute(new URL(url,"http://localhost")),{kind:"node-connection",id:"N",cid,query:{offset:200,limit:25}});
  assert.equal(readRoute(new URL("/admin/nodes/N/connections/0007","http://localhost")).cid,"7");
  for(const value of [0,-1,"+1","1.2",18446744073709551615,"18446744073709551616",null,{},"x"]){assert.throws(()=>nodeConnectionURL("N",value));}
  for(const suffix of ["7?q=x","7?limit=0","7?offset=1&offset=2","0","-1","18446744073709551616"]){assert.equal(readRoute(new URL(`/admin/nodes/N/connections/${suffix}`,"http://localhost")).kind,"invalid");}
});

test("detail reads only exact identity and projects lossless counters and source times",async()=>{
  const f=fixture();await f.start();const state=f.model.snapshot();
  assert.equal(f.calls.length,1);assert.equal(f.calls[0].path,`/api/v1/nodes/N/connections/${cid}`);
  assert.equal(state.detail.item.cid,BigInt(cid));assert.equal(state.detail.item.in_msgs,9007199254740993n);
  assert.equal(state.detail.item.name,undefined);assert.equal(state.detail.total,undefined);assert.equal(state.detail.read_at,time);assert.equal(state.detail.item.out_msgs,undefined);f.model.clear();
});

test("detail errors retain only unavailable history and distinguish missing node from missing CID",async()=>{
  for(const [error,failure]of [[{status:404,code:"connection_not_found"},"missing"],[{status:404,code:"not_found"},"node_missing"],[{status:404,code:"read_api_disabled"},"disabled"],[{status:403},"denied"],[{status:401},"denied"],[{status:409},"ambiguous"],[{status:400},"query"],[{kind:"invalid-response"},"invalid"]]){
    const f=fixture();await f.start();const before=f.model.snapshot();
    f.reply(async()=>{throw {status:503};});await f.model.refresh.refresh();assert.equal(f.model.snapshot().detail,before.detail);assert.equal(f.model.snapshot().readAt,before.readAt);
    f.reply(async()=>{throw error;});await f.model.refresh.refresh();assert.equal(f.model.snapshot().failure,failure);assert.equal(f.model.snapshot().detail,null);assert.equal(f.model.snapshot().readAt,null);
    f.reply(async()=>{throw {status:503};});await f.model.refresh.refresh();assert.equal(f.model.snapshot().detail,null);
    f.reply(async()=>({body}));await f.model.refresh.refresh();assert.equal(f.model.snapshot().phase,"ready");f.model.clear();
  }
});

test("mismatched or invalid detail evidence clears the entire observation",async()=>{
  for(const patch of [{node_id:"other"},{observed_at:"2026-02-30T00:00:00Z"},{read_at:"bad"},{item:{cid:1}},{item:{cid:BigInt(cid),in_msgs:-1}},{item:{cid:BigInt(cid),subscriptions:4294967296}},{item:{cid:Number(cid)}},{item:null}]){
    const f=fixture();await f.start();f.reply(async()=>({body:{...body,...patch}}));await f.model.refresh.refresh();assert.equal(f.model.snapshot().failure,"invalid");assert.equal(f.model.snapshot().detail,null);f.model.clear();
  }
});

test("detail navigation cancels pending reads and fences late success or failure",async()=>{
  for(const fail of [false,true]){
    const f=fixture();let resolve,reject;f.reply(()=>new Promise((yes,no)=>{resolve=yes;reject=no;}));f.model.refresh.start();await f.model.refresh.refresh();assert.equal(f.calls.length,1);
    f.model.clear();assert.equal(f.calls[0].options.signal.aborted,true);if(fail)reject({status:503});else resolve({body});await settle();assert.equal(f.model.snapshot().phase,"idle");assert.equal(f.model.snapshot().detail,null);
  }
});
