import test from "node:test";
import assert from "node:assert/strict";
import {createQueueDelete,validDeletePreview} from "../src/queue-delete.mjs";
import {readRoute} from "../src/routes.mjs";

const observation=()=>({queue:"orders",stream:"RJSQ_orders",base_revision:'"7"',observed_at:"2026-09-10T01:00:00Z",stream_present:true,ownership:"matching",messages:0,consumers:3,requires_force:false,blocked:false});
const response=body=>({body,headers:new Headers({ETag:'"7"'})});
function fixture({preview=observation(),write,error,canStart}={}){
  const calls=[];
  const api={async request(url,options={}){
    calls.push({url,...options});
    if(options.method==="DELETE"){
      if(error)throw error;
      return write?write():response({queue:"orders",stream:"RJSQ_orders",status:"deleted",blocked:false,forced:url.includes("force=true"),messages:0});
    }
    if(url.endsWith("/delete-preview"))return response(preview);
    return response({queue:"orders",plan:{queue:"orders",stream:{name:"RJSQ_orders"}}});
  }};
  return {model:createQueueDelete(api,"orders",{canStart}),calls,api};
}
function confirm(model){model.type("orders");model.acknowledge(true);}

test("deletion routes are exact and reject queries",()=>{
  assert.deepEqual(readRoute({pathname:"/admin/queues/by-name/orders/delete"}),{kind:"delete-queue",name:"orders"});
  assert.equal(readRoute({pathname:"/admin/queues/by-name/orders/delete",search:"?force=true"}).kind,"invalid");
  assert.equal(readRoute({pathname:"/admin/queues/by-name/orders.eu/delete"}).kind,"invalid");
});

test("preflight validates identity, exact numbers, presence and consistent blockers",()=>{
  const p=observation();assert.equal(validDeletePreview(p,"orders",'"7"','"7"'),true);
  for(const change of [{queue:"other"},{stream:"other"},{base_revision:'"8"'},{observed_at:""},{messages:-1},{messages:9007199254740992},{consumers:null},{ownership:"unobserved"},{blocked:true},{requires_force:true}])assert.equal(validDeletePreview({...p,...change},"orders",'"7"','"7"'),false,JSON.stringify(change));
  assert.equal(validDeletePreview({...p,messages:18446744073709551615n,requires_force:true,blocked:true,reason:"messages"},"orders",'"7"','"7"'),true);
  const absent={...p,stream_present:false,ownership:"unobserved"};delete absent.messages;delete absent.consumers;
  assert.equal(validDeletePreview(absent,"orders",'"7"','"7"'),true);
  assert.equal(validDeletePreview({...absent,messages:0},"orders",'"7"','"7"'),false);
  assert.equal(validDeletePreview(p,"orders",'"7"','"8"'),false);
});

test("exact typed confirmation, acknowledgment and current preview required; duplicate writes blocked",async()=>{
  const {model,calls}=fixture();await model.submit();assert.equal(calls.length,0);
  await model.preview();assert.equal(model.snapshot().force,false);
  model.type(" orders");model.acknowledge(true);await model.submit();assert.equal(calls.length,2);
  confirm(model);assert.equal(model.canSubmit(),true);
  await model.submit();await model.submit();assert.equal(model.snapshot().phase,"accepted");
  const writes=calls.filter(c=>c.method==="DELETE");assert.equal(writes.length,1);
  assert.equal(writes[0].url,"/api/v1/queues/orders?force=false");assert.equal(writes[0].headers["If-Match"],'"7"');
  assert.equal(writes[0].headers["X-RJS-Confirm-Queue"],"orders");assert.ok(writes[0].headers["X-Request-ID"]);
  await model.preview();assert.equal(calls.length,3);
});

