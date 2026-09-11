import {test} from "node:test";
import assert from "node:assert/strict";
import {createSession} from "../src/session.mjs";
import {createAPI} from "../src/api.mjs";

const identity = {actor: "verified-actor", role: "operator", permissions: ["resources:read"], expires_at: null, resource_read_policy: "authenticated"};
function fixture(request = async () => ({body: identity})) {
  let token = "";
  const calls = [];
  const api = {setToken(value) { token = value; }, clearToken() { token = ""; },
    request(...args) { calls.push(args); return request(...args); }};
  return {session: createSession(api), calls, token: () => token};
}

test("verifies identity through API and keeps token out of snapshots", async () => {
  const f = fixture();
  let updates = 0;
  const unsubscribe = f.session.subscribe(() => updates++);
  await f.session.signIn("secret");
  assert.equal(f.calls[0][0], "/api/v1/session");
  assert.equal(f.session.snapshot().phase, "authenticated");
  assert.equal(f.session.snapshot().identity.expires_at, null);
  assert.equal(JSON.stringify(f.session.snapshot()).includes("secret"), false);
  assert.throws(() => f.session.snapshot().identity.permissions.push("queue:delete"));
  f.session.clear();
  assert.equal(f.token(), "");
  assert.equal(f.session.snapshot().identity, null);
  assert.equal(updates, 3);
  unsubscribe();
});

test("distinguishes denial, unavailable and disabled without exposing raw error", async () => {
  for (const [status, code, expected] of [[401, "", "credentials-rejected"], [403, "", "role-denied"], [404, "session_api_disabled", "auth-disabled"], [404, "", "unavailable"], [503, "", "unavailable"]]) {
    const f = fixture(async () => { throw Object.assign(new Error("secret-server-detail"), {status, code}); });
    await f.session.signIn("secret");
    assert.equal(f.session.snapshot().failure.kind, expected);
    assert.equal(f.token(), "");
    assert.equal(JSON.stringify(f.session.snapshot()).includes("secret"), false);
    assert.equal(f.calls.length, 1);
  }
});

test("malformed or expired session cannot authenticate", async () => {
  for (const body of [null, {...identity, actor: ""}, {...identity, role: "viewer"}, {...identity, permissions: [null]}, {...identity, expires_at: "invalid"}, {...identity, expires_at: "2000-01-01T00:00:00Z"}, {...identity, resource_read_policy: "unknown"}]) {
    const f = fixture(async () => ({body}));
    await f.session.signIn("secret");
    assert.equal(f.session.snapshot().phase, "signed-out");
    assert.equal(f.token(), "");
  }
});

test("clear suppresses late response even if transport ignores abort", async () => {
  let resolve;
  const f = fixture(() => new Promise(done => { resolve = done; }));
  const pending = f.session.signIn("secret");
  const signal = f.calls[0][1].signal;
  f.session.clear();
  assert.equal(signal.aborted, true);
  resolve({body: identity});
  await pending;
  assert.equal(f.session.snapshot().phase, "signed-out");
  assert.equal(f.token(), "");
});

test("old failure cannot erase a newer authenticated credential", async () => {
  let reject;
  let count = 0;
  const f = fixture(() => ++count === 1 ? new Promise((_, fail) => { reject = fail; }) : Promise.resolve({body: {...identity, role: "auditor"}}));
  const old = f.session.signIn("old-token");
  await f.session.signIn("new-token");
  reject(new Error("late failure"));
  await old;
  assert.equal(f.token(), "new-token");
  assert.equal(f.session.snapshot().identity.role, "auditor");
});

test("empty input clears previous session without requesting", async () => {
  const f = fixture();
  await f.session.signIn("secret");
  await f.session.signIn("  ");
  assert.equal(f.token(), "");
  assert.equal(f.calls.length, 1);
  assert.equal(f.session.snapshot().failure.kind, "missing-token");
});

test("verified expiry retains identity, removes token and cancels old timers",async()=>{
  let now=1000, token="", expired=0;const timers=new Map();let next=0;
  const api={setToken:value=>{token=value;},clearToken:()=>{token="";},expireToken:()=>{token="";expired++;},request:async()=>({body:{...identity,expires_at:new Date(2000).toISOString()}})};
  const session=createSession(api,{now:()=>now,schedule:(fn,delay)=>{assert.equal(delay,1000);timers.set(++next,fn);return next;},unschedule:id=>timers.delete(id)});
  await session.signIn("secret");const old=[...timers.values()][0];now=2000;old();
  assert.equal(session.snapshot().phase,"expired");assert.equal(session.snapshot().identity.actor,identity.actor);assert.equal(token,"");assert.equal(expired,1);
  session.clear();old();assert.equal(session.snapshot().phase,"signed-out");assert.equal(expired,1);
});

test("dispatch guard catches expiry even when background timer has not fired",async()=>{
  let now=1000, calls=0;
  const api=createAPI({origin:"http://localhost",now:()=>now,fetch:async()=>{calls++;return new Response(JSON.stringify({...identity,expires_at:new Date(2000).toISOString()}),{status:200});}});
  const session=createSession(api,{now:()=>now,schedule:()=>1,unschedule:()=>{}});
  await session.signIn("secret");now=2000;
  await assert.rejects(api.request("/api/v1/queues/q",{method:"PUT",body:{}}),error=>error.kind==="expired"&&error.uncertain===false);
  assert.equal(calls,1);assert.equal(session.snapshot().phase,"expired");
  await assert.rejects(api.request("/api/v1/queues"));assert.equal(calls,1);
  session.clear();now=1000;await session.signIn("new-secret");assert.equal(session.snapshot().phase,"authenticated");
});
