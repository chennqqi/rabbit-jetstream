import {test} from "node:test";
import assert from "node:assert/strict";
import {createAuditList} from "../src/audit-list.mjs";
import {readAuditQuery,auditURL} from "../src/audit-query.mjs";
import {readRoute} from "../src/routes.mjs";

const query=readAuditQuery("?actor=owner");
const filter={...query};delete filter.before;
const window={items:[],streamPresent:true,firstSequence:1,lastSequence:9007199254742000n,scanned:256,missing:0,nextBefore:9007199254741745n,filter};
test("Audit URL preserves exact cursor and rejects hidden or duplicate parameters",()=>{
  const q=readAuditQuery("?before=18446744073709551615&actor=owner");
  assert.equal(q.before,"18446744073709551615");assert.deepEqual(readRoute(new URL(auditURL(q),"http://localhost")),{kind:"audit",query:q});
  for(const search of ["?token=x","?actor=a&actor=b","?before=-1","?before=18446744073709551616","?phase=unknown","?actor=%00"])assert.throws(()=>readAuditQuery(search));
});
test("Empty matching windows retain the next cursor and send exact server filters",async()=>{
  let path;const model=createAuditList({request:async value=>{path=value;return {body:window};}});
  await model.load(query);assert.equal(model.snapshot().phase,"ready");assert.equal(model.snapshot().page.nextBefore,9007199254741745n);
  assert.equal(new URL(path,"http://localhost").searchParams.get("actor"),"owner");
});
test("Mismatch and failure clear evidence, late reads cannot restore it",async()=>{
  const api={request:async()=>({body:window})};const model=createAuditList(api);await model.load(query);
  api.request=async()=>({body:{...window,filter:{...filter,actor:"other"}}});await model.load(query);assert.equal(model.snapshot().page,null);
  api.request=async()=>{throw Object.assign(new Error(),{status:403});};await model.load(query);assert.equal(model.snapshot().failure,"denied");
  let resolve;api.request=()=>new Promise(done=>{resolve=done;});const pending=model.load(query);model.clear();resolve({body:window});await pending;assert.equal(model.snapshot().phase,"idle");
});