test("force and refresh invalidate prior confirmation; ownership cannot be forced",async()=>{
  const p={...observation(),messages:2,requires_force:true,blocked:true,reason:"messages"};
  const {model}=fixture({preview:p});await model.preview();confirm(model);assert.equal(model.canSubmit(),false);
  model.force(true);assert.equal(model.snapshot().typed,"");assert.equal(model.snapshot().acknowledged,false);
  confirm(model);assert.equal(model.canSubmit(),true);await model.preview();assert.equal(model.snapshot().typed,"");assert.equal(model.snapshot().force,false);
  for(const ownership of ["unmarked","different"]){const foreign=fixture({preview:{...p,ownership}}).model;await foreign.preview();foreign.force(true);confirm(foreign);assert.equal(foreign.canSubmit(),false);}
});

test("all submitted errors remain uncertain, with no retry even after reads",async()=>{
  for(const error of [new Error("network"),{status:409,kind:"http"},{status:401,kind:"http"},{status:503,kind:"http"}]){
    const {model,calls}=fixture({error});await model.preview();confirm(model);await model.submit();
    assert.equal(model.snapshot().phase,"uncertain");await model.inspect();assert.equal(model.snapshot().phase,"uncertain");
    await model.submit();await model.preview();model.cancel();assert.equal(model.snapshot().phase,"uncertain");
    assert.equal(calls.filter(c=>c.method==="DELETE").length,1);assert.ok(model.snapshot().inspection);
  }
});

test("receiving attempt with no effects cannot unlock an unknown DELETE",async()=>{
  const mutation={schemaVersion:"rjs.mutation-evidence.v1",scope:"receiving-attempt",phase:"audit_intent",resourceEffects:"none"};
  const {model,calls}=fixture({error:{status:503,body:{error:{mutation:{...mutation,token:"secret"}}}}});
  await model.preview();confirm(model);await model.submit();
  assert.equal(model.snapshot().phase,"uncertain");assert.deepEqual(model.snapshot().error.mutation,mutation);
  await model.inspect();await model.submit();assert.equal(model.canSubmit(),false);
  assert.equal(calls.filter(c=>c.method==="DELETE").length,1);
});

test("malformed success is uncertain; refresh failure removes old preview",async()=>{
  const {model}=fixture({write:()=>response({queue:"other",status:"deleted"})});await model.preview();confirm(model);await model.submit();assert.equal(model.snapshot().phase,"uncertain");
  const f=fixture();await f.model.preview();confirm(f.model);f.api.request=async()=>{throw {status:409};};await f.model.preview();
  assert.equal(f.model.snapshot().phase,"preview-error");assert.equal(f.model.snapshot().preview,undefined);assert.equal(f.model.canSubmit(),false);
});

test("session fences block preview and submit; discarded late reads cannot revive evidence",async()=>{
  let allowed=false;const {model,calls}=fixture({canStart:()=>allowed});await model.preview();assert.equal(calls.length,0);
  allowed=true;await model.preview();confirm(model);allowed=false;await model.submit();assert.equal(calls.length,2);
  let resolve;const late=createQueueDelete({request:()=>new Promise(r=>{resolve=r;})},"orders");const pending=late.preview();late.discard();resolve(response({queue:"orders",plan:{queue:"orders",stream:{name:"RJSQ_orders"}}}));await pending;assert.equal(late.snapshot().phase,"idle");
});

test("pending write is single flight and accepted inspection stays accepted",async()=>{
  let resolve;const {model,calls}=fixture({write:()=>new Promise(r=>{resolve=r;})});await model.preview();confirm(model);
  const pending=model.submit();await model.submit();await model.preview();assert.equal(calls.filter(c=>c.method==="DELETE").length,1);
  assert.throws(()=>model.discard(),/pending deletion/);
  resolve(response({queue:"orders",stream:"RJSQ_orders",status:"noop",blocked:false,forced:false,messages:0}));await pending;
  await model.inspect();assert.equal(model.snapshot().phase,"accepted");assert.equal(model.canSubmit(),false);
});
