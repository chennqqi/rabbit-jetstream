import test from "node:test";
import assert from "node:assert/strict";
import {JSDOM} from "jsdom";
import {createServer} from "vite";
import react from "@vitejs/plugin-react";
import {fileURLToPath} from "node:url";

const globals=["window","document","navigator","location","history","HTMLElement","Node","Event"];

test("Global Consumers renders a maximum 200-row page from a 100k generation",async t=>{
  if (process.platform === "win32") return t.skip("vite ssrLoadModule drops the drive letter on Windows, resolving module imports to a phantom drive-root copy of this repository (dual React instance); the same assertions are covered by the Playwright dual-browser e2e suite and the S-4 matrix");

  const dom=new JSDOM('<!doctype html><html><body><div id="root"></div></body></html>',{url:"https://console.example/admin/consumers"});
  const previous=new Map(globals.map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
  for(const key of globals)Object.defineProperty(globalThis,key,{configurable:true,writable:true,value:dom.window[key]});
  const {render,waitFor,cleanup}=await import("@testing-library/react");
  const React=(await import("react")).default;
  const server=await createServer({configFile:false,root:fileURLToPath(new URL("..",import.meta.url)),mode:"test",plugins:[react()],server:{middlewareMode:true},appType:"custom",logLevel:"silent"});
  t.after(async()=>{cleanup();await server.close();dom.window.close();for(const [key,descriptor] of previous)descriptor?Object.defineProperty(globalThis,key,descriptor):delete globalThis[key];});
  const {GlobalConsumers}=await server.ssrLoadModule("/src/GlobalConsumers.jsx");
  const items=Array.from({length:200},(_,index)=>({stream:`stream-${String(index).padStart(5,"0")}`,name:`consumer-${String(index).padStart(5,"0")}`,queue:`queue-${String(index).padStart(5,"0")}`,durable:`consumer-${String(index).padStart(5,"0")}`,mode:"pull",status:"present",ownership:"matching",pending:BigInt(index),ack_pending:0n}));
  const api={request:async path=>{assert.match(path,/limit=200/);return {body:{state:"ready",generation_id:"docker-simulated-100k",started_at:"2026-09-12T00:00:00Z",completed_at:"2026-09-12T00:00:01Z",items,total:100000,offset:0,limit:200}};}};
  const route={query:{q:"",queue:"",stream:"",mode:"",state:"",order:"asc",offset:0,limit:200,generation:""}};
  const started=performance.now();
  const view=render(React.createElement(GlobalConsumers,{api,language:"en",route,router:{navigate:()=>true},canRefresh:true}),{container:document.getElementById("root")});
  await waitFor(()=>assert.equal(view.getAllByRole("row").length,201));
  const usableMillis=performance.now()-started;
  assert.match(view.getByRole("status").textContent,/100000/);
  const region=view.getByRole("region",{name:"Consumer list"});
  assert.equal(region.tabIndex,0);
  assert.equal(view.getByRole("table").querySelector("caption").textContent,"Global Consumer observation list");
  assert.ok([...view.getAllByRole("columnheader")].every(header=>header.getAttribute("scope")==="col"));
  assert.ok([...view.getAllByRole("rowheader")].every(header=>header.getAttribute("scope")==="row"));
  const pagination=view.getByRole("navigation",{name:"Consumer pagination"});
  assert.equal(pagination.querySelector('[aria-live="polite"]').textContent,"1–200 / 100000");
  assert.equal(view.getByRole("button",{name:"Previous"}).disabled,true);
  assert.equal(view.getByRole("button",{name:"Next"}).disabled,false);
  assert.ok(usableMillis<1000,`maximum page took ${usableMillis.toFixed(2)} ms to become usable`);
  t.diagnostic(`200-row first usable: ${usableMillis.toFixed(2)} ms`);
});
