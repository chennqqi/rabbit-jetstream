import {readFile} from "node:fs/promises";
import {parseJSON,stringifyJSON} from "../../admin-ui/src/api.mjs";

export async function queueExportChecks({page,api,origin,auditor,expect,assert,evidence,path,report}){
  const name="live_export",url=`/api/v1/queues/${name}`;
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name,labels:{owner:"fixture-label"}},spec:{replicas:1,subjects:["live_export.events"],maxPriority:0,retention:{maxMessages:9223372036854775807n}}};
  const created=await api(url,{method:"PUT",headers:{"If-None-Match":"*"},body:stringifyJSON(document)});
  assert.equal(created.status,200,await created.text());
  await page.addInitScript(()=>{
    window.__exportRevoked=[];
    const revoke=URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL=value=>{window.__exportRevoked.push(value);revoke(value);};
  });
  await page.goto(`${origin}/admin/queues/by-name/${name}?tab=configuration`);
  await page.locator(".recovery-login summary").click();
  await page.getByLabel("Recovery bearer token",{exact:true}).fill(auditor);
  await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  const panel=page.getByRole("region",{name:"Queue declaration export",exact:true});
  const prepare=()=>panel.getByRole("button",{name:"Prepare declaration download",exact:true}).click();
  const link=panel.getByRole("link",{name:"Download Queue JSON",exact:true});
  await expect(panel.getByRole("checkbox")).not.toBeChecked();
  await expect(link).toHaveCount(0);
  await prepare();await expect(link).toBeVisible();
  const firstURL=await link.getAttribute("href");assert.ok(firstURL.startsWith("blob:"));
  async function download(){
    const pending=page.waitForEvent("download");await link.click();const file=await pending;
    assert.equal(file.suggestedFilename(),`${name}.queue.json`);
    const raw=await readFile(await file.path(),"utf8");return {raw,body:parseJSON(raw)};
  }
  const redacted=await download();
  assert.equal(redacted.body.metadata.labels,undefined);assert.equal(redacted.raw.includes("fixture-label"),false);
  assert.equal(redacted.body.spec.retention.maxMessages,9223372036854775807n);assert.equal(redacted.body.spec.maxPriority,0);
  assert.deepEqual(Object.keys(redacted.body).sort(),["apiVersion","kind","metadata","spec"]);
  await expect(panel).toContainText("Omitted labels: 1");
  await panel.getByRole("checkbox").check();await expect(link).toHaveCount(0);
  await expect.poll(()=>page.evaluate(value=>window.__exportRevoked.includes(value),firstURL)).toBe(true);
  await prepare();await expect(link).toBeVisible();const full=await download();
  assert.equal(full.body.metadata.labels.owner,"fixture-label");await expect(panel).toContainText("Omitted labels: 0");
  const changeLink=panel.getByRole("link",{name:"Download editable change package",exact:true});await expect(changeLink).toBeVisible();const changeURL=await changeLink.getAttribute("href"),pendingChange=page.waitForEvent("download");await changeLink.click();const changeFile=await pendingChange;assert.equal(changeFile.suggestedFilename(),`${name}.queue-change.json`);const changePackage=parseJSON(await readFile(await changeFile.path(),"utf8"));assert.equal(changePackage.schema,"rjs.queue-change.v1");assert.equal(changePackage.document.metadata.labels.owner,"fixture-label");assert.equal(changePackage.document.spec.retention.maxMessages,9223372036854775807n);assert.match(changePackage.etag,/^"[1-9][0-9]*"$/);
  const secondURL=await link.getAttribute("href");
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  const chinese=page.getByRole("region",{name:"Queue 声明导出",exact:true});
  await expect(chinese.getByRole("link",{name:"下载 Queue JSON",exact:true})).toBeVisible();
  await page.setViewportSize({width:375,height:900});
  await chinese.screenshot({path:path.join(evidence,"queue-export-mobile-zh.png")});
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"export mobile overflow");
  await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
  const tabs=page.getByRole("navigation",{name:"Queue detail tabs",exact:true});
  await tabs.getByRole("link",{name:"Routing",exact:true}).click();
  await expect.poll(()=>page.evaluate(value=>window.__exportRevoked.includes(value),secondURL)).toBe(true);
  await expect.poll(()=>page.evaluate(value=>window.__exportRevoked.includes(value),changeURL)).toBe(true);
  await tabs.getByRole("link",{name:"Configuration",exact:true}).click();await expect(link).toHaveCount(0);

  const before=await api(url);document.metadata.labels.owner="changed-fixture";
  const changed=await api(url,{method:"PUT",headers:{"If-Match":before.headers.get("ETag")},body:stringifyJSON(document)});
  assert.equal(changed.status,200,await changed.text());
  await prepare();await expect(panel.getByRole("alert")).toContainText("Declaration changed");await expect(link).toHaveCount(0);
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await prepare();await expect(link).toBeVisible();
  const pattern=`**/api/v1/queues/${name}/export?*`;
  await page.route(pattern,route=>route.fulfill({status:200,contentType:"application/json",body:"{"}),{times:1});
  await prepare();await expect(panel.getByRole("alert")).toContainText("Export evidence is invalid");await expect(link).toHaveCount(0);
  await prepare();await expect(link).toBeVisible();
  let entered=false,release;const gate=new Promise(resolve=>{release=resolve;});
  const hold=async route=>{entered=true;await gate;await route.continue().catch(()=>{});};
  await page.route(pattern,hold,{times:1});
  try{await prepare();await expect.poll(()=>entered).toBe(true);await panel.getByRole("button",{name:"Cancel export",exact:true}).click();}finally{release();}
  await expect(link).toHaveCount(0);await page.unroute(pattern,hold);await prepare();await expect(link).toBeVisible();
  report.checks.push("queue-export-real-file-exact-int64-priority-label-policy-blob-revocation-bilingual-mobile");
  report.checks.push("queue-export-real-revision-change-invalid-json-cancel-no-old-download-recovery");
}
