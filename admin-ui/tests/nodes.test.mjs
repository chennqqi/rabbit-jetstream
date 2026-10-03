import {test} from "node:test";
import assert from "node:assert/strict";
import {createNodes,nodeSourceAvailable,nodeJSMetric} from "../src/nodes.mjs";
import {readRoute,nodeDetailURL} from "../src/routes.mjs";

test("Node routes use exact ID and reject unsafe identities",()=>{
  assert.deepEqual(readRoute(new URL(nodeDetailURL("N123"),"http://localhost")),{kind:"node",id:"N123"});
  assert.equal(readRoute(new URL("/admin/nodes?token=x","http://localhost")).kind,"invalid");
  assert.throws(()=>nodeDetailURL("../bad"));
});

test("JetStream metrics require both source availability and an exact reported count",()=>{
  const node={sources:{jsz:{available:true}},jetstream:{messages:0,memory_bytes:18446744073709551615n}};
  assert.equal(nodeJSMetric(node,"messages"),"0");
  assert.equal(nodeJSMetric(node,"memory_bytes"),"18446744073709551615");
  assert.equal(nodeJSMetric(node,"meta_pending"),null);
  for(const value of [null,undefined,-1,1.5,Number.MAX_SAFE_INTEGER+1,"0",false]){
    node.jetstream.messages=value;assert.equal(nodeJSMetric(node,"messages"),null);
  }
  node.jetstream.messages=0;node.sources.jsz.available=false;
  assert.equal(nodeJSMetric(node,"messages"),null);
  delete node.sources;assert.equal(nodeJSMetric(node,"messages"),null);
});
test("Node collection retains unavailable endpoints and separates source failures",async()=>{
  const nodes=[{endpoint:"http://a",id:"N123",status:"degraded",sources:{varz:{available:true,read_at:"2026-09-10T00:00:00Z"},jsz:{available:false,read_at:"2026-09-10T00:00:01Z"}},jetstream:{messages:0}}, {endpoint:"http://b",status:"unavailable"}];
  const api={request:async path=>{assert.equal(path,"/api/v1/nodes");return {body:{nodes,total:2}};}};
  const model=createNodes(api);await model.load();assert.equal(model.snapshot().snapshot.nodes.length,2);
  assert.equal(nodeSourceAvailable(nodes[0],"jsz"),false);assert.equal(nodeSourceAvailable(nodes[0],"varz"),true);assert.equal(nodeSourceAvailable(nodes[1],"varz"),false);
  api.request=async()=>{throw Object.assign(new Error(),{status:503});};await model.load();assert.equal(model.snapshot().snapshot,null);assert.equal(model.snapshot().failure,"unavailable");
});
test("Node validation rejects unsafe links, malformed sources and partial envelopes",async()=>{
  for(const body of [{nodes:[],total:1},{nodes:[{endpoint:"a",id:"a/b",status:"available"}],total:1},{nodes:[{endpoint:"a",status:"available",sources:{jsz:{available:"yes"}}}],total:1}]){
    const model=createNodes({request:async()=>({body})});await model.load();assert.equal(model.snapshot().failure,"invalid");
  }
});
test("Late node read cannot restore cleared state",async()=>{
  let finish;const model=createNodes({request:()=>new Promise(resolve=>{finish=resolve;})});const pending=model.load();model.clear();finish({body:{nodes:[],total:0}});await pending;assert.equal(model.snapshot().phase,"idle");
});
