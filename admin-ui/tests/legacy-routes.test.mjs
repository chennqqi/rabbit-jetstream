import {test} from "node:test";
import assert from "node:assert/strict";
import {legacyRouteURL,readRoute,createRouter} from "../src/routes.mjs";

test("only documented legacy entry links map to canonical pages",()=>{
  for(const base of ["/admin/","/admin/index.html"])for(const name of ["overview","queues","nodes"]){
    const url=new URL(`http://localhost${base}#${name}`);
    assert.equal(legacyRouteURL(url),`/admin/${name}`);assert.equal(readRoute(url).kind,name);
  }
  for(const path of ["/admin/?token=x#nodes","/admin/queues/by-name/orders#nodes","/admin/#delete","/admin/#nodes?token=x","/admin/#__proto__"]){
    assert.equal(legacyRouteURL(new URL(path,"http://localhost")),null);
  }
});
test("startup and legacy history changes replace entries without adding history",()=>{
  let location=new URL("http://localhost/admin/#nodes"),replaces=0;
  const events=new Map(),historyState={retained:true};
  const browser={get location(){return location;},history:{state:historyState,replaceState(state,title,path){assert.equal(state,historyState);replaces++;location=new URL(path,location);}},addEventListener:(name,fn)=>events.set(name,fn),removeEventListener:name=>events.delete(name)};
  const router=createRouter(browser);assert.equal(router.snapshot().kind,"nodes");assert.equal(location.hash,"");assert.equal(replaces,1);
  const unsubscribe=router.subscribe(()=>{});
  location=new URL("http://localhost/admin/#overview");events.get("hashchange")();assert.equal(router.snapshot().kind,"overview");assert.equal(replaces,2);
  location=new URL("http://localhost/admin/index.html#queues");events.get("popstate")();assert.equal(router.snapshot().kind,"queues");assert.equal(replaces,3);
  unsubscribe();assert.equal(events.size,0);
});
