import {test} from "node:test";
import assert from "node:assert/strict";
import {readRoute, readQueueQuery, queueListURL, queueDetailURL, createRouter,globalConsumersURL,readGlobalConsumerQuery} from "../src/routes.mjs";
import {queueTabURL} from "../src/routes.mjs";

test("Settings route rejects query payloads and keeps resource names separate",()=>{
  assert.deepEqual(readRoute(new URL("http://localhost/admin/settings")),{kind:"settings"});
  assert.equal(readRoute(new URL("http://localhost/admin/settings?token=x")).kind,"invalid");
  assert.equal(readRoute(new URL("http://localhost/admin/queues/by-name/settings")).kind,"queue");
});

test("Diagnostics route is exact and never accepts state in the URL",()=>{
  assert.deepEqual(readRoute(new URL("http://localhost/admin/diagnostics")),{kind:"diagnostics"});
  assert.equal(readRoute(new URL("http://localhost/admin/diagnostics?job=secret")).kind,"invalid");
});
test("Operational alerts route is exact and carries no browser query",()=>{
  assert.deepEqual(readRoute(new URL("https://example.test/admin/alerts")),{kind:"alerts"});
  assert.equal(readRoute(new URL("https://example.test/admin/alerts?rule=x")).kind,"invalid");
});

test("Queue tabs retain Consumer queries and exact audit cursors",()=>{
  const query={q:"x",mode:"pull",order:"desc",offset:100,limit:50};
  const route=readRoute(new URL(queueTabURL("q","events",query,"18446744073709551615"),"http://localhost"));
  assert.deepEqual(route,{kind:"queue",name:"q",tab:"events",consumerQuery:query,eventBefore:"18446744073709551615"});
  for(const search of ["?tab=wrong","?tab=summary&tab=events","?ebefore=-1","?ebefore=18446744073709551616"])assert.equal(readRoute(new URL(queueDetailURL("q")+search,"http://localhost")).kind,"invalid");
});

test("Queue named new remains distinct from creation; encoded segments validated", () => {
  assert.equal(readRoute({pathname:"/admin/queues/new"}).kind,"create-queue");
  assert.equal(readRoute({pathname:"/admin/queues/bulk-change"}).kind,"bulk-change");
  assert.equal(readRoute({pathname:"/admin/queues/bulk-change",search:"?token=x"}).kind,"invalid");
  assert.deepEqual(readRoute({pathname:queueDetailURL("new")}),{kind:"queue",name:"new"});
  assert.deepEqual(readRoute({pathname:queueDetailURL("orders.a-b_1")+"/edit"}),{kind:"edit-queue",name:"orders.a-b_1"});
  for(const segment of ["%", "%2F", "%5C", "%00", "..", "."]) assert.equal(readRoute({pathname:"/admin/queues/by-name/"+segment}).kind,"invalid");
});

test("list URL round trips explicit query without token or draft state", () => {
  const query={q:"orders & billing",order:"desc",offset:150,limit:25};
  const url=new URL(queueListURL(query),"http://localhost");
  assert.deepEqual(readRoute(url),{kind:"queues",query});
  for(const search of ["?offset=-1","?limit=201","?limit=1.5","?offset=9007199254740992","?q=a&q=b","?token=secret","?order=invalid"]) assert.throws(()=>readQueueQuery(search));
});

test("global Consumer URL round trips filters and rejects mixed or repeated state",()=>{
  const query={q:"same",queue:"orders",stream:"RJSQ_orders",mode:"pull",state:"missing",order:"desc",offset:50,limit:25,generation:"generation-1"};
  assert.deepEqual(readRoute(new URL(globalConsumersURL(query),"http://localhost")),{kind:"consumers",query});
  for(const search of ["?state=bad","?mode=push&mode=pull","?generation=x&generation=y","?token=secret"])assert.throws(()=>readGlobalConsumerQuery(search));
});

test("router rejects external targets and reacts to history events", () => {
  let location=new URL("http://localhost/admin/queues");
  let pushes=0, listener;
  const browser={get location(){return location;},history:{pushState(_state,_title,path){pushes++;location=new URL(path,location);}},
    addEventListener(type,fn){assert.ok(["popstate","hashchange"].includes(type));listener=fn;},removeEventListener(){listener=null;}};
  const router=createRouter(browser); let updates=0;
  const unsubscribe=router.subscribe(()=>updates++);
  assert.equal(router.navigate("/admin/queues"),false);
  router.navigate(queueDetailURL("new"));
  assert.equal(router.snapshot().name,"new");
  assert.equal(pushes,1);
  location=new URL("http://localhost/admin/queues?q=x&offset=50");listener();
  assert.equal(router.snapshot().query.offset,50);
  assert.equal(updates,2);
  for(const path of ["https://outside.test/admin/queues","/api/v1/queues","/admin/queues?token=x"])assert.throws(()=>router.navigate(path));
  unsubscribe();assert.equal(listener,null);
});
