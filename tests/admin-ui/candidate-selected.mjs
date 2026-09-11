// Isolated screenshot fixture: no NATS, credentials, proxy, mutations or remote hosts.
import assert from "node:assert/strict";
import {createServer} from "node:http";
import {once} from "node:events";
import {createRequire} from "node:module";
import {fileURLToPath} from "node:url";
import path from "node:path";
import {mkdir,mkdtemp,readFile,writeFile} from "node:fs/promises";
import {createHash} from "node:crypto";
import {selectedFixture,selectedFixtureResponse} from "./selected-fixture.mjs";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"../..");
const require=createRequire(path.join(root,"admin-ui/package.json"));
const {chromium,expect}=require("@playwright/test");
const assets=path.join(root,"admin-ui/build-candidate"),reference=path.join(root,"docs/design/webui/queue-detail-selected.png");
const index=await readFile(path.join(assets,"index.html"));
assert.equal(createHash("sha256").update(await readFile(reference)).digest("hex"),selectedFixture.referenceSHA256);
await mkdir(path.join(root,"artifacts"),{recursive:true});
const evidence=await mkdtemp(path.join(root,"artifacts/webui-selected-"));
const report={synthetic:true,passed:false,fixture:selectedFixture,requests:[],errors:[],viewport:{width:1487,height:1058},deviceScaleFactor:1,visualQAPassed:false};
report.buildIndexSHA256=createHash("sha256").update(index).digest("hex");
const {snapshotCandidate}=await import("./candidate-inputs.mjs");
const candidateSnapshot=await snapshotCandidate(assets);
assert.equal(candidateSnapshot.find(file=>file.path==="index.html").sha256,report.buildIndexSHA256,"Candidate index changed before capture");
report.buildAssets=[];
for(const match of index.toString().matchAll(/\/admin\/(assets\/[a-zA-Z0-9_-]+\.(?:js|css))/g)){
  const bytes=await readFile(path.join(assets,match[1]));
  report.buildAssets.push({path:match[1],sha256:createHash("sha256").update(bytes).digest("hex")});
}
assert.ok(report.buildAssets.some(asset=>asset.path.endsWith(".js"))&&report.buildAssets.some(asset=>asset.path.endsWith(".css")),"Candidate build must expose fingerprinted JS and CSS");
report.buildAssets=candidateSnapshot.filter(file=>file.path!=="index.html");
report.annotation="Only the existing candidate notice text is replaced with an explicit synthetic-data label for capture.";
let browser;
const server=createServer(async(req,res)=>{
  try {
    const url=new URL(req.url,"http://fixture.invalid");
    if(url.pathname.startsWith("/api/")){
      const result=selectedFixtureResponse(req.method,req.url);
      report.requests.push({method:req.method,path:req.url,status:result.status});
      res.writeHead(result.status,{"content-type":"application/json","cache-control":"no-store",...result.headers});res.end(JSON.stringify(result.body));return;
    }
    if(req.method!=="GET"||!url.pathname.startsWith("/admin/")){res.writeHead(404);res.end();return;}
    const asset=url.pathname.startsWith("/admin/assets/")?url.pathname.slice(7):"index.html";
    if(!/^(index\.html|assets\/[a-zA-Z0-9_-]+\.(js|css))$/.test(asset)){res.writeHead(404);res.end();return;}
    res.writeHead(200,{"content-type":asset.endsWith(".js")?"text/javascript":asset.endsWith(".css")?"text/css":"text/html","cache-control":"no-store","content-security-policy":"default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"});
    res.end(asset==="index.html"?index:await readFile(path.join(assets,asset)));
  }catch(error){report.errors.push(error.message);res.end();}
});
try {
  server.listen(0,"127.0.0.1");await once(server,"listening");
  const origin=`http://127.0.0.1:${server.address().port}`;
  browser=await chromium.launch({headless:true});
  const page=await browser.newPage({locale:"zh-CN",timezoneId:"Asia/Shanghai",viewport:report.viewport,deviceScaleFactor:1});
  page.setDefaultTimeout(15000);page.on("pageerror",error=>report.errors.push(error.message));
  // This clock controls presentation timestamps, not real credential-expiry qualification.
  await page.clock.setFixedTime(new Date(selectedFixture.instant));
  await page.route("**/*",route=>{if(new URL(route.request().url()).origin!==origin){report.errors.push("Unexpected external request");return route.abort();}return route.continue();});
  await page.goto(`${origin}/admin/queues/by-name/${selectedFixture.queue}`);
  await page.getByLabel("Bearer Token",{exact:true}).fill("visual-fixture-only-not-a-credential");
  await page.getByRole("button",{name:"验证身份",exact:true}).click();
  const metrics=page.getByLabel("范围内摘要指标",{exact:true});
  await expect(page.getByRole("region",{name:"Consumer 排查入口",exact:true})).toBeVisible();
  for(const [label,value] of [["存储消息数（Stream）","12480"],["待投递（主 Consumer）","8420"],["待确认（主 Consumer）","240"]])await expect(metrics.locator("dt").filter({hasText:label}).locator("+ dd")).toHaveText(value);
  await expect(page.locator(".declaration-meta")).toContainText('"12"');
  const replicas=page.getByRole("region",{name:"副本观测",exact:true});
  await expect(page.locator(".declaration-meta time")).toHaveText("2026-09-09 16:20:00.000 (Asia/Shanghai, GMT+08:00)");
  await expect(page.locator(".declaration-meta time")).toHaveAttribute("datetime",selectedFixture.instant);
  await expect(page.locator(".summary-config dt").filter({hasText:/^保留时间上限$/}).locator("+ dd")).toHaveText("24h");
  await expect(page.locator(".summary-config dt").filter({hasText:/^ACK 等待$/}).locator("+ dd")).toHaveText("30s");
  await expect(replicas.getByRole("rowheader")).toHaveCount(3);
  await expect(replicas.getByRole("row").filter({has:page.getByRole("rowheader",{name:"nats-1",exact:true})}).locator("td").first()).toHaveText("Leader（主节点）");
  await expect(replicas.getByRole("row").filter({has:page.getByRole("rowheader",{name:"nats-3",exact:true})}).locator("td").first()).toHaveText("Follower（副本节点）");
  await expect(replicas.getByRole("row").filter({has:page.getByRole("rowheader",{name:"nats-3",exact:true})}).locator("td").nth(2)).toHaveText("是");
  await expect(page.getByRole("link",{name:"编辑草稿与预览",exact:true})).toBeVisible();
  const guides=page.locator(".reading-help");await expect(guides).toHaveCount(4);
  const beforeGuides=report.requests.length;
  for(let index=0;index<await guides.count();index++){
    const guide=guides.nth(index),summary=guide.locator("summary");
    await summary.focus();await page.keyboard.press("Enter");await expect(guide).toHaveAttribute("open","");
    await expect(guide.locator("p").first()).toBeVisible();
    await page.keyboard.press("Enter");await expect(guide).not.toHaveAttribute("open","");
  }
  assert.equal(report.requests.length,beforeGuides,"Reading guides must not issue API calls");
  report.guideKeyboardChecks=4;
  report.layout={replicaTable:await replicas.boundingBox(),metrics:await metrics.locator("dl").boundingBox(),configuration:await page.locator(".summary-config").boundingBox()};
  // Explicit test annotation only; never alter values, components or layout for capture.
  await page.locator(".candidate").evaluate(element=>{element.textContent="模拟数据 · 固定视觉对照，非真实运行状态";});
  await page.evaluate(()=>window.scrollTo(0,0));
  await page.screenshot({path:path.join(evidence,"selected-desktop.png")});
  await page.screenshot({path:path.join(evidence,"selected-desktop-full.png"),fullPage:true});
  await page.locator(".session-controls summary").click();
  await expect(page.locator(".identity-panel")).toBeVisible();
  await page.screenshot({path:path.join(evidence,"identity-desktop.png")});
  await page.keyboard.press("Escape");
  await page.locator(".queue-resource-header").screenshot({path:path.join(evidence,"resource-header.png")});
  await page.locator(".queue-summary-grid").screenshot({path:path.join(evidence,"evidence-configuration.png")});
  await page.getByRole("button",{name:"刷新 Queue 页面",exact:true}).click();
  await expect(metrics).toContainText("8420");
  await page.setViewportSize({width:375,height:812});await page.evaluate(()=>window.scrollTo(0,0));
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  assert.equal(await replicas.evaluate(node=>node.scrollWidth>node.clientWidth),true,"localized mobile replica table must scroll internally");
  await page.screenshot({path:path.join(evidence,"selected-mobile.png"),fullPage:true});
  await page.locator(".session-controls summary").click();
  await expect(page.locator(".identity-panel")).toBeVisible();
  await page.screenshot({path:path.join(evidence,"identity-mobile.png")});
  await page.keyboard.press("Escape");
  assert.deepEqual(report.errors,[]);
  assert.ok(report.requests.every(request=>request.method==="GET"&&request.status===200));
  assert.deepEqual(await snapshotCandidate(assets),candidateSnapshot,"Candidate file set or content changed during capture");
  report.inputsVerifiedAt=new Date().toISOString();
  report.passed=true;
}catch(error){report.error=error.message;throw error;}
finally {
  await browser?.close();server.closeAllConnections();if(server.listening)await new Promise(resolve=>server.close(resolve));
  await writeFile(path.join(evidence,"report.json"),JSON.stringify(report,null,2));
  console.log(`Synthetic selected-data capture ${report.passed?"passed":"failed"}: ${evidence}`);
}
