import test from "node:test";
import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import {buildMetadata,sdkMetadata,createCompatibility} from "../src/compatibility.mjs";
import {readRoute} from "../src/routes.mjs";
const sdk=JSON.parse(await readFile(new URL("../../api/native-sdk-contract.json",import.meta.url),"utf8"));
const build={schemaVersion:"rjs.build-info.v1",version:"dev",goVersion:"go1.25",os:"linux",arch:"amd64",uiAssets:{algorithm:"sha256-framed-files-v1",digest:"a".repeat(64),fileCount:4}};
test("compatibility models preserve explicit unknown VCS and unreleased SDK stage",()=>{
  assert.equal(buildMetadata(build).modified,undefined);
  assert.equal(sdkMetadata(sdk).availability,"native-sdk-implemented-unreleased");
  assert.equal(buildMetadata({...build,private:"secret"}).private,undefined);
  assert.equal(buildMetadata(build).uiAssets.digest,"a".repeat(64));
  assert.equal(buildMetadata({...build,revision:"b".repeat(40),revisionSource:"release-build",modified:false}).revisionSource,"release-build");
  assert.equal(buildMetadata({...build,uiAssets:undefined}).uiAssets,undefined);
  for(const patch of [{schemaVersion:"future"},{modified:"false"},{arch:null},{revision:42},{revisionSource:"unknown"},{revisionSource:"release-build"},{uiAssets:{algorithm:"sha256",digest:"a".repeat(64),fileCount:4}},{uiAssets:{algorithm:"sha256-framed-files-v1",digest:"A".repeat(64),fileCount:4}},{uiAssets:{algorithm:"sha256-framed-files-v1",digest:"a".repeat(64),fileCount:0}}])assert.throws(()=>buildMetadata({...build,...patch}));
  assert.throws(()=>sdkMetadata({...sdk,schema:"future"}));
  assert.equal(readRoute(new URL("http://localhost/admin/compatibility")).kind,"compatibility");
  assert.equal(readRoute(new URL("http://localhost/admin/compatibility?upgrade=1")).kind,"invalid");
});
test("compatibility sources fail independently and clear prior invalid values",async()=>{
  let fail=false;const calls=[];const model=createCompatibility({request:async(path,options)=>{calls.push({path,options});if(path.endsWith("/build")){if(fail)throw {status:503};return {body:build};}return {body:sdk};}});
  await model.load();assert.equal(model.snapshot().build.phase,"ready");fail=true;await model.load();
  assert.equal(model.snapshot().build.phase,"error");assert.equal(model.snapshot().build.value,null);assert.equal(model.snapshot().sdk.phase,"ready");
  assert.ok(calls.every(call=>!call.options.method));
});
test("cleared compatibility view cannot be restored by late reads",async()=>{
  const pending=[];const model=createCompatibility({request:()=>new Promise(resolve=>pending.push(resolve))});const done=model.load();model.clear();pending[0]({body:build});pending[1]({body:sdk});await done;
  assert.equal(model.snapshot().build.phase,"idle");assert.equal(model.snapshot().sdk.phase,"idle");
});
