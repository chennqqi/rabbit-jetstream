import test from "node:test";
import assert from "node:assert/strict";
import {mkdtemp,mkdir,writeFile,rm,symlink} from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import {snapshotCandidate} from "../../tests/admin-ui/candidate-inputs.mjs";

async function fixture(t) {
  const root=await mkdtemp(path.join(os.tmpdir(),"rjs-candidate-inputs-"));
  t.after(()=>rm(root,{recursive:true,force:true}));
  await writeFile(path.join(root,"index.html"),"entry only");
  await mkdir(path.join(root,"assets"));
  await writeFile(path.join(root,"assets","entry.js"),"entry");
  await writeFile(path.join(root,"assets","lazy.js"),"lazy");
  return root;
}

test("candidate snapshots include unreferenced chunks and detect content, additions and removals",async t=>{
  const root=await fixture(t),before=await snapshotCandidate(root);
  assert.deepEqual(before.map(file=>file.path),["assets/entry.js","assets/lazy.js","index.html"]);
  assert.deepEqual(await snapshotCandidate(root),before);
  await writeFile(path.join(root,"assets","lazy.js"),"LAZY");
  const changed=await snapshotCandidate(root);
  assert.equal(changed[1].bytes,before[1].bytes);assert.notEqual(changed[1].sha256,before[1].sha256);
  await mkdir(path.join(root,"assets","nested"));
  await writeFile(path.join(root,"assets","nested","extra.css"),"body{}");
  assert.equal((await snapshotCandidate(root)).length,before.length+1);
  await rm(path.join(root,"assets","entry.js"));
  assert.ok(!(await snapshotCandidate(root)).some(file=>file.path==="assets/entry.js"));
  await rm(path.join(root,"index.html"));
  await assert.rejects(snapshotCandidate(root),/index.html is required/);
});

test("candidate snapshots reject directory links without following them",async t=>{
  const root=await fixture(t);
  await symlink(path.join(root,"assets"),path.join(root,"linked-assets"),process.platform==="win32"?"junction":"dir");
  await assert.rejects(snapshotCandidate(root),/symbolic links/);
});
