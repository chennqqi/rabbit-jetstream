import {test} from "node:test";
import assert from "node:assert/strict";
import {parseJSON, stringifyJSON, createAPI, latestRead, APIError} from "../src/api.mjs";

test("Go integer boundaries survive decode, edit and encode", () => {
  const text = '{"signed":9223372036854775807,"unsigned":18446744073709551615,"min":-9223372036854775808,"safe":9007199254740991,"decimal":0.125}';
  const value = parseJSON(text);
  assert.equal(value.unsigned, 18446744073709551615n);
  assert.equal(typeof value.safe, "number");
  assert.equal(stringifyJSON(value), text);
  assert.throws(() => stringifyJSON({alreadyRounded: 9223372036854775807}), /Unsafe/);
});

test("JSON grammar, escaping and prototype safety", () => {
  const text = '{"__proto__":{"polluted":true},"x":[true,false,null,"a\\\"b",{},[]]}';
  assert.deepEqual(parseJSON(text), JSON.parse(text));
  assert.equal({}.polluted, undefined);
  for (const malformed of ["", "[1,]", '{"x":1,}', "01", "1e", "truefalse", '"unterminated', '{x:1}', "NaN", "1e400", "9007199254740993e0"]) assert.throws(() => parseJSON(malformed), SyntaxError);
  assert.throws(() => parseJSON("[".repeat(102) + "0" + "]".repeat(102)), /nesting/);
});

test("write preserves original ETag, exact body, memory token and makes one request", async () => {
  const calls = [];
  const api = createAPI({origin: "http://localhost", fetch: async (...args) => { calls.push(args); return new Response('{"status":"ready"}', {headers: {ETag: '"12"'}}); }});
  api.setToken("secret");
  const result = await api.request("/api/v1/queues/orders", {method: "PUT", headers: {"If-Match": '"7"'}, body: {max: 9223372036854775807n}});
  assert.equal(calls.length, 1);
  assert.equal(calls[0][1].headers.get("If-Match"), '"7"');
  assert.equal(calls[0][1].headers.get("Authorization"), "Bearer secret");
  assert.equal(calls[0][1].body, '{"max":9223372036854775807}');
  assert.equal(result.headers.get("ETag"), '"12"');
  api.clearToken(); await api.request("/api/v1/queues");
  assert.equal(calls[1][1].headers.get("Authorization"), null);
  await assert.rejects(api.request("https://outside.test/api/v1/info"), /same-origin/);
  assert.equal(calls.length, 2);
});

test("tenant context is explicit, validated, and fenced across credential changes",async()=>{
  const calls=[];const api=createAPI({origin:"http://localhost",fetch:async(url,options)=>{calls.push({url,headers:options.headers});return new Response("{}");}});
  api.setToken("access");api.setTenant("team-a");
  await api.request("/api/v1/queues");
  assert.equal(calls[0].headers.get("Authorization"),"Bearer access");assert.equal(calls[0].headers.get("X-RJS-Tenant"),"team-a");
  await api.request("/api/v1/session");assert.equal(calls[1].headers.get("X-RJS-Tenant"),null);
  assert.throws(()=>api.setTenant("bad tenant"),TypeError);
  api.setToken("replacement");await api.request("/api/v1/queues");assert.equal(calls[2].headers.get("X-RJS-Tenant"),null);
  api.setTenant("team-b");api.clearToken();await api.request("/api/v1/queues");assert.equal(calls[3].headers.get("X-RJS-Tenant"),null);
});

test("HTTP errors preserve blocked operations, status and correlation without retry", async () => {
  for (const status of [401, 403, 409, 503]) {
    let calls = 0;
    const api = createAPI({origin: "http://localhost", fetch: async () => { calls++; return new Response('{"status":"blocked","operations":[{"blocked":true}]}', {status, headers: {"X-Request-ID": "op-1"}}); }});
    await assert.rejects(api.request("/api/v1/queues/orders", {method: "PUT", body: {}}), error => error instanceof APIError && error.status === status && error.body.operations[0].blocked && error.headers.get("X-Request-ID") === "op-1");
    assert.equal(calls, 1);
  }
});

test("unreadable success and connection failure retain write uncertainty", async () => {
  for (const fetch of [async () => new Response("broken"), async () => { throw new Error("private network details"); }]) {
    const api = createAPI({origin: "http://localhost", fetch});
    await assert.rejects(api.request("/api/v1/queues/orders", {method: "PUT", body: {}}), error => error.uncertain && !error.message.includes("private"));
  }
});

test("timeout cancels request and is not retried", async () => {
  let calls = 0;
  const api = createAPI({origin: "http://localhost", fetch: (_, options) => { calls++; return new Promise((_, reject) => options.signal.addEventListener("abort", () => reject(options.signal.reason))); }});
  await assert.rejects(api.request("/api/v1/queues", {timeout: 5}), error => error.kind === "aborted" && !error.uncertain);
  assert.equal(calls, 1);
});

test("late resource response cannot replace newer navigation", async () => {
  const pending = [];
  const reader = latestRead({request: () => new Promise(resolve => pending.push(resolve))});
  const old = reader.run("/api/v1/queues/old");
  const current = reader.run("/api/v1/queues/current");
  pending[1]({body: "current"});
  assert.equal((await current).result.body, "current");
  pending[0]({body: "old"});
  assert.deepEqual(await old, {stale: true});
  const closing = reader.run("/api/v1/queues/closed"); reader.cancel(); pending[2]({});
  assert.deepEqual(await closing, {stale: true});
});

test("diagnostic download is same-origin, authenticated and strictly bounded",async()=>{
  const calls=[];const api=createAPI({origin:"http://localhost",fetch:async(url,options)=>{calls.push({url,options});return new Response(new Uint8Array([80,75,3,4]),{headers:{"Content-Type":"application/zip","Content-Length":"4"}});}});api.setToken("secret");
  const result=await api.download("/api/v1/diagnostics/jobs/0123456789abcdef0123456789abcdef/download",{maxBytes:4});assert.equal(result.size,4);assert.equal(result.blob.type,"application/zip");assert.equal(calls[0].options.headers.get("Authorization"),"Bearer secret");assert.equal(calls[0].options.headers.get("Accept"),"application/zip");
  await assert.rejects(api.download("/api/v1/diagnostics/jobs/x/download?token=secret"),TypeError);
});

test("diagnostic download rejects wrong type, oversized declarations and length mismatch",async()=>{
  for(const response of [new Response("zip",{headers:{"Content-Type":"text/plain","Content-Length":"3"}}),new Response("zip",{headers:{"Content-Type":"application/zip","Content-Length":"9"}}),new Response("zip",{headers:{"Content-Type":"application/zip","Content-Length":"2"}})]){
    const api=createAPI({origin:"http://localhost",fetch:async()=>response});await assert.rejects(api.download("/api/v1/diagnostics/jobs/x/download",{maxBytes:8}),error=>error instanceof APIError&&error.kind==="invalid-response");
  }
});
