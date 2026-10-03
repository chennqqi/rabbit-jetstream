import test from "node:test";
import assert from "node:assert/strict";
import {createQueueExport} from "../src/queue-export.mjs";
import {createAPI,parseJSON,stringifyJSON} from "../src/api.mjs";

const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"orders",labels:{owner:"sensitive-owner"}},spec:{replicas:1,subjects:["orders.created"],maxPriority:0,retention:{maxMessages:9223372036854775807n},deadLetter:{queue:"failed"}}};
const etag='"9007199254740993"';
function reply(include=false){const body=structuredClone(document);if(!include)delete body.metadata.labels;return {body,headers:new Headers({ETag:etag,"Content-Disposition":'attachment; filename="orders.queue.json"',"X-RJS-Export-Scope":"single-queue-declaration","X-RJS-Export-Omitted-Labels":include?"0":"1"})};}
function fixture(){let respond=async()=>reply();const calls=[];const model=createQueueExport({request:(path,options)=>{calls.push({path,options});return respond();}},"orders",document,etag);return {model,calls,set(fn){respond=fn;}};}

test("Queue export is explicit, exact, labels-off by default, and never mutates source",async()=>{
  const f=fixture(),before=stringifyJSON(document);assert.equal(f.calls.length,0);await f.model.prepare();
  assert.equal(f.calls[0].path,"/api/v1/queues/orders/export?include_labels=false");assert.equal(f.calls[0].options.body,undefined);
  let file=f.model.snapshot().file;assert.equal(file.omitted,1);assert.equal(file.etag,etag);assert.equal(parseJSON(file.content).spec.retention.maxMessages,9223372036854775807n);assert.equal(file.content.includes("sensitive-owner"),false);
  assert.equal(parseJSON(file.content).spec.maxPriority,0);assert.equal(parseJSON(file.content).spec.deadLetter.queue,"failed");
  f.set(async()=>reply(true));await f.model.prepare(true);file=f.model.snapshot().file;assert.equal(parseJSON(file.content).metadata.labels.owner,"sensitive-owner");assert.equal(file.omitted,0);assert.equal(stringifyJSON(document),before);
  const change=parseJSON(file.changeContent);assert.equal(change.schema,"rjs.queue-change.v1");assert.equal(change.etag,etag);assert.equal(change.document.metadata.labels.owner,"sensitive-owner");assert.equal(change.document.spec.retention.maxMessages,9223372036854775807n);assert.equal(file.changeName,"orders.queue-change.json");
});

test("Queue export rejects changed provenance, added fields, policy violations and missing sources",async()=>{
  for(const mutate of [r=>{r.headers.set("ETag",'"2"');},r=>{r.headers.delete("X-RJS-Export-Scope");},r=>{r.headers.set("X-RJS-Export-Omitted-Labels","0");},r=>{r.headers.set("Content-Disposition",'attachment; filename="other.json"');},r=>{r.body.spec.retention.maxMessages=1;},r=>{r.body.token="secret";},r=>{r.body.metadata.labels={owner:"sensitive-owner"};}]){
    const f=fixture();await f.model.prepare();f.set(async()=>{const r=reply();mutate(r);return r;});await f.model.prepare();assert.equal(f.model.snapshot().phase,"error");assert.equal(f.model.snapshot().file,null);
  }
  const model=createQueueExport({request(){throw Error("must not read");}},"orders",null,etag);await model.prepare();assert.equal(model.snapshot().failure,"invalid");
});

test("Queue export decoder preserves auth precedence and clears errors before recovery",async()=>{
  let raw=stringifyJSON(reply().body),status=200;
  const api=createAPI({origin:"http://localhost",fetch:async()=>new Response(raw,{status,headers:reply().headers})});
  const model=createQueueExport(api,"orders",document,etag);
  for(const [nextStatus,nextRaw,want]of [[403,"<html>denied</html>","denied"],[200,"{", "invalid"],[404,'{"error":{"code":"proxy_missing"}}',"invalid"],[404,'{"error":{"code":"not_found"}}',"missing"],[409,'{"error":{"code":"export_declaration_unavailable"}}',"changed"],[503,'{"error":{"code":"export_limit"}}',"limit"]]){
    status=200;raw=stringifyJSON(reply().body);await model.prepare();assert.ok(model.snapshot().file);
    status=nextStatus;raw=nextRaw;await model.prepare();assert.equal(model.snapshot().failure,want);assert.equal(model.snapshot().file,null);
  }
  status=200;raw=stringifyJSON(reply().body);await model.prepare();assert.equal(model.snapshot().phase,"ready");
});

test("Queue export cancellation and new policy fence late responses",async()=>{
  const f=fixture();let resolve;f.set(()=>new Promise(done=>{resolve=done;}));
  const old=f.model.prepare();f.model.clear();assert.equal(f.calls[0].options.signal.aborted,true);resolve(reply());await old;assert.equal(f.model.snapshot().phase,"idle");
  const later=f.model.prepare();f.set(async()=>reply(true));await f.model.prepare(true);resolve(reply());await later;assert.equal(f.model.snapshot().file.includeLabels,true);
});
