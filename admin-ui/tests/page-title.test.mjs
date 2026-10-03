import test from "node:test";
import assert from "node:assert/strict";
import {consolePageTitle} from "../src/page-title.mjs";

test("page title follows session, route identity and language without unrelated data",()=>{
  assert.equal(consolePageTitle({phase:"anonymous"}),"Management console — Rabbit JetStream");
  assert.equal(consolePageTitle({phase:"expired",language:"zh"}),"会话已过期 — Rabbit JetStream");
  assert.equal(consolePageTitle({authenticated:true,route:{kind:"queues"}}),"Queue list — Rabbit JetStream");
  assert.equal(consolePageTitle({authenticated:true,route:{kind:"queue",name:"orders"},language:"zh"}),"Queue: orders — Rabbit JetStream");
  assert.equal(consolePageTitle({authenticated:true,route:{kind:"consumer",stream:"RJSQ_orders",name:"RJSQC_orders"}}),"Consumer: RJSQ_orders / RJSQC_orders — Rabbit JetStream");
  assert.equal(consolePageTitle({authenticated:true,route:{kind:"node-connection",id:"node-a",cid:"42"},language:"zh"}),"连接: node-a / 42 — Rabbit JetStream");
  assert.equal(consolePageTitle({authenticated:true,route:{kind:"future",name:"secret"}}),"Page unavailable — Rabbit JetStream");
});
