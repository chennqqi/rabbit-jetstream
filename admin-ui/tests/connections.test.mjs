import test from "node:test";
import assert from "node:assert/strict";
import {createConnections} from "../src/connections.mjs";
import {nodeConnectionsURL,readRoute} from "../src/routes.mjs";
const query={offset:0,limit:50},timestamp="2026-09-10T00:00:00.123456789Z";
const body={node_id:"N",offset:0,limit:50,total:201,observed_at:timestamp,read_at:timestamp,items:[{cid:18446744073709551615n,in_msgs:9007199254740993n,name:"secret"}]};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(){const calls=[];let reply=async()=>({body});const model=createConnections({request:(path,options)=>{calls.push({path,options});return reply();}},"N",query);model.refresh.setInterval(0);return {model,calls,reply:fn=>reply=fn,async start(){model.refresh.start();await settle();}};}
test("connection routes retain node/offset/limit and reject unsupported or ambiguous parameters",()=>{
  const url=new URL(nodeConnectionsURL("N",{offset:200,limit:25}),"http://localhost");assert.deepEqual(readRoute(url),{kind:"node-connections",id:"N",query:{offset:200,limit:25}});
  assert.deepEqual(readRoute(new URL(nodeConnectionsURL("N",{offset:0,limit:25,cid:"18446744073709551615"}),"http://localhost")),{kind:"node-connections",id:"N",query:{offset:0,limit:25,cid:"18446744073709551615"}});
  for(const tail of ["?q=x","?offset=1000001","?offset=1&offset=2","?limit=0","?limit=","?offset=-1","?cid=0","?cid=-1","?cid=18446744073709551616","?cid=7&offset=1"]){assert.equal(readRoute(new URL(`/admin/nodes/N/connections${tail}`,"http://localhost")).kind,"invalid");}
  for(const id of ["x y","x\u007f","..","a/b"]){assert.throws(()=>nodeConnectionsURL(id));}
});
test("exact CID search stays a server query and validates its filtered total",async()=>{
  const calls=[],filtered={...body,total:1,items:[body.items[0]]},model=createConnections({request:async(path)=>{calls.push(path);return {body:filtered};}},"N",{offset:0,limit:50,cid:"18446744073709551615"});model.refresh.setInterval(0);model.refresh.start();await settle();assert.equal(calls[0],"/api/v1/nodes/N/connections?offset=0&limit=50&cid=18446744073709551615");assert.equal(model.snapshot().page.total,1);model.clear();
  for(const invalid of [{...filtered,total:2},{...filtered,items:[{cid:7}]}]){const candidate=createConnections({request:async()=>({body:invalid})},"N",{offset:0,limit:50,cid:"18446744073709551615"});candidate.refresh.setInterval(0);candidate.refresh.start();await settle();assert.equal(candidate.snapshot().failure,"invalid");candidate.clear();}
});
test("exact identity search stays out of URL and pages on the server",async()=>{
  const calls=[],reply={...body,total:2,items:[{cid:7}]},model=createConnections({request:async(path,options)=>{calls.push({path,options});return {body:{...reply,offset:options.body.offset,limit:options.body.limit}}; }},"N",query);
  assert.equal(await model.search("account","private account",0,25),true);
  assert.equal(calls[0].path,"/api/v1/nodes/N/connections/search");assert.equal(calls[0].path.includes("private"),false);assert.equal(calls[0].options.method,"POST");assert.deepEqual(calls[0].options.body,{kind:"account",value:"private account",offset:0,limit:25});assert.deepEqual(model.snapshot().identity,{kind:"account",offset:0,limit:25});
  await model.searchPage(1);assert.equal(calls[1].options.body.offset,1);assert.equal(calls[1].options.body.value,"private account");model.clear();
});
test("connection page preserves exact integers and drops unknown metadata",async()=>{
  const f=fixture();await f.start();const page=f.model.snapshot().page;assert.equal(page.total,201);assert.equal(page.items[0].cid,18446744073709551615n);assert.equal(page.items[0].in_msgs,9007199254740993n);assert.equal(page.items[0].name,undefined);assert.equal(page.items[0].out_msgs,undefined);assert.equal(f.calls[0].path,"/api/v1/nodes/N/connections?offset=0&limit=50");f.model.clear();
});
test("connection source timestamps retain nanoseconds without millisecond conversion",async()=>{
  const f=fixture();await f.start();assert.equal(f.model.snapshot().page.observed_at,timestamp);assert.equal(f.model.snapshot().page.read_at,timestamp);f.model.clear();
});
test("unavailable retains original connection page/time; confirmed failure clears it",async()=>{
  for(const error of [{status:401},{status:403},{status:404,code:"not_found"},{status:404,code:"read_api_disabled"},{status:409},{status:400},{kind:"invalid-response"}]){
    const f=fixture();await f.start();const old=f.model.snapshot();f.reply(async()=>{throw {status:503};});await f.model.refresh.refresh();assert.equal(f.model.snapshot().page,old.page);assert.equal(f.model.snapshot().readAt,old.readAt);
    f.reply(async()=>{throw error;});await f.model.refresh.refresh();assert.equal(f.model.snapshot().page,null);assert.equal(f.model.snapshot().readAt,null);f.model.clear();
  }
});
test("invalid connection identities, counters, times and pagination reject whole page",async()=>{
  for(const invalid of [{...body,node_id:"other"},{...body,offset:1},{...body,total:0},{...body,observed_at:"bad"},{...body,items:[{cid:0}]},{...body,items:[{cid:2},{cid:1}]},{...body,items:[{cid:1},{cid:1}]},{...body,items:[{cid:1,subscriptions:4294967296}]},{...body,items:[{cid:1,in_msgs:-1}]},{...body,items:[{cid:1,pending_bytes:9223372036854775808n}]}]){
    const f=fixture();await f.start();f.reply(async()=>({body:invalid}));await f.model.refresh.refresh();assert.equal(f.model.snapshot().failure,"invalid");assert.equal(f.model.snapshot().page,null);f.model.clear();
  }
});
test("navigation cancels pending connection requests and fences late success/failure",async()=>{
  for(const fail of [false,true]){const f=fixture();let finish,reject;f.reply(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));f.model.refresh.start();await f.model.refresh.refresh();assert.equal(f.calls.length,1);f.model.clear();assert.equal(f.calls[0].options.signal.aborted,true);if(fail)reject({status:503});else finish({body});await settle();assert.equal(f.model.snapshot().phase,"idle");assert.equal(f.model.snapshot().page,null);}
});
