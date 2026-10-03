import test from "node:test";
import assert from "node:assert/strict";
import {createConnectionDetail} from "../src/connection-detail.mjs";
import {createAPI} from "../src/api.mjs";

const body={node_id:"N",observed_at:"2026-09-10T00:00:00.123456789Z",read_at:"2026-09-10T00:00:00.123456789Z",item:{cid:7,in_msgs:9007199254740993n}};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(){
  const timers=new Map(),calls=[];let next=0,reply=async()=>({body});
  const view=createConnectionDetail({request:(path,options)=>{calls.push({path,options});return reply();}},"N","7",{schedule:(fn,ms)=>{timers.set(++next,{fn,ms});return next;},unschedule:id=>timers.delete(id)});
  return {view,timers,calls,setReply:fn=>reply=fn,delay:()=>[...timers.values()][0]?.ms,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}

test("connection detail completion-based backoff preserves history and resets after recovery",async()=>{
  const f=fixture();f.view.refresh.start();await settle();const original=f.view.snapshot();assert.equal(f.delay(),10000);
  f.setReply(async()=>{throw {status:503};});
  for(const delay of [20000,40000,60000,60000]){await f.tick();assert.equal(f.delay(),delay);assert.equal(f.view.snapshot().detail,original.detail);assert.equal(f.view.snapshot().readAt,original.readAt);}
  f.setReply(async()=>({body}));await f.tick();assert.equal(f.delay(),10000);assert.equal(f.view.snapshot().phase,"ready");
  assert.ok(f.calls.every(call=>call.path==="/api/v1/nodes/N/connections/7"&&!call.options.method));f.view.clear();assert.equal(f.timers.size,0);
});

test("connection detail hidden/manual modes and preference changes never queue parallel reads",async()=>{
  const f=fixture();f.view.refresh.setInterval(0);f.view.refresh.start(true);assert.equal(f.calls.length,0);await f.view.refresh.refresh();assert.equal(f.calls.length,0);
  f.view.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
  let finish;f.setReply(()=>new Promise(resolve=>{finish=resolve;}));const pending=f.view.refresh.refresh();assert.equal(f.calls.length,2);
  f.view.refresh.setInterval(30000);await f.view.refresh.refresh();assert.equal(f.calls.length,2);assert.equal(f.timers.size,0);
  f.view.refresh.visibility(true);finish({body});await pending;assert.equal(f.timers.size,0);
  f.view.refresh.visibility(false);assert.equal(f.calls.length,2);assert.equal(f.delay(),30000);
  f.view.refresh.setInterval(0);assert.equal(f.timers.size,0);assert.equal(f.view.refresh.setInterval(123),false);assert.equal(f.timers.size,0);
  f.view.clear();f.view.refresh.visibility(false);assert.equal(f.timers.size,0);
});

test("detail real JSON decoding rejects malformed responses without reviving prior history",async()=>{
  const valid='{"node_id":"N","observed_at":"2026-09-10T00:00:00.123456789Z","read_at":"2026-09-10T00:00:00.123456789Z","item":{"cid":18446744073709551615,"in_msgs":9007199254740993}}';
  let raw=valid,status=200;
  const api=createAPI({origin:"http://localhost",fetch:async()=>new Response(raw,{status})});
  const view=createConnectionDetail(api,"N","18446744073709551615");view.refresh.setInterval(0);view.refresh.start();await settle();
  assert.equal(view.snapshot().detail.item.in_msgs,9007199254740993n);
  for(const malformed of ['{"',valid+' {}','',valid.replace('9007199254740993','1e999')]){
    raw=malformed;await view.refresh.refresh();assert.equal(view.snapshot().failure,"invalid");assert.equal(view.snapshot().detail,null);assert.equal(view.snapshot().readAt,null);
    status=503;raw='{"error":{"code":"connections_unavailable"}}';await view.refresh.refresh();assert.equal(view.snapshot().detail,null);
    status=200;raw=valid;await view.refresh.refresh();assert.equal(view.snapshot().phase,"ready");
  }
  status=403;raw='{"';await view.refresh.refresh();assert.equal(view.snapshot().failure,"denied");assert.equal(view.snapshot().detail,null);view.clear();
});
