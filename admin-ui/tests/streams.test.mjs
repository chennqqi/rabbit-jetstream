import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueList} from "../src/queue-list.mjs";
import {createStreamRead} from "../src/stream-detail.mjs";
import {readRoute, streamListURL, streamDetailURL} from "../src/routes.mjs";

test("Stream routes round trip lists and Consumer filters, reject unsupported parameters", () => {
  const query = {q:"external", order:"desc", offset:100, limit:25};
  assert.deepEqual(readRoute(new URL(streamListURL(query), "http://localhost")), {kind:"streams", query});
  assert.deepEqual(readRoute(new URL(streamDetailURL("external")+"?offset=50&limit=25&q=x&mode=push&order=desc", "http://localhost")), {kind:"stream", name:"external", query:{offset:50, limit:25,q:"x",mode:"push",order:"desc"}});
  for (const suffix of ["?token=x", "?mode=invalid", "?offset=1&offset=2", "?limit=201", "?q=x&q=y"]) assert.equal(readRoute(new URL(streamDetailURL("external")+suffix,"http://localhost")).kind,"invalid");
  assert.throws(() => streamDetailURL("../x"));
});

test("Stream Consumer queries reach server and rejected queries never appear empty", async () => {
  const api={request:async path=>{
    const params=new URL(path,"http://localhost").searchParams;
    assert.equal(params.get("q"),"billing");assert.equal(params.get("mode"),"push");assert.equal(params.get("order"),"desc");assert.equal(params.get("sort"),"name");assert.equal(params.get("offset"),"100");
    return {body:{items:[],total:75,offset:75,limit:25}};
  }};
  const model=createStreamRead(api,"s",true);await model.load({q:"billing",mode:"push",order:"desc",offset:100,limit:25});assert.equal(model.snapshot().resource.total,75);
  api.request=async()=>{throw Object.assign(new Error(),{status:400});};await model.load();assert.equal(model.snapshot().failure,"invalid-query");assert.equal(model.snapshot().resource,null);
});

test("Stream collection uses server filtering and preserves exact large counts", async () => {
  let path;
  const model = createQueueList({request:async value => {path=value;return {body:{items:[{name:"external",messages:18446744073709551615n}],total:1,offset:0,limit:50}};}}, "streams");
  await model.load({q:"external"});
  assert.ok(path.startsWith("/api/v1/streams?"));
  assert.equal(new URL(path,"http://localhost").searchParams.get("sort"),"name");
  assert.equal(model.snapshot().page.items[0].messages,18446744073709551615n);
});

test("Stream detail uses exact lookup and clears stale data after backend failure", async () => {
  const api = {request:async path => {assert.equal(path,"/api/v1/streams/external");return {body:{name:"external",subjects:null,messages:9007199254740993n}};}};
  const model = createStreamRead(api,"external");await model.load();assert.equal(model.snapshot().resource.messages,9007199254740993n);
  api.request=async()=>{throw Object.assign(new Error("private"),{status:503});};await model.load();
  assert.equal(model.snapshot().failure,"unavailable");assert.equal(model.snapshot().resource,null);assert.equal(model.snapshot().readAt,null);
  api.request=async()=>{throw Object.assign(new Error(),{status:404,code:"not_found"});};await model.load();assert.equal(model.snapshot().failure,"missing");
});

test("Stream Consumer page rejects mismatched stream, duplicate names and unsafe links", async () => {
  const row={stream:"s",name:"c",mode:"pull"};
  for (const items of [[{...row,stream:"other"}],[{...row,name:"a/b"}],[row,row]]) {
    const model=createStreamRead({request:async()=>({body:{items,total:items.length,offset:0,limit:50}})},"s",true);
    await model.load();assert.equal(model.snapshot().failure,"invalid");assert.equal(model.snapshot().resource,null);
  }
});

test("Stream Consumer page preserves clamped empty page and ignores late responses", async () => {
  let finish;
  const api={request:async path=>{assert.match(path,/offset=100&limit=25/);return {body:{items:[],total:75,offset:75,limit:25}};}};
  const model=createStreamRead(api,"s",true);await model.load({offset:100,limit:25});assert.equal(model.snapshot().resource.total,75);
  api.request=()=>new Promise(resolve=>{finish=resolve;});const pending=model.load();model.clear();finish({body:{items:[],total:0,offset:0,limit:50}});await pending;assert.equal(model.snapshot().phase,"idle");
});
