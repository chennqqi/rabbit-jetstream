import assert from "node:assert/strict";
import {test} from "node:test";
import {mkdtemp, mkdir, readFile, rm, symlink, writeFile} from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import {syncDistribution} from "../scripts/dist-sync.mjs";

async function fixture() {
  const root=await mkdtemp(path.join(os.tmpdir(),"rjs-ui-dist-")),candidate=path.join(root,"candidate"),dist=path.join(root,"dist");
  await mkdir(path.join(candidate,"assets"),{recursive:true});
  await writeFile(path.join(candidate,"index.html"),'<script type="module" src="/admin/assets/app-a1.js"></script><link rel="stylesheet" href="/admin/assets/app-b2.css">');
  await writeFile(path.join(candidate,"assets/app-a1.js"),"export default 1;");
  await writeFile(path.join(candidate,"assets/app-b2.css"),"body{color:#123}");
  return {root,candidate,dist};
}

test("promotion exactly replaces a stale distribution and check detects drift",async()=>{
  const value=await fixture();
  try {
    await mkdir(value.dist);await writeFile(path.join(value.dist,"legacy.js"),"legacy");
    const files=await syncDistribution(value.candidate,value.dist);
    assert.deepEqual(files.map(file=>file.path),["assets/app-a1.js","assets/app-b2.css","index.html"]);
    await syncDistribution(value.candidate,value.dist,{check:true});
    await writeFile(path.join(value.dist,"assets/app-a1.js"),"changed");
    await assert.rejects(syncDistribution(value.candidate,value.dist,{check:true}),/differs/);
  } finally {await rm(value.root,{recursive:true,force:true});}
});

test("promotion rejects missing entrypoint assets",async()=>{
  const missing=await fixture();
  try {
    await rm(path.join(missing.candidate,"assets/app-a1.js"));
    await assert.rejects(syncDistribution(missing.candidate,missing.dist),/missing asset/);
  } finally {await rm(missing.root,{recursive:true,force:true});}
});

test("promotion rejects symbolic links",async t=>{
  const linked=await fixture();
  try {
    try{await symlink(path.join(linked.candidate,"assets/app-a1.js"),path.join(linked.candidate,"assets/link.js"));}
    catch(error){if(error.code==="EPERM"){t.skip("symbolic-link creation is not permitted on this Windows host");return;}throw error;}
    await assert.rejects(syncDistribution(linked.candidate,linked.dist),/symbolic links/);
  } finally {await rm(linked.root,{recursive:true,force:true});}
});
