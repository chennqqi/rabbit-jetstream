// Local isolated real-service smoke. No existing listener/container is reused.
import assert from "node:assert/strict";
import {spawn} from "node:child_process";
import {once} from "node:events";
import {createServer, request as httpRequest} from "node:http";
import {createServer as createTCPServer} from "node:net";
import {mkdir, mkdtemp, readFile, writeFile, access} from "node:fs/promises";
import {createRequire} from "node:module";
import {fileURLToPath} from "node:url";
import path from "node:path";
import {createHash,randomBytes} from "node:crypto";
import {routingBindingChecks} from "./routing-binding-live.mjs";
import {queueExportChecks} from "./queue-export-live.mjs";
import {queueImportChecks} from "./queue-import-live.mjs";
import {batchImportChecks} from "./batch-import-live.mjs";
import {snapshotCandidate} from "./candidate-inputs.mjs";
import {consumerDiagnosisChecks} from "./consumer-diagnosis-live.mjs";
import {queueTemplateChecks} from "./queue-templates-live.mjs";
import {bulkChangeChecks} from "./bulk-change-live.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const require = createRequire(path.join(root,"admin-ui/package.json"));
const {chromium, firefox, expect} = require("@playwright/test");
const browserEngine=process.env.RJS_TEST_BROWSER||"chromium";
assert.ok(["chromium","firefox"].includes(browserEngine),"Unsupported browser engine");
const suffix = process.platform === "win32" ? ".exe" : "";
const natsBinary = path.join(root,`bin/nats-server-candidate${suffix}`);
const managementBinary = path.resolve(root,process.env.RJS_TEST_MANAGEMENT_BINARY||`bin/rjs-management-candidate${suffix}`);
const metadataDrift=process.env.RJS_TEST_METADATA_DRIFT==="1";
const metadataBinary=path.join(root,`bin/metadata-drift-candidate${suffix}`);
const embeddedUI=process.env.RJS_TEST_EMBEDDED_UI==="1";
const assets = path.join(root,embeddedUI?"admin-ui/dist":"admin-ui/build-candidate");
await Promise.all([access(natsBinary),access(managementBinary),access(path.join(assets,"index.html"))]);
await mkdir(path.join(root,"artifacts"),{recursive:true});
const evidence = await mkdtemp(path.join(root,"artifacts/webui-live-"));
const operator = randomBytes(24).toString("hex"), auditor = randomBytes(24).toString("hex");
const children = [], logs = new Map();
let browser, front, prometheusServer, alertFiring=true, dropNextApplyResponse=false, droppedApplies=0, forwardedPuts=0;
const report = {startedAt:new Date().toISOString(), browserEngine, embeddedUI, checks:[], passed:false};
const accessibility=process.env.RJS_TEST_A11Y==="1";
const axeSource=accessibility?await readFile(require.resolve("axe-core/axe.min.js"),"utf8"):null;
report.accessibility=[];
async function auditAccessibility(page,name,selector){
  if(!accessibility)return;
  await page.evaluate(axeSource);
  const result=await page.evaluate(async selector=>{
    const result=await window.axe.run(selector?{include:[selector]}:document,{runOnly:selector?{type:"rule",values:["color-contrast"]}:{type:"tag",values:["wcag2a","wcag2aa","wcag21a","wcag21aa","wcag22aa","best-practice"]}});
    const project=items=>items.map(item=>({id:item.id,impact:item.impact,nodes:item.nodes.map(node=>({target:node.target,checks:[...node.any,...node.all,...node.none].map(check=>check.id)}))}));
    return {version:result.testEngine.version,violations:project(result.violations),incomplete:project(result.incomplete),passedRules:result.passes.map(rule=>({id:rule.id,nodeCount:rule.nodes.length}))};
  },selector);
  report.accessibility.push({name,...(selector?{selector}:{}),...result});
  await writeFile(path.join(evidence,`accessibility-${name}.json`),JSON.stringify(result,null,2));
  return result;
}
// Bind evidence to exact executable and frontend inputs, not just filenames.
async function fingerprint(file){
  const bytes=await readFile(file);
  return {path:path.relative(root,file).replaceAll("\\","/"),bytes:bytes.length,sha256:createHash("sha256").update(bytes).digest("hex")};
}
const candidateIndex=await readFile(path.join(assets,"index.html"),"utf8");
const candidateAssets=[...candidateIndex.matchAll(/\/admin\/(assets\/[a-zA-Z0-9_-]+\.(?:js|css))/g)].map(match=>path.join(assets,match[1]));
assert.ok(candidateAssets.some(file=>file.endsWith(".js"))&&candidateAssets.some(file=>file.endsWith(".css")),"Fingerprintable candidate assets required");
const candidateSnapshot=await snapshotCandidate(assets);
async function framedAssetDigest(directory,snapshot){
  const hash=createHash("sha256");
  for(const entry of [...snapshot].sort((a,b)=>a.path.localeCompare(b.path))){
    const name=Buffer.from(entry.path),content=await readFile(path.join(directory,entry.path)),size=Buffer.alloc(8);
    size.writeBigUInt64BE(BigInt(name.length));hash.update(size);hash.update(name);
    size.writeBigUInt64BE(BigInt(content.length));hash.update(size);hash.update(content);
  }
  return hash.digest("hex");
}
const candidateAssetDigest=await framedAssetDigest(assets,candidateSnapshot);
report.inputs=await Promise.all([natsBinary,managementBinary].map(fingerprint));
report.inputs.push(...candidateSnapshot.map(input=>({...input,path:path.relative(root,path.join(assets,input.path)).replaceAll("\\","/")})));
if(metadataDrift)report.inputs.push(await fingerprint(metadataBinary));
const clusterMode=process.env.RJS_TEST_CLUSTER==="1";
report.clusterMode=clusterMode;
const faultMode=process.env.RJS_TEST_RESPONSE_FAULT||"invalid-json";
assert.ok(["invalid-json","socket-reset","attempt-evidence"].includes(faultMode));
// Synthetic receiving-attempt response after a real completed write. It tests
// client replay conservatism, not the server's actual audit failure stage.
const syntheticMutation={schemaVersion:"rjs.mutation-evidence.v1",scope:"receiving-attempt",phase:"audit_intent",resourceEffects:"none"};
const syntheticMutationError=JSON.stringify({error:{code:"audit_unavailable",message:"Synthetic receiving-attempt failure",mutation:syntheticMutation}});
report.responseFault=faultMode;report.faultRequests=[];
const cleanEnv = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/^(RJS_|OTEL_|NATS_)/i.test(key)));

async function unusedPort() {
  const server = createTCPServer();
  server.listen(0,"127.0.0.1");await once(server,"listening");
  const port=server.address().port;
  await new Promise(resolve=>server.close(resolve));return port;
}
function start(name,binary,args,extra={}) {
  const child=spawn(binary,args,{cwd:evidence,env:{...cleanEnv,...extra},windowsHide:true,stdio:["ignore","pipe","pipe"]});
  const chunks=[];logs.set(name,chunks);children.push(child);
  child.stdout.on("data",chunk=>chunks.push(chunk.toString()));child.stderr.on("data",chunk=>chunks.push(chunk.toString()));
  child.on("error",error=>chunks.push(error.message));return child;
}
async function ready(url,child) {
  const deadline=Date.now()+15000;
  while(Date.now()<deadline){
    if(child.exitCode!==null || child.signalCode!==null)throw new Error("Owned test service exited before readiness");
    try {if((await fetch(url,{signal:AbortSignal.timeout(1000)})).ok)return;}catch{}
    await new Promise(resolve=>setTimeout(resolve,100));
  }
  throw new Error("Owned test service readiness deadline exceeded");
}
async function stop(child) {
  if(child.exitCode!==null || child.signalCode!==null)return;
  const done=once(child,"exit");child.kill();await done;
}

try {
  const ports=new Set();while(ports.size<(clusterMode?11:5))ports.add(await unusedPort());
  const allocated=[...ports],managementPort=allocated.pop(),prometheusPort=allocated.pop(),nodeSpecs=[];
  for(let index=0;index<(clusterMode?3:1);index++){
    const [natsPort,monitorPort,routePort]=allocated.splice(0,3),name=`live-${index+1}`;
    const args=["-js","-a","127.0.0.1","-p",String(natsPort),"-m",String(monitorPort),"-sd",path.join(evidence,`${name}-data`),"--name",name];
    if(clusterMode)args.push("--cluster_name","webui-live","--cluster",`nats://127.0.0.1:${routePort}`,"--routes",`nats://127.0.0.1:${index>0?nodeSpecs[0].routePort:allocated[2]}`);
    nodeSpecs.push({name,args,natsPort,monitorPort,routePort,child:start(name,natsBinary,args)});
  }
  for(const spec of nodeSpecs)await ready(`http://127.0.0.1:${spec.monitorPort}/healthz`,spec.child);
  let stableClusterReads=0;
  if(clusterMode)await expect.poll(async()=>{
    try {
      const states=await Promise.all(nodeSpecs.map(async spec=>(await fetch(`http://127.0.0.1:${spec.monitorPort}/jsz`,{signal:AbortSignal.timeout(1000)})).json()));
      const leader=states.find(state=>state.meta_cluster?.replicas?.length===2);
      const ready=leader&&states.every(state=>state.meta_cluster?.cluster_size===3&&state.meta_cluster?.leader===leader.meta_cluster.leader)&&leader.meta_cluster.replicas.every(peer=>peer.current&&!peer.offline);
      stableClusterReads=ready?stableClusterReads+1:0;return stableClusterReads>=3;
    }catch{stableClusterReads=0;return false;}
  },{timeout:20000,intervals:[1000]}).toBe(true);
  report.prometheusQueries=[];
  prometheusServer=createServer((req,res)=>{
    const url=new URL(req.url,"http://localhost");
    if(url.pathname==="/api/v1/rules"&&url.searchParams.get("type")==="alert"){
      const alerts=alertFiring?[{labels:{queue:"live_candidate"},annotations:{},state:"firing",activeAt:"2026-09-11T00:00:00Z",value:"100001"}]:[];
      res.writeHead(200,{"content-type":"application/json"});res.end(JSON.stringify({status:"success",data:{groups:[{name:"rabbit-jetstream-message-safety",file:"alerts.yml",rules:[{state:alertFiring?"firing":"inactive",name:"RabbitJetStreamQueueBacklogHigh",query:"rjs_queue_messages > 100000",duration:900,labels:{severity:"warning"},annotations:{summary:"Queue backlog is above the default safety threshold",description:"Queue backlog threshold."},alerts,health:"ok",lastError:"",evaluationTime:0.01,lastEvaluation:"2026-09-11T00:01:00Z",type:"alerting"}],interval:15,limit:0,evaluationTime:0.01,lastEvaluation:"2026-09-11T00:01:00Z"}]}}));return;
    }
    if(url.pathname!=="/api/v1/query_range"){res.writeHead(404);res.end();return;}
    const query=url.searchParams.get("query"),end=Number(url.searchParams.get("end")),step=Number(url.searchParams.get("step"));
    report.prometheusQueries.push({query,start:url.searchParams.get("start"),end:url.searchParams.get("end"),step:url.searchParams.get("step")});
    const queue=/queue="([A-Za-z0-9_-]+)"/.exec(query??"")?.[1],metric={__name__:query?.split(/[\s{]/,1)[0],instance:"management-test",...(queue?{queue}:{})};
    const values=[[end-step*3,"10"],[end-step*2,"NaN"],[end,"14"]];
    res.writeHead(200,{"content-type":"application/json"});res.end(JSON.stringify({status:"success",warnings:["synthetic missing sample"],data:{resultType:"matrix",result:[{metric,values}]}}));
  });
  prometheusServer.listen(prometheusPort,"127.0.0.1");await once(prometheusServer,"listening");
  const management=start("management",managementBinary,[],{
    RJS_HTTP_ADDR:`127.0.0.1:${managementPort}`,RJS_DEPLOYMENT_PROFILE:clusterMode?"cluster":"standalone",RJS_NATS_URL:nodeSpecs.map(spec=>`nats://127.0.0.1:${spec.natsPort}`).join(","),
    RJS_NATS_MONITOR_URLS:nodeSpecs.map(spec=>`http://127.0.0.1:${spec.monitorPort}`).join(","),RJS_ADMIN_TOKEN:operator,RJS_AUDIT_TOKENS:auditor,
    RJS_CONTROLLER_ENABLED:"false",RJS_LOCAL_DEMO:"false",RJS_METADATA_REPLICAS:clusterMode?"3":"1",
    RJS_PROMETHEUS_URL:`http://127.0.0.1:${prometheusPort}`,RJS_PROMETHEUS_PUBLIC_URL:`http://127.0.0.1:${prometheusPort}`,RJS_PROMETHEUS_ALLOW_INSECURE:"true",
  });
  const backend=`http://127.0.0.1:${managementPort}`;
  await ready(`${backend}/readyz`,management);
  front=createServer(async(req,res)=>{
    try {
      const url=new URL(req.url,"http://localhost");
      if(url.pathname.startsWith("/api/v1/")){
        if(req.method==="PUT")forwardedPuts++;
        const drop=dropNextApplyResponse&&req.method==="PUT";
        if(drop){dropNextApplyResponse=false;droppedApplies++;}
        const proxy=httpRequest(backend+req.url,{method:req.method,headers:req.headers},upstream=>{
          if(req.method==="PUT"&&droppedApplies>0)report.faultRequests.push({requestId:req.headers["x-request-id"]??null,status:upstream.statusCode,dropped:drop});
          // Deliver an unreadable success body after the real backend commits.
          // A pre-header socket reset may be retried by the browser transport.
          if(drop){upstream.resume();upstream.on("end",()=>{if(faultMode==="socket-reset"){res.destroy();return;}res.writeHead(faultMode==="attempt-evidence"?503:200,{"content-type":"application/json"});res.end(faultMode==="attempt-evidence"?syntheticMutationError:'{"');});return;}
          res.writeHead(upstream.statusCode,upstream.headers);upstream.pipe(res);
        });
        proxy.on("error",()=>{if(!res.headersSent)res.writeHead(502,{"content-type":"application/json"});res.end('{"error":{"code":"test_proxy_unavailable"}}');});
        req.pipe(proxy);return;
      }
      if(embeddedUI&&url.pathname.startsWith("/admin/")){
        const proxy=httpRequest(backend+req.url,{method:req.method,headers:req.headers},upstream=>{
          res.writeHead(upstream.statusCode,upstream.headers);upstream.pipe(res);
        });
        proxy.on("error",()=>{if(!res.headersSent)res.writeHead(502);res.end();});
        req.pipe(proxy);return;
      }
      if(!url.pathname.startsWith("/admin/")){res.writeHead(404);res.end();return;}
      const asset=url.pathname.startsWith("/admin/assets/")?url.pathname.slice("/admin/".length):"index.html";
      if(!/^(index\.html|assets\/[a-zA-Z0-9_-]+\.(js|css))$/.test(asset)){res.writeHead(404);res.end();return;}
      const body=await readFile(path.join(assets,asset));
      res.writeHead(200,{"content-type":asset.endsWith(".js")?"text/javascript":asset.endsWith(".css")?"text/css":"text/html",
        "cache-control":"no-store","content-security-policy":"default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"});res.end(body);
    }catch{res.writeHead(500);res.end();}
  });
  front.listen(0,"127.0.0.1");await once(front,"listening");
  const origin=`http://127.0.0.1:${front.address().port}`;
  const api=async(url,options={})=>fetch(origin+url,{...options,headers:{Authorization:`Bearer ${operator}`,"Content-Type":"application/json",...options.headers},signal:AbortSignal.timeout(10000)});
  assert.equal((await fetch(origin+"/api/v1/queues")).status,401);report.checks.push("anonymous-resource-read-denied");
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"live_candidate"},spec:{subjects:["live_candidate.events"],replicas:1,maxPriority:2,storage:"file",retention:{maxMessages:100},delivery:{ackWait:"30s",maxDeliver:5}}};
  if(clusterMode)document.spec.replicas=3;
  const preview=await api("/api/v1/queues/live_candidate/preview",{method:"POST",headers:{"If-None-Match":"*"},body:JSON.stringify(document)});
  assert.equal(preview.status,200);assert.equal((await api("/api/v1/queues/live_candidate")).status,404);report.checks.push("preview-does-not-create-declaration");
  const applied=await api("/api/v1/queues/live_candidate",{method:"PUT",headers:{"If-None-Match":"*"},body:JSON.stringify(document)});
  assert.equal(applied.status,200,await applied.text());
  const declarationResponse=await api("/api/v1/queues/live_candidate"), etag=declarationResponse.headers.get("etag");
  const declaration=await declarationResponse.json();assert.equal(declaration.document.metadata.name,"live_candidate");
  const collectionResponse=await api("/api/v1/queues/live_candidate/consumers"), collection=await collectionResponse.json();
  assert.equal(collectionResponse.status,200);assert.equal(collection.total,3);assert.equal(collection.declaration_revision,etag);
  assert.ok(collection.items.every(row=>row.expected && row.observed && row.status==="present"));report.checks.push("real-priority-collection-three-members");
  browser=await ({chromium,firefox}[browserEngine]).launch({headless:true,...(browserEngine==="chromium"&&process.env.RJS_PLAYWRIGHT_CHANNEL?{channel:process.env.RJS_PLAYWRIGHT_CHANNEL}:{})});
  report.browserVersion=browser.version();
  const page=await browser.newPage({locale:"en-US",viewport:{width:1440,height:1000}});
  const errors=[];page.on("pageerror",error=>errors.push(error.message));
  const routingRequests=new Map();report.routingProbeRequests=[];
  page.on("request",request=>{
    if(!new URL(request.url()).pathname.endsWith("/routing-probe"))return;
    const entry={method:request.method(),startedAt:new Date().toISOString()};routingRequests.set(request,{entry,started:Date.now()});report.routingProbeRequests.push(entry);
  });
  page.on("response",response=>{const value=routingRequests.get(response.request());if(value){value.entry.status=response.status();value.entry.headersAfterMs=Date.now()-value.started;}});
  page.on("requestfinished",request=>{const value=routingRequests.get(request);if(value)value.entry.finishedAfterMs=Date.now()-value.started;});
  page.on("requestfailed",request=>{const value=routingRequests.get(request);if(value){value.entry.failed=true;value.entry.failedAfterMs=Date.now()-value.started;}});
  await page.goto(origin+"/admin/");await auditAccessibility(page,"login");
  for(const [legacy,target] of [["/admin/#overview","overview"],["/admin/index.html#queues","queues"],["/admin/#nodes","nodes"]]){
    await page.goto(origin+legacy);await expect(page).toHaveURL(origin+`/admin/${target}`);
    await expect(page.getByRole("button",{name:"Sign in",exact:true})).toBeVisible();
  }
  await page.goto(origin+"/admin/");
  await page.evaluate(()=>{location.hash="nodes";});await expect(page).toHaveURL(origin+"/admin/nodes");
  await page.goBack();await expect(page).toHaveURL(origin+"/admin/");
  report.checks.push("legacy-hash-startup-index-entry-hashchange-canonical-replacement-back");
  await page.goto(origin+"/admin/");
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  await page.locator(".recovery-login summary").click();
  await page.getByLabel("恢复用 Bearer Token",{exact:true}).fill("unsaved-language-test-input");
  await page.reload();await expect(page.locator("html")).toHaveAttribute("lang","zh-CN");
  await expect(page.getByRole("button",{name:"登录",exact:true})).toBeVisible();
  await page.locator(".recovery-login summary").click();
  await expect(page.getByLabel("恢复用 Bearer Token",{exact:true})).toHaveValue("");
  assert.deepEqual(await page.evaluate(()=>Object.fromEntries(Object.entries(localStorage))),{"rjs.language":"zh"});
  await page.getByRole("button",{name:"English",exact:true}).click();
  await page.reload();await expect(page.locator("html")).toHaveAttribute("lang","en");
  await page.evaluate(()=>Object.defineProperty(window,"localStorage",{configurable:true,get(){throw new Error("Synthetic storage denial");}}));
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  await expect(page.locator("html")).toHaveAttribute("lang","zh-CN");
  await expect(page.getByRole("status")).toContainText("无法记住此次选择");
  await page.reload();await expect(page.locator("html")).toHaveAttribute("lang","en");
  report.checks.push("language-preference-survives-reload-only-allowlisted-value-no-token-input");
  await page.locator(".recovery-login summary").click();
  await page.getByLabel("Recovery bearer token",{exact:true}).fill(auditor);
  await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  await expect(page.locator("#console-content")).toBeFocused();
  const skipNavigation=page.getByRole("button",{name:"Skip to page content",exact:true}),skipURL=page.url();
  await skipNavigation.focus();await expect(skipNavigation).toBeFocused();
  assert.ok((await skipNavigation.boundingBox()).y>=0,"Focused skip control must enter the viewport");
  await page.keyboard.press("Enter");await expect(page.locator("#console-content")).toBeFocused();assert.equal(page.url(),skipURL);
  const routeAnnouncement=page.locator(".route-announcement");
  await expect(page).toHaveTitle("Queue list — Rabbit JetStream");await expect(routeAnnouncement).toHaveText("Queue list — Rabbit JetStream");
  report.checks.push("authenticated-keyboard-skip-navigation-focuses-content-without-route-change");
  const primaryNav=page.getByRole("navigation",{name:"Primary navigation",exact:true});
  const navigate=async label=>{
    await expect(primaryNav).toBeVisible();
    const menu=page.getByRole("button",{name:"Navigation menu",exact:true});
    if(await menu.isVisible()&&await menu.getAttribute("aria-expanded")==="false")await menu.click();
    await primaryNav.getByRole("link",{name:label,exact:true}).click();
  };
  await expect(primaryNav.getByRole("link",{name:"Queue list",exact:true})).toHaveAttribute("aria-current","page");
  await expect(primaryNav.getByRole("link",{name:"Create Queue",exact:true})).toHaveCount(0);
  await expect(page.getByRole("columnheader",{name:"Declared storage",exact:true})).toBeVisible();
  await expect(page.getByRole("columnheader",{name:"Requested replicas",exact:true})).toBeVisible();
  await expect(page.getByRole("columnheader",{name:"Observed state",exact:true})).toBeVisible();
  await expect(page.getByRole("columnheader",{name:"Stored messages",exact:true})).toBeVisible();
  await expect(page.getByRole("columnheader",{name:"Consumers",exact:true})).toBeVisible();
  const candidateQueueRow=page.getByRole("row").filter({has:page.getByRole("link",{name:"live_candidate",exact:true})});
  await expect(candidateQueueRow.getByRole("cell").nth(0)).toContainText("Stream consistent");
  await expect(candidateQueueRow.getByRole("cell").nth(1)).toHaveText("0");
  await expect(candidateQueueRow.getByRole("cell").nth(2)).toHaveText("3");
  await expect(candidateQueueRow.getByRole("cell").nth(3)).toHaveText("file");
  await expect(candidateQueueRow.getByRole("cell").nth(4)).toHaveText(String(clusterMode?3:1));
  report.checks.push("queue-list-joins-real-stream-state-zero-messages-three-consumers-and-declared-deployment");
  await navigate("Consumer list");
  await expect(page.getByRole("heading",{name:"Consumers",exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Collect new generation",exact:true})).toHaveCount(0);
  await expect(page.getByText("Auditors can query an existing generation; only operators can start collection.",{exact:true})).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Consumer index unavailable");
  report.checks.push("global-consumer-auditor-sees-list-first-unavailable-state-without-collection-authority");
  await navigate("Queue list");
  const identityToggle=page.locator(".console-topbar .session-controls summary");
  const identityPanel=page.locator(".identity-panel");
  for(const width of [1440,900,375,320]){
    await page.setViewportSize({width,height:1000});
    await identityToggle.focus();await page.keyboard.press("Enter");
    await expect(page.getByRole("heading",{name:"Verified identity",exact:true})).toBeVisible();
    await expect(identityPanel).toContainText("auditor");
    const box=await identityPanel.boundingBox();
    assert.ok(box.x>=0&&box.x+box.width<=width,`Identity panel stays within ${width}px viewport`);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    await page.keyboard.press("Escape");
    await expect(identityPanel).toBeHidden();await expect(identityToggle).toBeFocused();
    await page.keyboard.press("Space");await expect(identityPanel).toBeVisible();
    await page.keyboard.press("Tab");
    await expect(identityPanel).toBeFocused();
    await page.keyboard.press("Escape");await expect(identityToggle).toBeFocused();await expect(identityPanel).toBeHidden();
    await page.keyboard.press("Enter");await page.keyboard.press("Tab");await page.keyboard.press("Tab");
    await expect(page.getByRole("button",{name:"Clear local session",exact:true})).toBeFocused();
    await expect(identityPanel).toBeHidden();
    await identityToggle.click();await expect(identityPanel).toBeVisible();
    await page.getByRole("button",{name:"简体中文",exact:true}).click();
    await expect(identityPanel).toBeHidden();
    await expect(page.getByRole("button",{name:"English",exact:true})).toBeFocused();
    await page.getByRole("button",{name:"English",exact:true}).click();
  }
  await page.setViewportSize({width:375,height:300});
  await identityToggle.click();await page.keyboard.press("Tab");await expect(identityPanel).toBeFocused();
  assert.ok(await identityPanel.evaluate(element=>element.scrollHeight>element.clientHeight));
  await page.keyboard.press("End");
  await expect.poll(()=>identityPanel.evaluate(element=>element.scrollTop)).toBeGreaterThan(0);
  await page.keyboard.press("Escape");await expect(identityToggle).toBeFocused();
  await page.setViewportSize({width:1440,height:1000});
  report.checks.push("topbar-identity-enter-space-escape-tab-outside-focus-responsive-320-375-900-1440");
  const settingsPuts=forwardedPuts;
  await navigate("Access and settings");
  await expect(page).toHaveTitle("Access and settings — Rabbit JetStream");await expect(routeAnnouncement).toHaveText("Access and settings — Rabbit JetStream");
  await expect(page.getByRole("heading",{name:"Access and settings",exact:true})).toBeVisible();
  await expect(primaryNav.getByRole("link",{name:"Access and settings",exact:true})).toHaveAttribute("aria-current","page");
  const verifiedSettings=await (await api("/api/v1/session",{headers:{Authorization:`Bearer ${auditor}`}})).json();
  await expect(page.getByRole("list",{name:"Reported permissions",exact:true}).locator("code")).toHaveText(verifiedSettings.permissions);
  const capabilityPanel=page.getByRole("region",{name:"Server capabilities",exact:true});
  await expect(capabilityPanel).toContainText("Parser-supported replicas");
  const capabilitiesResponse=await api("/api/v1/console/capabilities",{headers:{Authorization:`Bearer ${auditor}`}});
  assert.equal(capabilitiesResponse.status,200);assert.equal(capabilitiesResponse.headers.get("cache-control"),"no-store");
  const capabilities=await capabilitiesResponse.json();assert.equal(capabilities.schemaVersion,"rjs.console-capabilities.v1");
  assert.deepEqual(capabilities.deployment,{profile:clusterMode?"cluster":"standalone",source:"configuration"});assert.equal(capabilities.qualification.status,"unreported");
  assert.equal(capabilities.queue.requiresExplicitReplicas,true);assert.deepEqual(capabilities.queue.supportedReplicas,[1,3,5]);
  assert.deepEqual(capabilities.queue.defaults,{storage:"file",delivery:{ackWait:"30s",maxDeliver:5}});
  assert.equal((await api("/api/v1/console/capabilities",{headers:{Authorization:""}})).status,401);
  await capabilityPanel.getByText("Canonical omitted-field defaults",{exact:true}).click();await expect(capabilityPanel).toContainText('"ackWait":"30s"');
  await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({...capabilities,deployment:{profile:"cluster",source:"observed"}})}),{times:1});
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();
  await expect(capabilityPanel.getByRole("alert")).toContainText("Capabilities unavailable");await expect(capabilityPanel.locator("dl")).toHaveCount(0);
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();await expect(capabilityPanel).toContainText("Parser-supported replicas");
  const reportedQualification={status:"reported",statement:"local-release-gates-passed; native-linux-soak-and-canary-required",manifestDigest:"a".repeat(64)};
  await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,headers:{ETag:capabilitiesResponse.headers.get("etag")},contentType:"application/json",body:JSON.stringify({...capabilities,qualification:reportedQualification})}),{times:1});
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();
  await expect(capabilityPanel).toContainText("Manifest statement reported");await expect(capabilityPanel).toContainText(reportedQualification.statement);await expect(capabilityPanel).toContainText(reportedQualification.manifestDigest);
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();await expect(capabilityPanel).toContainText("Unreported by this API");
  await writeFile(path.join(evidence,"console-capabilities.json"),JSON.stringify(capabilities,null,2));
  const schemaResponse=await api("/api/v1/console/queue-schema",{headers:{Authorization:`Bearer ${auditor}`,"X-RJS-If-Capabilities-Match":capabilitiesResponse.headers.get("etag")}});
  assert.equal(schemaResponse.status,200);assert.equal(schemaResponse.headers.get("etag"),capabilities.queue.schema.etag);
  assert.equal(schemaResponse.headers.get("content-type"),"application/schema+json");
  const schemaText=await schemaResponse.text();assert.ok(schemaText.includes("9223372036854775807"));
  await writeFile(path.join(evidence,"queue-schema.json"),schemaText);
  assert.equal((await api("/api/v1/console/queue-schema",{headers:{Authorization:""}})).status,401);
  await capabilityPanel.getByText("Verified Queue authoring schema",{exact:true}).click();
  await expect(capabilityPanel).toContainText("9223372036854775807");
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:200,headers:{ETag:capabilities.queue.schema.etag},contentType:"application/schema+json",body:'{"$id":"future"}'}),{times:1});
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();
  await expect(capabilityPanel.getByRole("alert")).toContainText("Capabilities unavailable");await expect(capabilityPanel.locator("dl")).toHaveCount(0);
  await capabilityPanel.getByRole("button",{name:"Refresh server capabilities",exact:true}).click();await expect(capabilityPanel).toContainText("Verified Queue authoring schema");
  report.checks.push("real-authenticated-schema-revision-exact-int64-settings-incompatible-refresh-clears-stale");
  report.checks.push(`real-capabilities-auditor-auth-${clusterMode?"cluster":"standalone"}-configuration-parser-defaults-reported-and-unreported-qualification-incompatible-refresh-clears-stale`);
  await page.getByRole("button",{name:"Switch interface language",exact:true}).click();
  await expect(page).toHaveTitle("访问与设置 — Rabbit JetStream");await expect(routeAnnouncement).toHaveText("访问与设置 — Rabbit JetStream");
  await expect(page.getByRole("heading",{name:"访问与设置",exact:true})).toBeVisible();
  await expect(page.getByRole("region",{name:"服务端能力",exact:true})).toContainText("解析器支持副本数");
  await page.getByRole("button",{name:"切换界面语言",exact:true}).click();
  await page.setViewportSize({width:375,height:812});
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await page.screenshot({path:path.join(evidence,"access-settings-mobile.png"),fullPage:true});
  await page.getByRole("button",{name:"Clear this session",exact:true}).click();
  await expect(page.getByLabel("Username",{exact:true})).toBeVisible();await expect(page.getByLabel("Username",{exact:true})).toBeFocused();
  assert.equal(await page.evaluate(()=>localStorage.getItem("rjs.language")),"en");
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(auditor);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Access and settings",exact:true})).toBeVisible();
  assert.equal(forwardedPuts,settingsPuts);
  await navigate("Queue list");await page.setViewportSize({width:1440,height:1000});
  report.checks.push("settings-verified-permissions-language-session-clear-mobile-no-write");
  assert.equal((await fetch(`${origin}/api/v1/console/build`)).status,401);
  await navigate("Compatibility");
  const buildPanel=page.getByRole("region",{name:"Management build",exact:true}),sdkPanel=page.getByRole("region",{name:"Native SDK contract",exact:true});
  const buildResponse=await api("/api/v1/console/build",{headers:{Authorization:`Bearer ${auditor}`}}),buildMetadata=await buildResponse.json();
  assert.equal(buildResponse.status,200);assert.equal(buildMetadata.uiAssets?.algorithm,"sha256-framed-files-v1");assert.match(buildMetadata.uiAssets?.digest??"",/^[0-9a-f]{64}$/);assert.equal(buildMetadata.uiAssets?.fileCount>0,true);
  if(buildMetadata.revision!==undefined){assert.match(buildMetadata.revision,/^[0-9a-f]{40}$/);assert.ok(["release-build","injected","go-build-info"].includes(buildMetadata.revisionSource));}
  if(embeddedUI){assert.equal(buildMetadata.uiAssets.digest,candidateAssetDigest);assert.equal(buildMetadata.uiAssets.fileCount,candidateSnapshot.length);}
  await expect(buildPanel).toContainText("Go runtime");await expect(buildPanel).toContainText("Revision source");await expect(buildPanel).toContainText(buildMetadata.revisionSource??"Unreported");await expect(buildPanel).toContainText(buildMetadata.uiAssets.digest);await expect(buildPanel).toContainText(String(buildMetadata.uiAssets.fileCount));await expect(sdkPanel).toContainText("native-sdk-implemented-unreleased");
  await page.route("**/api/v1/console/build",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await page.getByRole("button",{name:"Refresh compatibility metadata",exact:true}).click();
  await expect(buildPanel.getByRole("alert")).toBeVisible();await expect(buildPanel.locator("dl")).toHaveCount(0);
  await expect(sdkPanel).toContainText("native-sdk-implemented-unreleased");
  await page.getByRole("button",{name:"Refresh compatibility metadata",exact:true}).click();await expect(buildPanel).toContainText("Go runtime");
  await page.getByRole("button",{name:"简体中文",exact:true}).click();const chineseCompatibility=page.getByRole("region",{name:"兼容性",exact:true});await expect(chineseCompatibility.getByRole("region",{name:"管理进程构建",exact:true})).toContainText("Go 运行时");await expect(chineseCompatibility.getByRole("region",{name:"原生 SDK 契约",exact:true})).toContainText("native-sdk-implemented-unreleased");await page.getByRole("button",{name:"English",exact:true}).click();
  await page.screenshot({path:path.join(evidence,"compatibility.png"),fullPage:true});
  await auditAccessibility(page,"compatibility-desktop");
  report.checks.push(`compatibility-auditor-build-sdk-ui-assets-${embeddedUI?"exact":"reported"}-independent-failure-clears-prior-no-write`);
  await navigate("Queue list");
  for(const [navigation,resource,name] of [["Queue list","queues","live_candidate"],["Stream list","streams",collection.stream]]){
    await navigate(navigation);await expect(page.getByRole("link",{name,exact:true})).toBeVisible();
    const list=page.locator(".queue-list"),initialTime=await list.locator("time").getAttribute("datetime"),listURL=page.url();
    await list.locator("#queue-search").fill("not-submitted");
    await expect(list.locator("time")).not.toHaveAttribute("datetime",initialTime,{timeout:20000});
    await expect(list.locator("#queue-search")).toHaveValue("not-submitted");assert.equal(page.url(),listURL);
    await expect(page.getByRole("link",{name,exact:true})).toBeVisible();
    await list.locator("#queue-search").fill("");
    const time=await list.locator("time").getAttribute("datetime");
    await page.route(`**/api/v1/${resource}?*`,route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
    await list.getByRole("button",{name:"Refresh",exact:true}).click();await expect(list.getByRole("alert")).toBeVisible();
    await expect(list).toContainText("last successful read of this query, not current results");
    await expect(list.locator("time")).toHaveAttribute("datetime",time);await expect(page.getByRole("link",{name,exact:true})).toBeVisible();
    await list.getByRole("button",{name:"Refresh",exact:true}).click();await expect(list.getByRole("alert")).toHaveCount(0);
    await expect(list).not.toContainText("last successful read of this query, not current results");
  }
  await navigate("Queue list");
  report.checks.push("queue-stream-same-query-refresh-failure-retains-labeled-prior-page-time-and-recovers");
  report.checks.push("queue-stream-real-periodic-refresh-keeps-url-query-and-unsubmitted-search");
  await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  page.setDefaultTimeout(15000);
  const tabs=page.getByRole("navigation",{name:"Queue detail tabs",exact:true});
  await expect(page.getByRole("heading",{name:`Observed Stream: ${collection.stream}`,exact:true})).toBeVisible();
  const summaryURL=page.url(),beforeRefreshPuts=forwardedPuts;
  const declarationFailure=route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable","message":"Injected declaration read failure"}}'});
  await page.route("**/api/v1/queues/live_candidate",declarationFailure);
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await expect(page.getByRole("alert")).toContainText("Declaration read unavailable");
  await expect(page.locator(".queue-summary-grid")).toHaveCount(0);
  await expect(page.locator(".declaration-meta")).toHaveCount(0);
  assert.equal(page.url(),summaryURL);assert.equal(forwardedPuts,beforeRefreshPuts);
  await page.unroute("**/api/v1/queues/live_candidate",declarationFailure);
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await expect(page.getByRole("heading",{name:`Observed Stream: ${collection.stream}`,exact:true})).toBeVisible();
  await page.getByRole("link",{name:"All Queues",exact:true}).click();
  await expect(page.getByRole("link",{name:"live_candidate",exact:true})).toBeVisible();
  await page.goBack();await expect(page.getByRole("heading",{name:`Observed Stream: ${collection.stream}`,exact:true})).toBeVisible();
  report.checks.push("queue-header-read-only-refresh-failure-removes-old-evidence-recovery-breadcrumb-back");
  if(clusterMode){
    const replicas=page.getByRole("region",{name:"Replica observations",exact:true});
    const observed=async()=>{const response=await api(`/api/v1/streams/${collection.stream}`);assert.equal(response.status,200);return response.json();};
    await expect.poll(async()=>{const stream=await observed();return stream.cluster?.replicas?.length===2&&stream.cluster.replicas.every(peer=>peer.current&&!peer.offline);},{timeout:20000}).toBe(true);
    await page.getByRole("button",{name:"Refresh observation",exact:true}).click();await expect(replicas.getByRole("rowheader")).toHaveCount(3);
    const initial=await observed(),follower=initial.cluster.replicas[0].name,spec=nodeSpecs.find(item=>item.name===follower);assert.ok(spec);
    await stop(spec.child);
    // Abrupt termination does not publish a graceful shutdown event. The pinned
    // server detects orphans after 150s with a 90s sweep; allow that real delay.
    report.stoppedFollower={name:follower,at:new Date().toISOString()};
    await expect.poll(async()=>{try{return (await observed()).cluster?.replicas?.find(peer=>peer.name===follower)?.offline===true;}catch{return false;}},{timeout:270000,intervals:[1000]}).toBe(true);
    report.stoppedFollower.offlineObservedAt=new Date().toISOString();
    await page.getByRole("button",{name:"Refresh observation",exact:true}).click();
    const offlineRow=replicas.getByRole("row").filter({has:page.getByRole("rowheader",{name:follower,exact:true})});
    await expect(offlineRow.locator("td").nth(2)).toHaveText("Yes");
    await page.screenshot({path:path.join(evidence,"replica-offline.png"),fullPage:true});
    spec.child=start(`${spec.name}-restarted`,natsBinary,spec.args);await ready(`http://127.0.0.1:${spec.monitorPort}/healthz`,spec.child);
    await expect.poll(async()=>{try{const peers=(await observed()).cluster?.replicas;return peers?.length===2&&peers.every(peer=>peer.current&&!peer.offline);}catch{return false;}},{timeout:30000}).toBe(true);
    await page.getByRole("button",{name:"Refresh observation",exact:true}).click();await expect(replicas.getByRole("rowheader")).toHaveCount(3);
    await expect(offlineRow.locator("td").nth(2)).toHaveText("No");
    report.checks.push("real-three-node-r3-follower-stop-offline-render-restart-catchup");
  }else await expect(page.getByRole("region",{name:"Replica evidence",exact:true})).toContainText("A single-replica configuration has no replica fault tolerance.");
  const scopedSummary=page.getByLabel("Scoped summary metrics",{exact:true});
  const primary=declaration.plan.consumer;
  const primaryPath=`/api/v1/streams/${encodeURIComponent(primary.stream)}/consumers/${encodeURIComponent(primary.name)}`;
  const primaryObservation=await (await api(primaryPath)).json();
  await expect(scopedSummary.getByRole("link",{name:`${primary.stream} / ${primary.name}`,exact:true})).toBeVisible();
  const pendingMetric=scopedSummary.locator("dt").filter({hasText:/^Pending \(primary Consumer\)$/}).locator("+ dd");
  await expect(pendingMetric).toHaveText(String(primaryObservation.pending));
  await expect(scopedSummary.locator("dt").filter({hasText:/^Ack pending \(primary Consumer\)$/}).locator("+ dd")).toHaveText(String(primaryObservation.ack_pending));
  const summaryStreamTime=page.locator(".summary-evidence > time"),summaryConsumerTime=scopedSummary.locator("time");
  const summaryDeclaration=await page.locator(".declaration-meta").textContent();
  const initialStreamTime=await summaryStreamTime.getAttribute("datetime"),initialConsumerTime=await summaryConsumerTime.getAttribute("datetime");
  await expect(summaryStreamTime).not.toHaveAttribute("datetime",initialStreamTime,{timeout:20000});
  await expect(summaryConsumerTime).not.toHaveAttribute("datetime",initialConsumerTime,{timeout:20000});
  assert.equal(await page.locator(".declaration-meta").textContent(),summaryDeclaration);
  report.checks.push("summary-periodic-exact-primary-and-stream-independent-times-declaration-unchanged");
  for(const [status,code] of [[503,"unavailable"],[404,"not_found"]]){
    const priorConsumerTime=await summaryConsumerTime.getAttribute("datetime");
    const failureRoute=route=>route.fulfill({status,contentType:"application/json",body:JSON.stringify({error:{code,message:"Injected summary read failure"}})});
    await page.route(`**${primaryPath}`,failureRoute);
    await scopedSummary.getByRole("button",{name:"Refresh summary Consumer",exact:true}).click();
    await expect(scopedSummary.getByRole("alert")).toBeVisible();
    await expect(pendingMetric).toHaveText(status===503?String(primaryObservation.pending):"Unknown");
    if(status===503){
      await expect(summaryConsumerTime).toHaveAttribute("datetime",priorConsumerTime);
      await expect(scopedSummary).toContainText("not current observations");
      await expect(scopedSummary).toContainText("Stale primary Consumer observation");
    }else await expect(summaryConsumerTime).toHaveCount(0);
    await expect(scopedSummary.locator("dt").filter({hasText:/^Stored messages \(Stream\)$/}).locator("+ dd")).toHaveText("0");
    await page.unroute(`**${primaryPath}`,failureRoute);
    await scopedSummary.getByRole("button",{name:"Refresh summary Consumer",exact:true}).click();
    await expect(pendingMetric).toHaveText(String(primaryObservation.pending));
  }
  await page.route(`**${primaryPath}`,route=>route.fulfill({status:200,contentType:"application/json",body:'{"name":'}),{times:1});
  await scopedSummary.getByRole("button",{name:"Refresh summary Consumer",exact:true}).click();await expect(scopedSummary.getByRole("alert")).toBeVisible();await expect(pendingMetric).toHaveText("Unknown");await expect(summaryConsumerTime).toHaveCount(0);await expect(summaryStreamTime).toHaveCount(1);
  await scopedSummary.getByRole("button",{name:"Refresh summary Consumer",exact:true}).click();await expect(pendingMetric).toHaveText(String(primaryObservation.pending));
  report.checks.push("summary-malformed-primary-json-clears-only-consumer-time-and-metrics-recovers");
  await scopedSummary.getByRole("link").click();
  await expect(page.getByRole("heading",{name:`Consumer: ${primary.name}`,exact:true})).toBeVisible();
  await page.goBack();await expect(pendingMetric).toHaveText(String(primaryObservation.pending));
  await page.getByRole("link",{name:"View Consumer collection",exact:true}).click();
  await expect(tabs.getByRole("link",{name:"Consumers",exact:true})).toHaveAttribute("aria-current","page");
  await page.goBack();await expect(pendingMetric).toHaveText(String(primaryObservation.pending));
  report.checks.push("summary-real-primary-identity-navigation-unavailable-retains-labeled-history-missing-clears-consumer-only");
  await page.setViewportSize({width:1487,height:1058});
  await page.evaluate(()=>window.scrollTo(0,0));
  await page.screenshot({path:path.join(evidence,"queue-summary-desktop.png"),fullPage:true});
  await auditAccessibility(page,"queue-summary-desktop");
  const evidenceBox=await page.locator(".summary-evidence").boundingBox(),configBox=await page.locator(".summary-config").boundingBox();
  assert.ok(configBox.x>evidenceBox.x+evidenceBox.width&&Math.abs(configBox.y-evidenceBox.y)<2,"Desktop summary retains evidence/configuration columns");
  await page.setViewportSize({width:375,height:812});
  await page.evaluate(()=>window.scrollTo(0,0));
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth+1),"Summary must not overflow mobile viewport");
  const mobileEvidence=await page.locator(".summary-evidence").boundingBox(),mobileConfig=await page.locator(".summary-config").boundingBox();
  assert.ok(mobileConfig.y>=mobileEvidence.y+mobileEvidence.height,"Mobile evidence precedes configuration");
  await page.screenshot({path:path.join(evidence,"queue-summary-mobile.png"),fullPage:true});
  await auditAccessibility(page,"queue-summary-mobile");
  if(accessibility){
    // Preserve the original incomplete scan. Resolve only its clipped contrast
    // targets by scrolling real content into view, never by changing its CSS.
    const original=report.accessibility.at(-1);
    const targets=original.incomplete.filter(rule=>rule.id==="color-contrast").flatMap(rule=>rule.nodes.map(node=>node.target));
    for(const [index,target] of targets.entries()){
      assert.equal(target.length,1,"Unexpected frame/shadow target needs a dedicated audit");
      const selector=target[0],element=page.locator(selector);await expect(element).toHaveCount(1);
      await element.evaluate(node=>node.scrollIntoView({block:"center",inline:"center",behavior:"instant"}));
      const result=await auditAccessibility(page,`mobile-contrast-visible-${index+1}`,selector);
      await page.screenshot({path:path.join(evidence,`mobile-contrast-visible-${index+1}.png`)});
      assert.deepEqual(result.violations,[]);assert.deepEqual(result.incomplete,[]);
      assert.ok(result.passedRules.some(rule=>rule.id==="color-contrast"&&rule.nodeCount>0),"The actual visible target must be tested, not skipped");
    }
    await page.locator('nav[aria-label="Queue detail tabs"], [aria-label="Replica observations"]').evaluateAll(nodes=>nodes.forEach(node=>node.scrollLeft=0));
    await page.evaluate(()=>window.scrollTo(0,0));
    report.checks.push("mobile-clipped-contrast-targets-scrolled-visible-and-positively-tested");
  }
  const menu=page.getByRole("button",{name:"Navigation menu",exact:true});
  await expect(menu).toHaveAttribute("aria-expanded","false");
  await expect(primaryNav.getByRole("link")).toHaveCount(0);
  await menu.focus();await page.keyboard.press("Enter");
  await expect(menu).toHaveAttribute("aria-expanded","true");
  await page.keyboard.press("Tab");await expect(primaryNav.getByRole("link",{name:"Overview",exact:true})).toBeFocused();
  await page.keyboard.press("Escape");await expect(menu,"Escape returns focus to navigation toggle").toBeFocused();await expect(menu).toHaveAttribute("aria-expanded","false");
  await menu.click();await expect(primaryNav.getByRole("link")).toHaveCount(9);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await page.screenshot({path:path.join(evidence,"mobile-navigation-open.png"),fullPage:true});
  await navigate("Overview");await expect(menu).toHaveAttribute("aria-expanded","false");await expect(menu,"Route selection returns focus to navigation toggle").toBeFocused();
  await page.goBack();await expect(page.locator(".summary-evidence")).toBeVisible();
  await menu.click();await primaryNav.getByRole("link",{name:"Queue list",exact:true}).focus();
  await page.setViewportSize({width:1440,height:1000});
  await expect(menu).toBeHidden();await expect(primaryNav.getByRole("link",{name:"Queue list",exact:true})).toBeFocused();
  await page.setViewportSize({width:375,height:812});await expect(menu,"Desktop to mobile resize preserves navigation focus").toBeFocused();await expect(menu).toHaveAttribute("aria-expanded","false");
  await identityToggle.focus();
  await page.setViewportSize({width:1440,height:1000});await expect(identityToggle).toBeFocused();
  await page.setViewportSize({width:375,height:812});await expect(identityToggle).toBeFocused();
  report.checks.push("mobile-navigation-keyboard-escape-focus-route-collapse-responsive-boundaries");
  await page.setViewportSize({width:1440,height:1000});
  report.checks.push("selected-shell-permission-navigation-identity-disclosure-summary-responsive-grid");
  await tabs.getByRole("link",{name:"Configuration",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Declared configuration (not observed state)",exact:true})).toBeVisible();
  await tabs.getByRole("link",{name:"Routing",exact:true}).click();
  await expect(page.getByText(/No declared Exchange bindings/)).toContainText("rjs.q.live_candidate.p.0");
  await expect(page.getByText(/Declared input Subjects/)).toContainText("live_candidate.events");
  const probePanel=page.getByRole("region",{name:"Routing probe",exact:true});
  await probePanel.getByLabel("Subject",{exact:true}).fill("rjs.q.live_candidate.p.0");
  await probePanel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(probePanel.getByRole("status")).toContainText("Generated Stream Subject matched");
  await expect(probePanel.locator("time")).toHaveCount(1);
  await probePanel.getByLabel("Subject",{exact:true}).fill("live_candidate.events");
  await expect(probePanel.locator("time")).toHaveCount(0);
  await probePanel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(probePanel.getByRole("status")).toContainText("No generated Stream Subject matched");
  await probePanel.getByLabel("Probe mode",{exact:true}).selectOption("exchange");
  await probePanel.getByLabel("Exchange",{exact:true}).fill("events");
  await probePanel.getByLabel("Routing key",{exact:true}).fill("created");
  await probePanel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(probePanel.getByRole("alert")).toContainText("Priority Queues require a literal priority Subject");
  await probePanel.getByLabel("Probe mode",{exact:true}).selectOption("subject");
  await probePanel.getByLabel("Subject",{exact:true}).fill("rjs.q.live_candidate.p.0");
  await probePanel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(probePanel.getByRole("status")).toContainText("Generated Stream Subject matched");
  await page.screenshot({path:path.join(evidence,"routing-probe-desktop.png"),fullPage:true});
  await auditAccessibility(page,"routing-probe-desktop",".routing-probe");
  await page.setViewportSize({width:375,height:900});
  await page.screenshot({path:path.join(evidence,"routing-probe-mobile.png"),fullPage:true});
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"routing probe mobile overflow");
  await auditAccessibility(page,"routing-probe-mobile",".routing-probe");
  await page.setViewportSize({width:1440,height:1000});
  report.checks.push("routing-probe-real-priority-match-nonmatch-input-clearing-unsupported-exchange-recovery-responsive");
  await tabs.getByRole("link",{name:"Events",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Queue management audit",exact:true})).toBeVisible();
  await expect(page.getByText(/Matches in this window:/)).toBeVisible();
  await page.evaluate(()=>{const url=new URL(location.href);url.searchParams.set("ebefore","1");history.pushState({},"",url);dispatchEvent(new PopStateEvent("popstate"));});
  await expect(page.getByText(/No matches in this window; absence of operations/)).toBeVisible();
  await tabs.getByRole("link",{name:"Summary",exact:true}).click();
  assert.equal(new URL(page.url()).searchParams.get("ebefore"),"1");
  await tabs.getByRole("link",{name:"Events",exact:true}).click();
  const eventWindowURL=page.url();
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await expect(page.getByText(/No matches in this window; absence of operations/)).toBeVisible();
  assert.equal(page.url(),eventWindowURL);
  await page.goBack();await page.goBack();await page.goBack();
  report.checks.push("queue-event-cursor-survives-tab-switch-and-page-refresh");
  await tabs.getByRole("link",{name:"Consumers",exact:true}).click();
  report.checks.push("queue-five-tabs-observation-configuration-routing-consumers-management-events");
  const region=page.getByRole("region",{name:"Consumer collection — scroll horizontally for all columns",exact:true});await expect(region.getByRole("rowheader")).toHaveCount(3);
  const queueConsumerView=page.getByRole("region",{name:"Consumers",exact:true});
  const queueConsumerTime=await queueConsumerView.locator("time").getAttribute("datetime"),queueDeclarationBefore=await page.locator(".declaration-meta").textContent(),queueCollectionURL=page.url();
  await page.getByLabel("Consumer name or filter Subject",{exact:true}).fill("not-submitted");
  await expect(queueConsumerView.locator("time")).not.toHaveAttribute("datetime",queueConsumerTime,{timeout:20000});
  await expect(page.getByLabel("Consumer name or filter Subject",{exact:true})).toHaveValue("not-submitted");assert.equal(page.url(),queueCollectionURL);assert.equal(await page.locator(".declaration-meta").textContent(),queueDeclarationBefore);
  await page.getByLabel("Consumer name or filter Subject",{exact:true}).fill("");
  const queueConsumerOld=await queueConsumerView.locator("time").getAttribute("datetime");
  await page.route("**/api/v1/queues/live_candidate/consumers?*",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(queueConsumerView.getByRole("alert")).toContainText("Observation unavailable");
  await expect(queueConsumerView.locator("time")).toHaveAttribute("datetime",queueConsumerOld);await expect(region.getByRole("rowheader")).toHaveCount(3);await expect(queueConsumerView).toContainText("Stale collection observation");
  await page.route("**/api/v1/queues/live_candidate/consumers?*",route=>route.fulfill({status:409,contentType:"application/json",body:'{"error":{"code":"conflict"}}'}),{times:1});
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(queueConsumerView.getByRole("alert")).toContainText("Declaration changed");await expect(region).toHaveCount(0);
  await page.route("**/api/v1/queues/live_candidate/consumers?*",async route=>{const response=await route.fetch(),body=await response.json();body.declaration_revision='"9007199254740993"';await route.fulfill({response,json:body});},{times:1});
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(queueConsumerView).toContainText("not verified at the same revision");assert.equal(await page.locator(".declaration-meta").textContent(),queueDeclarationBefore);
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(region.getByRole("rowheader")).toHaveCount(3);
  report.checks.push("queue-consumer-periodic-query-input-preserved-declaration-not-refreshed-failure-retention-conflict-clear-revision-warning");
  await page.route("**/api/v1/queues/live_candidate/consumers?*",route=>route.fulfill({status:200,contentType:"application/json",body:'{"items":'}),{times:1});
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(queueConsumerView.getByRole("alert")).toContainText("Invalid collection response");await expect(region).toHaveCount(0);await expect(queueConsumerView.locator("time")).toHaveCount(0);assert.equal(await page.locator(".declaration-meta").textContent(),queueDeclarationBefore);
  await queueConsumerView.getByRole("button",{name:"Refresh Consumers",exact:true}).click();await expect(region.getByRole("rowheader")).toHaveCount(3);await expect(queueConsumerView.getByRole("alert")).toHaveCount(0);
  report.checks.push("queue-consumer-malformed-json-clears-rows-time-not-declaration-recovers");
  const target=collection.items[0];
  await page.getByLabel("Consumer name or filter Subject",{exact:true}).fill(target.name);
  await page.getByRole("button",{name:"Filter Consumers",exact:true}).click();
  await expect(region.getByRole("rowheader")).toHaveCount(1);
  const filteredQueueURL=page.url();
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await expect(region.getByRole("rowheader")).toHaveCount(1);
  await expect(page.getByLabel("Consumer name or filter Subject",{exact:true})).toHaveValue(target.name);
  assert.equal(page.url(),filteredQueueURL);
  await region.getByRole("link",{name:target.name,exact:true}).click();
  await expect(page.getByRole("heading",{name:`Consumer: ${target.name}`,exact:true})).toBeVisible();
  await expect(page.getByText("pull",{exact:true})).toBeVisible();
  const consumerView=page.getByRole("region",{name:`Consumer: ${target.name}`,exact:true});
  const firstConsumerTime=await consumerView.locator("time").getAttribute("datetime");
  await expect(consumerView.locator("time")).not.toHaveAttribute("datetime",firstConsumerTime,{timeout:20000});
  const exactConsumerPath=`**/api/v1/streams/${collection.stream}/consumers/${target.name}`;
  await page.route(exactConsumerPath,route=>route.fulfill({status:200,contentType:"application/json",body:'{"name":'}),{times:1});
  await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.getByRole("alert")).toContainText("Invalid Consumer response");await expect(consumerView.locator("time")).toHaveCount(0);await expect(consumerView.locator("dl")).toHaveCount(0);
  await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.locator("time")).toHaveCount(1);await expect(consumerView.getByRole("alert")).toHaveCount(0);
  report.checks.push("consumer-detail-malformed-json-clears-configuration-metrics-time-and-recovers");
  const retainedConsumerTime=await consumerView.locator("time").getAttribute("datetime");
  await page.route(exactConsumerPath,route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"jetstream_unavailable"}}'}),{times:1});
  await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();
  await expect(consumerView.getByRole("alert")).toContainText("Consumer observation unavailable");await expect(consumerView).toContainText("Stale observation");
  await expect(consumerView.locator("time")).toHaveAttribute("datetime",retainedConsumerTime);await expect(consumerView.getByText("pull",{exact:true})).toBeVisible();
  for(const code of ["not_found","read_api_disabled"]){
    await page.route(exactConsumerPath,route=>route.fulfill({status:404,contentType:"application/json",body:JSON.stringify({error:{code}})}),{times:1});
    await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();
    await expect(consumerView.getByRole("alert")).toContainText(code==="not_found"?"Consumer not found":"Resource read API disabled");
    await expect(consumerView.locator("time")).toHaveCount(0);await expect(consumerView.locator("dl")).toHaveCount(0);
    await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.getByText("pull",{exact:true})).toBeVisible();
  }
  await page.screenshot({path:path.join(evidence,"consumer-detail-refresh.png"),fullPage:true});
  for(const field of ["durable","ack_policy"]){
    await page.route(exactConsumerPath,async route=>{const response=await route.fetch(),body=await response.json();body[field]={invalid:true};await route.fulfill({response,json:body});},{times:1});
    await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.getByRole("alert")).toContainText("Invalid Consumer response");
    await expect(consumerView.locator("dl")).toHaveCount(0);await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.getByText("pull",{exact:true})).toBeVisible();
  }
  report.checks.push("consumer-detail-invalid-display-field-types-clear-data-without-render-crash-recovery");
  report.checks.push("consumer-detail-exact-periodic-read-retained-failure-time-missing-disabled-clear-and-recovery");
  await consumerDiagnosisChecks({page,api,consumerView,stream:collection.stream,name:target.name,expect,assert,evidence,path,report});
  await page.goBack();await expect(page.getByLabel("Consumer name or filter Subject",{exact:true})).toHaveValue(target.name);
  await expect(region.getByRole("link",{name:target.name,exact:true})).toBeVisible();
  assert.deepEqual(await page.evaluate(()=>({local:Object.fromEntries(Object.entries(localStorage)),session:sessionStorage.length,cookie:document.cookie})),{local:{"rjs.language":"en"},session:0,cookie:""});
  await page.screenshot({path:path.join(evidence,"collection-desktop.png"),fullPage:true});
  await page.setViewportSize({width:375,height:812});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  assert.equal(await region.evaluate(element=>element.scrollWidth>element.clientWidth),true);
  assert.equal(await region.locator("table").evaluate(element=>element.getBoundingClientRect().width>=800),true);
  await page.screenshot({path:path.join(evidence,"collection-mobile.png"),fullPage:true});
  report.checks.push("auditor-login-list-collection-exact-detail-back-memory-only","desktop-mobile-no-page-overflow");
  const denied=await api("/api/v1/queues/live_candidate/preview",{method:"POST",headers:{Authorization:`Bearer ${auditor}`,"If-Match":etag},body:JSON.stringify(document)});assert.equal(denied.status,403);
  report.checks.push("auditor-preview-denied");
  await navigate("Stream list");
  await page.getByLabel("Stream name contains",{exact:true}).fill(collection.stream);
  await page.getByRole("button",{name:"Search",exact:true}).click();
  await page.getByRole("link",{name:collection.stream,exact:true}).click();
  await expect(page.getByRole("heading",{name:"Observed configuration",exact:true})).toBeVisible();
  const streamRegion=page.getByRole("region",{name:"Stream Consumers",exact:true});
  await expect(streamRegion.getByRole("rowheader")).toHaveCount(3);
  const streamPage=page.getByRole("region",{name:"Stream detail",exact:true}),streamConfigTime=await streamPage.locator("time").first().getAttribute("datetime"),streamConsumerTime=await streamPage.locator("time").last().getAttribute("datetime"),streamCollectionURL=page.url();
  await page.getByLabel("Stream Consumer name or filter Subject",{exact:true}).fill("not-submitted");
  await expect(streamPage.locator("time").last()).not.toHaveAttribute("datetime",streamConsumerTime,{timeout:20000});
  await expect(streamPage.locator("time").first()).not.toHaveAttribute("datetime",streamConfigTime,{timeout:20000});assert.equal(page.url(),streamCollectionURL);await expect(page.getByLabel("Stream Consumer name or filter Subject",{exact:true})).toHaveValue("not-submitted");
  await page.getByLabel("Stream Consumer name or filter Subject",{exact:true}).fill("");
  const streamConsumerOld=await streamPage.locator("time").last().getAttribute("datetime");
  await page.route(`**/api/v1/streams/${collection.stream}/consumers?*`,route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await page.getByRole("button",{name:"Refresh Stream Consumers",exact:true}).click();await expect(streamPage.getByRole("alert")).toContainText("Data unavailable");await expect(streamPage).toContainText("Stale collection observation");
  await expect(streamPage.locator("time").last()).toHaveAttribute("datetime",streamConsumerOld);await expect(streamRegion.getByRole("rowheader")).toHaveCount(3);
  await page.getByRole("button",{name:"Refresh Stream Consumers",exact:true}).click();await expect(streamPage.getByRole("alert")).toHaveCount(0);
  report.checks.push("stream-and-consumer-independent-periodic-reads-preserve-query-input-collection-failure-retains-page-time");
  const streamStateTime=streamPage.locator(":scope > time"),streamStatePath=`**/api/v1/streams/${collection.stream}`;
  for(const [status,code] of [[503,"unavailable"],[404,"not_found"],[404,"read_api_disabled"],[403,"forbidden"]]){
    const before=await streamStateTime.getAttribute("datetime");
    const failureRoute=route=>route.fulfill({status,contentType:"application/json",body:JSON.stringify({error:{code}})});
    await page.route(streamStatePath,failureRoute);
    await streamPage.getByRole("button",{name:"Refresh Stream",exact:true}).click();await expect(streamPage.getByRole("alert")).toHaveCount(1);
    if(status===503){await expect(streamStateTime).toHaveAttribute("datetime",before);await expect(streamPage).toContainText("Stale Stream observation");await expect(streamPage).toContainText("not current observations");}
    else{await expect(streamStateTime).toHaveCount(0);await expect(streamPage.getByRole("heading",{name:"Observed configuration",exact:true})).toHaveCount(0);await expect(streamPage.getByText("Full Stream response",{exact:true})).toHaveCount(0);}
    await expect(streamRegion.getByRole("rowheader")).toHaveCount(3);
    await page.unroute(streamStatePath,failureRoute);await streamPage.getByRole("button",{name:"Refresh Stream",exact:true}).click();await expect(streamStateTime).toHaveCount(1);await expect(streamPage.getByRole("alert")).toHaveCount(0);
  }
  await page.screenshot({path:path.join(evidence,"stream-detail-refresh.png"),fullPage:true});
  await page.route(streamStatePath,route=>route.fulfill({status:200,contentType:"application/json",body:'{"name":'}),{times:1});
  await streamPage.getByRole("button",{name:"Refresh Stream",exact:true}).click();await expect(streamPage.getByRole("alert")).toContainText("Invalid response");await expect(streamStateTime).toHaveCount(0);await expect(streamRegion.getByRole("rowheader")).toHaveCount(3);
  await streamPage.getByRole("button",{name:"Refresh Stream",exact:true}).click();await expect(streamStateTime).toHaveCount(1);await expect(streamPage.getByRole("alert")).toHaveCount(0);
  await page.getByRole("button",{name:"简体中文",exact:true}).click();const chineseStream=page.getByRole("region",{name:"Stream 详情",exact:true}),chineseStreamConsumers=chineseStream.getByRole("region",{name:"Stream Consumer 列表",exact:true});await expect(chineseStreamConsumers.getByRole("rowheader")).toHaveCount(3);await expect(chineseStreamConsumers.getByRole("columnheader")).toHaveText(["Consumer","模式","待投递","待确认"]);for(const label of ["保留策略","丢弃策略","存储字节","Consumer 数","首序列号","末序列号"])await expect(chineseStream.locator("dt").filter({hasText:new RegExp(`^${label}$`)})).toHaveCount(1);await expect(chineseStream.getByRole("navigation",{name:"Stream Consumer 分页",exact:true})).toBeVisible();await page.getByRole("button",{name:"English",exact:true}).click();
  report.checks.push("stream-state-unavailable-retains-history-missing-disabled-denied-clear-only-state-recovery");
  await page.getByLabel("Stream Consumer page size",{exact:true}).selectOption("1");
  const streamPagination=page.getByRole("navigation",{name:"Stream Consumer pagination",exact:true});
  await streamPagination.getByRole("button",{name:"Next",exact:true}).click();
  await expect(streamPagination).toContainText("2–2 / 3");
  await page.getByLabel("Stream Consumer name or filter Subject",{exact:true}).fill(target.name);
  await page.getByRole("button",{name:"Filter Stream Consumers",exact:true}).click();
  await expect(streamPagination).toContainText("1–1 / 1");
  assert.equal(new URL(page.url()).searchParams.get("offset"),"0");
  await page.getByLabel("Stream Consumer mode",{exact:true}).selectOption("push");
  await expect(page.getByText("No matching Consumers.",{exact:true})).toBeVisible();
  await page.getByLabel("Stream Consumer mode",{exact:true}).selectOption("pull");
  await page.getByLabel("Stream Consumer sort",{exact:true}).selectOption("desc");
  await streamRegion.getByRole("link",{name:target.name,exact:true}).click();
  await expect(page.getByRole("heading",{name:`Consumer: ${target.name}`,exact:true})).toBeVisible();
  await page.goBack();await expect(streamRegion.getByRole("rowheader")).toHaveCount(1);
  await expect(page.getByLabel("Stream Consumer name or filter Subject",{exact:true})).toHaveValue(target.name);
  await expect(page.getByLabel("Stream Consumer mode",{exact:true})).toHaveValue("pull");
  await expect(page.getByLabel("Stream Consumer sort",{exact:true})).toHaveValue("desc");
  for(let step=0;step<7;step++)await page.goBack();
  await expect(page.getByLabel("Stream name contains",{exact:true})).toHaveValue(collection.stream);
  report.checks.push("auditor-stream-search-consumer-server-filter-reset-mode-sort-pagination-history");
  await navigate("Overview");
  await expect(page.getByRole("region",{name:"Declared Queues",exact:true})).toContainText("Declaration total: 1");
  await expect(page.getByRole("region",{name:"Management and account",exact:true})).toContainText("Account Consumers");
  await expect(page.getByRole("region",{name:"Monitoring summary",exact:true})).toContainText(`Configured endpoints: ${nodeSpecs.length}`);
  report.checks.push("overview-independent-real-account-declaration-monitoring-sources");
  const accountPanel=page.getByRole("region",{name:"Management and account",exact:true});
  const firstOverviewTime=await accountPanel.locator("time").getAttribute("datetime");
  await expect(accountPanel.locator("time")).not.toHaveAttribute("datetime",firstOverviewTime,{timeout:20000});
  const beforeFailedRefresh=await accountPanel.locator("time").getAttribute("datetime");
  await page.route("**/api/v1/info",route=>route.fulfill({status:503,contentType:"application/json",body:JSON.stringify({error:{code:"unavailable",message:"Synthetic refresh failure"}})}),{times:1});
  await page.getByRole("button",{name:"Refresh overview",exact:true}).click();
  await expect(accountPanel.getByRole("alert")).toBeVisible();
  await expect(accountPanel).toContainText("Stale observation");await expect(accountPanel).toContainText("Account Consumers");
  await expect(accountPanel.locator("time")).toHaveAttribute("datetime",beforeFailedRefresh);
  await expect(page.getByRole("region",{name:"Declared Queues",exact:true})).toContainText("Declaration total: 1");
  await expect(page.getByRole("button",{name:"Refresh overview",exact:true})).toBeEnabled();
  await page.getByRole("button",{name:"Refresh overview",exact:true}).click();
  await expect(accountPanel.getByRole("alert")).toHaveCount(0);await expect(accountPanel).not.toContainText("Stale observation");
  await expect(accountPanel.locator("time")).not.toHaveAttribute("datetime",beforeFailedRefresh);
  await page.getByRole("button",{name:"简体中文",exact:true}).click();const chineseOverview=page.getByRole("region",{name:"总览",exact:true});await expect(chineseOverview.getByRole("region",{name:"监控摘要",exact:true})).toContainText(`配置端点数: ${nodeSpecs.length}`);await expect(chineseOverview.getByRole("region",{name:"管理服务与账户",exact:true})).toContainText("账户 Consumer 数");await expect(chineseOverview.getByRole("region",{name:"已声明 Queue",exact:true})).toContainText("声明总数: 1");await page.getByRole("button",{name:"English",exact:true}).click();
  report.checks.push("overview-real-auto-refresh-failed-source-retains-old-value-time-stale-label-sibling-manual-recovery");
  await page.route("**/api/v1/info",route=>route.fulfill({status:404,contentType:"application/json",body:'{"error":{"code":"read_api_disabled"}}'}),{times:1});
  await page.getByRole("button",{name:"Refresh overview",exact:true}).click();
  await expect(accountPanel.getByRole("alert")).toContainText("Resource read API disabled");await expect(accountPanel.locator("time")).toHaveCount(0);
  await expect(accountPanel).not.toContainText("Account Consumers");
  await expect(page.getByRole("region",{name:"Declared Queues",exact:true})).toContainText("Declaration total: 1");
  await page.getByRole("button",{name:"Refresh overview",exact:true}).click();await expect(accountPanel).toContainText("Account Consumers");
  report.checks.push("overview-explicit-read-disablement-clears-source-values-time-preserves-sibling-recovers");
  await navigate("Access and settings");
  const refreshChoice=page.getByLabel("Read-page refresh interval",{exact:true});
  await expect(refreshChoice).toHaveValue("10");await refreshChoice.selectOption("0");
  assert.equal(await page.evaluate(()=>localStorage.getItem("rjs.overview-refresh-seconds")),"0");
  await navigate("Overview");await expect(accountPanel.locator("time")).toHaveCount(1);
  await expect(page.getByRole("region",{name:"Overview",exact:true})).toContainText("Automatic refresh disabled");
  const manualTime=await accountPanel.locator("time").getAttribute("datetime");
  let manualInfoRequests=0;const countManual=request=>{if(new URL(request.url()).pathname==="/api/v1/info")manualInfoRequests++;};
  page.on("request",countManual);await page.waitForTimeout(11000);
  assert.equal(manualInfoRequests,0);await expect(accountPanel.locator("time")).toHaveAttribute("datetime",manualTime);
  await page.getByRole("button",{name:"Refresh overview",exact:true}).click();
  await expect(accountPanel.locator("time")).not.toHaveAttribute("datetime",manualTime);assert.equal(manualInfoRequests,1);page.off("request",countManual);
  for(const [navigation,resource] of [["Queue list","queues"],["Stream list","streams"]]){
    await navigate(navigation);const list=page.locator(".queue-list");await expect(list.locator("time")).toHaveCount(1);
    await expect(list).toContainText("Automatic refresh disabled");const initial=await list.locator("time").getAttribute("datetime");
    const requests=[];const count=request=>{if(new URL(request.url()).pathname===`/api/v1/${resource}`)requests.push(request.method());};
    page.on("request",count);await page.waitForTimeout(11000);assert.deepEqual(requests,[]);
    await expect(list.locator("time")).toHaveAttribute("datetime",initial);
    await list.getByRole("button",{name:"Refresh",exact:true}).click();await expect(list.locator("time")).not.toHaveAttribute("datetime",initial);
    assert.deepEqual(requests,["GET"]);page.off("request",count);
  }
  report.checks.push("queue-stream-shared-manual-preference-entry-read-no-periodic-read-explicit-get-only");
  await navigate("Node list");const manualNodes=page.getByRole("region",{name:"Nodes",exact:true});
  await expect(manualNodes.locator("time")).toHaveCount(1);await expect(manualNodes).toContainText("Automatic refresh disabled");
  const manualNodeTime=await manualNodes.locator("time").getAttribute("datetime");
  let manualNodeReads=0;const countManualNodes=request=>{if(new URL(request.url()).pathname==="/api/v1/nodes")manualNodeReads++;};
  page.on("request",countManualNodes);await page.waitForTimeout(11000);assert.equal(manualNodeReads,0);
  await manualNodes.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(manualNodes.locator("time")).not.toHaveAttribute("datetime",manualNodeTime);
  assert.equal(manualNodeReads,1);page.off("request",countManualNodes);
  report.checks.push("nodes-shared-manual-preference-entry-read-no-periodic-requests-explicit-refresh");
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await expect(summaryConsumerTime).toHaveCount(1);await expect(summaryStreamTime).toHaveCount(1);
  await expect(page.locator(".summary-evidence")).toContainText("Automatic summary refresh disabled");
  const manualSummaryStream=await summaryStreamTime.getAttribute("datetime"),manualSummaryConsumer=await summaryConsumerTime.getAttribute("datetime");
  const summaryRequests=[],summaryStreamPath=`/api/v1/streams/${encodeURIComponent(primary.stream)}`;
  const countSummary=request=>{const pathname=new URL(request.url()).pathname;if([primaryPath,summaryStreamPath].includes(pathname))summaryRequests.push([request.method(),pathname]);};
  page.on("request",countSummary);await page.waitForTimeout(11000);assert.deepEqual(summaryRequests,[]);
  await scopedSummary.getByRole("button",{name:"Refresh summary Consumer",exact:true}).click();
  await expect(summaryConsumerTime).not.toHaveAttribute("datetime",manualSummaryConsumer);await expect(summaryStreamTime).toHaveAttribute("datetime",manualSummaryStream);
  const refreshedConsumer=await summaryConsumerTime.getAttribute("datetime");
  await page.getByRole("button",{name:"Refresh observation",exact:true}).click();
  await expect(summaryStreamTime).not.toHaveAttribute("datetime",manualSummaryStream);await expect(summaryConsumerTime).toHaveAttribute("datetime",refreshedConsumer);
  assert.deepEqual(summaryRequests,[["GET",primaryPath],["GET",summaryStreamPath]]);page.off("request",countSummary);
  report.checks.push("summary-shared-manual-preference-no-periodic-read-explicit-source-only-independent-times");
  for(const kind of ["queue","stream"]){
    await navigate(kind==="queue"?"Queue list":"Stream list");await page.getByRole("link",{name:kind==="queue"?"live_candidate":collection.stream,exact:true}).click();
    if(kind==="queue")await page.getByRole("navigation",{name:"Queue detail tabs",exact:true}).getByRole("link",{name:"Consumers",exact:true}).click();
    await expect((kind==="queue"?region:streamRegion).getByRole("rowheader")).toHaveCount(3);
    const view=kind==="queue"?queueConsumerView:streamPage,time=kind==="queue"?view.locator("time"):view.locator("time").last();
    await expect(time).toHaveCount(1);await expect(view).toContainText("manual refresh only");const before=await time.getAttribute("datetime");
    const requests=[];const count=request=>{if(new URL(request.url()).pathname===`/api/v1/${kind==="queue"?"queues/live_candidate":`streams/${collection.stream}`}/consumers`)requests.push(request.method());};
    page.on("request",count);await page.waitForTimeout(11000);assert.deepEqual(requests,[]);await expect(time).toHaveAttribute("datetime",before);
    await page.getByRole("button",{name:kind==="queue"?"Refresh Consumers":"Refresh Stream Consumers",exact:true}).click();await expect(time).not.toHaveAttribute("datetime",before);assert.deepEqual(requests,["GET"]);page.off("request",count);
    if(kind==="stream"){
      await expect(streamPage).toContainText("Automatic Stream refresh disabled");const beforeState=await streamStateTime.getAttribute("datetime"),beforeCollection=await time.getAttribute("datetime");
      const reads=[],countState=request=>{if(new URL(request.url()).pathname===`/api/v1/streams/${collection.stream}`)reads.push(request.method());};
      page.on("request",countState);await page.waitForTimeout(11000);assert.deepEqual(reads,[]);await expect(streamStateTime).toHaveAttribute("datetime",beforeState);
      await streamPage.getByRole("button",{name:"Refresh Stream",exact:true}).click();await expect(streamStateTime).not.toHaveAttribute("datetime",beforeState);await expect(time).toHaveAttribute("datetime",beforeCollection);assert.deepEqual(reads,["GET"]);page.off("request",countState);
      const afterState=await streamStateTime.getAttribute("datetime");await page.getByLabel("Stream Consumer page size",{exact:true}).selectOption("1");await expect(streamRegion.getByRole("rowheader")).toHaveCount(1);await expect(streamStateTime).toHaveAttribute("datetime",afterState);
      report.checks.push("stream-state-shared-manual-preference-no-periodic-get-explicit-source-only-query-change-preserves-state-time");
    }
  }
  report.checks.push("queue-stream-consumer-collections-shared-manual-preference-entry-only-and-explicit-get");
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("navigation",{name:"Queue detail tabs",exact:true}).getByRole("link",{name:"Consumers",exact:true}).click();
  await region.getByRole("link",{name:target.name,exact:true}).click();await expect(consumerView.locator("time")).toHaveCount(1);
  await expect(consumerView).toContainText("Automatic refresh disabled");const manualConsumerTime=await consumerView.locator("time").getAttribute("datetime");
  let manualConsumerReads=0;const countManualConsumer=request=>{if(new URL(request.url()).pathname===`/api/v1/streams/${collection.stream}/consumers/${target.name}`)manualConsumerReads++;};
  page.on("request",countManualConsumer);await page.waitForTimeout(11000);assert.equal(manualConsumerReads,0);
  await consumerView.getByRole("button",{name:"Refresh Consumer",exact:true}).click();await expect(consumerView.locator("time")).not.toHaveAttribute("datetime",manualConsumerTime);
  assert.equal(manualConsumerReads,1);page.off("request",countManualConsumer);
  report.checks.push("consumer-detail-shared-manual-preference-entry-only-explicit-read-no-periodic-read");
  await navigate("Access and settings");await expect(refreshChoice).toHaveValue("0");
  await page.evaluate(()=>{window.__restoreRefreshStorage=Object.getOwnPropertyDescriptor(Storage.prototype,"setItem");Object.defineProperty(Storage.prototype,"setItem",{configurable:true,value(){throw Error("Synthetic storage denial");}});});
  await refreshChoice.selectOption("30");await expect(page.getByRole("status").filter({hasText:"Refresh preference applies to this page session only"})).toBeVisible();
  await page.evaluate(()=>{Object.defineProperty(Storage.prototype,"setItem",window.__restoreRefreshStorage);delete window.__restoreRefreshStorage;});
  await refreshChoice.selectOption("10");await expect(refreshChoice).toHaveValue("10");
  assert.deepEqual(await page.evaluate(()=>Object.fromEntries(Object.entries(localStorage))),{"rjs.language":"en","rjs.overview-refresh-seconds":"10"});
  report.checks.push("overview-refresh-preference-manual-no-periodic-requests-explicit-read-storage-denial-allowlisted-persistence");
  await navigate("Node list");
  const nodeSnapshot=await (await api("/api/v1/nodes")).json();
  const nodeID=nodeSnapshot.nodes[0].id;assert.ok(nodeID);assert.equal(nodeSnapshot.nodes[0].sources.varz.available,true);
  await page.getByRole("button",{name:"简体中文",exact:true}).click();await expect(page.getByRole("region",{name:"节点",exact:true}).getByRole("region",{name:"节点集合",exact:true})).toContainText(nodeID);await page.getByRole("button",{name:"English",exact:true}).click();
  await page.getByRole("region",{name:"Node collection",exact:true}).getByRole("link",{name:nodeID,exact:true}).click();
  await expect(page.getByRole("heading",{name:`Node: ${nodeID}`,exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Source observations",exact:true})).toBeVisible();
  const nodeView=page.getByRole("region",{name:"Nodes",exact:true}),nodeTime=await nodeView.locator("time").getAttribute("datetime");
  await expect(nodeView.locator("time")).not.toHaveAttribute("datetime",nodeTime,{timeout:20000});
  const nodeRetainedTime=await nodeView.locator("time").getAttribute("datetime");
  await page.route("**/api/v1/nodes",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(nodeView.getByRole("alert")).toContainText("Node observation unavailable");
  await expect(nodeView).toContainText("Stale observation");await expect(nodeView).toContainText("last successful read, not current observations");
  await expect(nodeView.locator("time")).toHaveAttribute("datetime",nodeRetainedTime);await expect(nodeView.getByRole("heading",{name:"Source observations",exact:true})).toBeVisible();
  await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(nodeView.getByRole("alert")).toHaveCount(0);
  const missingNodeID=async route=>{const response=await route.fetch(),body=await response.json();for(const node of body.nodes)if(node.id===nodeID)delete node.id;await route.fulfill({response,json:body});};
  await page.route("**/api/v1/nodes",missingNodeID,{times:1});await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();
  await expect(nodeView).toContainText("cannot uniquely identify the node");await expect(nodeView.getByRole("heading",{name:"Source observations",exact:true})).toHaveCount(0);
  await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(nodeView.getByRole("heading",{name:"Source observations",exact:true})).toBeVisible();
  report.checks.push("node-detail-periodic-read-failure-retains-labeled-time-successful-missing-id-clears-detail-recovery");
  await page.route("**/api/v1/nodes",route=>route.fulfill({status:404,contentType:"application/json",body:'{"error":{"code":"read_api_disabled"}}'}),{times:1});
  await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(nodeView.getByRole("alert")).toContainText("Resource read API disabled");
  await expect(nodeView.locator("time")).toHaveCount(0);await expect(nodeView.getByRole("heading",{name:"Source observations",exact:true})).toHaveCount(0);
  await nodeView.getByRole("button",{name:"Refresh nodes",exact:true}).click();await expect(nodeView.getByRole("heading",{name:"Source observations",exact:true})).toBeVisible();
  report.checks.push("node-explicit-read-disablement-clears-old-identity-metrics-time-and-recovers");
  const jsMetrics=page.locator('dl[aria-label="JetStream metrics"]');
  for(const [key,label] of [["memory_bytes","JetStream memory bytes"],["messages","Stored messages"],["meta_cluster_size","Metadata cluster size"],["meta_pending","Metadata pending"]]){
    const value=nodeSnapshot.nodes[0].jetstream[key];
    await expect(jsMetrics.locator("dt").filter({hasText:new RegExp(`^${label}$`)}).locator("+ dd")).toHaveText(value===undefined?"Unknown":String(value));
  }
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth+1),"Node detail must not overflow the mobile viewport");
  await page.screenshot({path:path.join(evidence,"node-js-metrics.png"),fullPage:true});
  report.checks.push("node-jsz-reported-zero-versus-unreported-metrics");
  assert.equal(nodeSnapshot.nodes[0].jetstream.enabled,true);
  await expect(page.getByText("Enabled: true",{exact:true})).toBeVisible();
  for(const enabled of [undefined,false]){
    const injectEnablement=async route=>{
      const response=await route.fetch(),body=await response.json();
      for(const node of body.nodes){if(enabled===undefined)delete node.jetstream.enabled;else node.jetstream.enabled=enabled;}
      await route.fulfill({response,json:body});
    };
    await page.route("**/api/v1/nodes",injectEnablement);
    await page.getByRole("button",{name:"Refresh nodes",exact:true}).click();
    await expect(page.getByText(`Enabled: ${enabled===undefined?"Unknown":"false"}`,{exact:true})).toBeVisible();
    await page.unroute("**/api/v1/nodes",injectEnablement);
  }
  await page.getByRole("button",{name:"Refresh nodes",exact:true}).click();
  await expect(page.getByText("Enabled: true",{exact:true})).toBeVisible();
  report.checks.push("real-js-enabled-and-synthetic-missing-versus-explicit-disabled-rendering");
  await expect(page.locator("dt").filter({hasText:/^Slow consumers$/}).locator("+ dd")).toHaveText("0");
  await expect(page.getByRole("heading",{name:"Connected peers",exact:true})).toBeVisible();
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  const chineseNode=page.getByRole("region",{name:"节点",exact:true});await expect(chineseNode.getByLabel("JetStream 指标",{exact:true})).toBeVisible();
  await expect(chineseNode.locator("dt").filter({hasText:/^慢消费者数$/}).locator("+ dd")).toHaveText("0");
  await page.getByRole("button",{name:"English",exact:true}).click();
  await page.goBack();await expect(page.getByRole("region",{name:"Node collection",exact:true})).toBeVisible();
  report.checks.push("real-node-id-detail-source-availability-history");
  await page.getByRole("region",{name:"Node collection",exact:true}).getByRole("link",{name:nodeID,exact:true}).click();
  await page.getByRole("link",{name:"View node connections",exact:true}).click();
  const connectionsView=page.getByRole("region",{name:"Node connections",exact:true}),connectionRows=page.getByRole("region",{name:"Connection rows",exact:true});
  const connectionAPI=`/api/v1/nodes/${encodeURIComponent(nodeID)}/connections`,connectionPattern=`**${connectionAPI}?*`;
  const connectionPage=await (await api(connectionAPI)).json();
  await expect(connectionRows.getByRole("rowheader")).toHaveCount(connectionPage.items.length);
  await expect(connectionsView).toContainText(`Reported node total: ${connectionPage.total}`);
  await expect(connectionsView).toContainText("Client-name search has a 1,000-open-connection node limit");
  const exactCID=String(connectionPage.items[0].cid),cidSearch=page.getByLabel("Exact CID",{exact:true});
  await cidSearch.fill(exactCID);await connectionsView.getByRole("button",{name:"Search",exact:true}).click();
  await expect(page).toHaveURL(new RegExp(`[?&]cid=${exactCID}(?:&|$)`));await expect(connectionRows.getByRole("rowheader")).toHaveCount(1);await expect(connectionRows.getByRole("rowheader")).toHaveText(exactCID);await expect(connectionsView).toContainText("Reported node total: 1");await expect(connectionsView.getByRole("navigation",{name:"Connection pagination",exact:true})).toHaveCount(0);
  await cidSearch.fill("");await connectionsView.getByRole("button",{name:"Search",exact:true}).click();await expect(page).not.toHaveURL(/[?&]cid=/);await expect(connectionRows.getByRole("rowheader")).toHaveCount(connectionPage.items.length);
  report.checks.push("connections-exact-cid-server-search-filtered-total-no-page-local-filter");
  const identityRequests=[];const observeIdentity=request=>{if(new URL(request.url()).pathname===`${connectionAPI}/search`)identityRequests.push({method:request.method(),url:request.url(),body:request.postDataJSON()});};page.on("request",observeIdentity);
  await page.getByLabel("Exact identity field",{exact:true}).selectOption("name");await page.getByLabel("Exact value",{exact:true}).fill("rabbit-jetstream");await connectionsView.getByRole("button",{name:"Search identity",exact:true}).click();
  await expect(connectionsView.getByRole("button",{name:"Clear identity search",exact:true})).toBeVisible();await expect(connectionRows.getByRole("rowheader").first()).toBeVisible();assert.equal(new URL(page.url()).searchParams.has("value"),false);assert.equal(page.url().includes("rabbit-jetstream"),false);assert.equal(identityRequests.length,1);assert.equal(identityRequests[0].method,"POST");assert.equal(new URL(identityRequests[0].url).search,"");assert.deepEqual(identityRequests[0].body,{kind:"name",value:"rabbit-jetstream",offset:0,limit:50});
  await page.getByLabel("Exact identity field",{exact:true}).selectOption("user");await page.getByLabel("Exact value",{exact:true}).fill("unsubmitted-draft");await connectionsView.getByRole("button",{name:"Refresh connections",exact:true}).click();await expect.poll(()=>identityRequests.length).toBe(2);assert.deepEqual(identityRequests[1].body,{kind:"name",value:"rabbit-jetstream",offset:0,limit:50});
  await connectionsView.getByRole("button",{name:"Clear identity search",exact:true}).click();await expect(connectionsView.getByRole("button",{name:"Clear identity search",exact:true})).toHaveCount(0);page.off("request",observeIdentity);
  report.checks.push("connections-exact-client-name-post-no-url-or-response-echo-frozen-refresh-clear");
  const connectionTime=connectionsView.locator("time").last(),oldConnectionTime=await connectionTime.getAttribute("datetime");
  await expect(connectionTime).not.toHaveAttribute("datetime",oldConnectionTime,{timeout:20000});
  for(const [status,code] of [[503,"connections_unavailable"],[409,"node_identity_ambiguous"],[404,"not_found"],[403,"forbidden"]]){
    const before=await connectionTime.getAttribute("datetime");
    await page.route(connectionPattern,route=>route.fulfill({status,contentType:"application/json",body:JSON.stringify({error:{code}})}),{times:1});
    await connectionsView.getByRole("button",{name:"Refresh connections",exact:true}).click();await expect(connectionsView.getByRole("alert")).toBeVisible();
    if(status===503){await expect(connectionTime).toHaveAttribute("datetime",before);await expect(connectionsView).toContainText("Historical page");await expect(connectionsView).toContainText("Stale observation");}
    else{await expect(connectionRows).toHaveCount(0);await expect(connectionsView.locator("time")).toHaveCount(0);}
    await connectionsView.getByRole("button",{name:"Refresh connections",exact:true}).click();await expect(connectionRows.getByRole("rowheader")).toHaveCount(connectionPage.items.length);await expect(connectionsView.getByRole("alert")).toHaveCount(0);await expect(connectionsView.getByRole("button",{name:"Refresh connections",exact:true})).toBeEnabled();
  }
  for(const field of ["observed_at","read_at"]){
    await page.route(connectionPattern,route=>route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({...connectionPage,[field]:"2026-02-30T12:00:00Z"})}),{times:1});
    await connectionsView.getByRole("button",{name:"Refresh connections",exact:true}).click();
    await expect(connectionsView.getByRole("alert")).toBeVisible();await expect(connectionRows).toHaveCount(0);await expect(connectionsView.locator("time")).toHaveCount(0);
    await connectionsView.getByRole("button",{name:"Refresh connections",exact:true}).click();
    await expect(connectionRows.getByRole("rowheader")).toHaveCount(connectionPage.items.length);await expect(connectionsView.getByRole("alert")).toHaveCount(0);await expect(connectionsView.getByRole("button",{name:"Refresh connections",exact:true})).toBeEnabled();
  }
  report.checks.push("connections-impossible-source-and-management-dates-clear-and-recover");
  await page.getByLabel("Connections per page",{exact:true}).selectOption("25");await expect(page).toHaveURL(/limit=25/);await expect(connectionRows.getByRole("rowheader")).toHaveCount(Math.min(25,connectionPage.items.length));
  await page.goBack();await expect(page.getByLabel("Connections per page",{exact:true})).toHaveValue("50");
  await page.setViewportSize({width:375,height:812});assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),"Connection page overflow");
  assert.ok(await connectionRows.evaluate(element=>element.scrollWidth>element.clientWidth),"Narrow connection table must scroll instead of splitting identifiers");
  await expect(connectionsView.getByRole("navigation",{name:"Connection pagination",exact:true})).toContainText(`1–${connectionPage.items.length} / ${connectionPage.total}`);
  await page.screenshot({path:path.join(evidence,"connections-mobile.png"),fullPage:true});await auditAccessibility(page,"connections-mobile");
  if(accessibility){
    for(let column=2;column<=7;column++){
      const selector=`.connection-rows tbody tr:first-child td:nth-child(${column})`,cell=page.locator(selector);
      await cell.scrollIntoViewIfNeeded();
      assert.ok(await cell.evaluate(element=>{
        const cell=element.getBoundingClientRect(),region=element.closest('[role="region"]').getBoundingClientRect();
        return cell.left>=Math.max(0,region.left)-1&&cell.right<=Math.min(innerWidth,region.right)+1&&cell.top>=0&&cell.bottom<=innerHeight;
      }),`Connection column ${column} must be fully visible for contrast testing`);
      const result=await auditAccessibility(page,`connections-mobile-visible-column-${column}`,selector);
      assert.equal(result.violations.length,0);assert.equal(result.incomplete.length,0);
      assert.ok(result.passedRules.some(rule=>rule.id==="color-contrast"&&rule.nodeCount>0),"Visible cell must actually receive a contrast check");
      await page.screenshot({path:path.join(evidence,`connections-mobile-visible-column-${column}.png`)});
    }
    await connectionRows.evaluate(element=>{element.scrollLeft=0;});
    report.checks.push("connections-mobile-all-numeric-columns-visible-contrast");
  }
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  const chineseConnections=page.getByRole("region",{name:"节点连接",exact:true}),chineseRows=chineseConnections.getByRole("region",{name:"连接数据行",exact:true});
  await expect(chineseConnections).toContainText("节点报告总数");await expect(chineseRows).toBeVisible();await expect(chineseConnections.getByRole("navigation",{name:"连接分页",exact:true})).toBeVisible();await expect(page.getByLabel("每页连接数",{exact:true})).toBeVisible();
  await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1487,height:1058});await page.screenshot({path:path.join(evidence,"connections-desktop.png"),fullPage:true});await auditAccessibility(page,"connections-desktop");
  await page.getByLabel("Connections per page",{exact:true}).selectOption("25");
  const selectedCID=String(connectionPage.items[0].cid),detailEndpoint=`${connectionAPI}/${selectedCID}`;
  let subscriptionRequests=0;const observeSubscriptions=request=>{if(new URL(request.url()).pathname===`${detailEndpoint}/subscriptions`)subscriptionRequests++;};page.on("request",observeSubscriptions);
  await connectionRows.getByRole("link",{name:selectedCID,exact:true}).click();
  const detailView=page.getByRole("region",{name:"Connection detail",exact:true}),detailRefresh=detailView.getByRole("button",{name:"Refresh connection",exact:true});
  await expect(detailView.getByRole("heading",{name:`Connection: ${selectedCID}`,exact:true})).toBeVisible();await expect(detailView.locator("dd")).toHaveCount(6);await expect(page).toHaveURL(/limit=25/);
  const subscriptionsView=page.getByRole("region",{name:"Connection subscriptions",exact:true}),loadSubscriptions=subscriptionsView.getByRole("button",{name:"Load subscriptions",exact:true});
  assert.equal(subscriptionRequests,0);await expect(subscriptionsView).toContainText("Load explicitly");await loadSubscriptions.click();await expect(subscriptionsView.getByRole("alert")).toHaveCount(0);assert.equal(subscriptionRequests,1);
  const realSubscriptions=await (await api(`${detailEndpoint}/subscriptions`)).json();await expect(subscriptionsView.getByRole("navigation",{name:"Subscription pages",exact:true})).toContainText(`/ ${realSubscriptions.items.length}`);
  const syntheticSubscriptions={...realSubscriptions,items:[{sid:"001",subject:"orders.*",messages:0},{sid:"02",subject:"orders.*",queue:"workers",messages:9007199254740992,maximum:10}]},syntheticSubscriptionJSON=JSON.stringify(syntheticSubscriptions).replace('"messages":9007199254740992','"messages":9007199254740993');
  await page.route(`**${detailEndpoint}/subscriptions`,route=>route.fulfill({status:200,contentType:"application/json",body:syntheticSubscriptionJSON}),{times:1});await subscriptionsView.getByRole("button",{name:"Refresh subscriptions",exact:true}).click();
  await expect(subscriptionsView.getByRole("rowheader")).toHaveText(["001","02"]);await expect(subscriptionsView).toContainText("9007199254740993");
  const subscriptionFilter=subscriptionsView.getByLabel("Filter this complete observation",{exact:true});await subscriptionFilter.fill("workers");await expect(subscriptionsView.getByRole("rowheader")).toHaveText("02");await subscriptionFilter.fill("");
  await page.route(`**${detailEndpoint}/subscriptions`,route=>route.fulfill({status:422,contentType:"application/json",body:'{"error":{"code":"subscription_limit_exceeded"}}'}),{times:1});await subscriptionsView.getByRole("button",{name:"Refresh subscriptions",exact:true}).click();await expect(subscriptionsView.getByRole("alert")).toContainText("More than 1,000");await expect(subscriptionsView.getByRole("table")).toHaveCount(0);
  await subscriptionsView.getByRole("button",{name:"Load subscriptions",exact:true}).click();await expect(subscriptionsView.getByRole("alert")).toHaveCount(0);
  const detailTime=detailView.locator("time").last(),initialDetailTime=await detailTime.getAttribute("datetime");
  // Exact connection presence can change independently while this page is
  // exercising subscriptions. Use an explicit read for deterministic live
  // evidence; completion scheduling/backoff is covered by model tests.
  await detailRefresh.click();await expect(detailTime).not.toHaveAttribute("datetime",initialDetailTime);
  for(const [status,code,message] of [[503,"connections_unavailable","absence is not established"],[404,"connection_not_found","disappearance reason is unknown"],[404,"not_found","Node not found"],[403,"forbidden","read denied"],[409,"node_identity_ambiguous","identity is ambiguous"]]){
    const before=await detailTime.getAttribute("datetime");
    await page.route(`**${detailEndpoint}`,route=>route.fulfill({status,contentType:"application/json",body:JSON.stringify({error:{code}})}),{times:1});
    await detailRefresh.click();await expect(detailView.getByRole("alert")).toContainText(message);
    if(status===503){await expect(detailTime).toHaveAttribute("datetime",before);await expect(detailView).toContainText("Historical connection evidence");}
    else{await expect(detailView.locator("dd")).toHaveCount(0);await expect(detailView.locator("time")).toHaveCount(0);}
    await detailRefresh.click();await expect(detailView.getByRole("alert")).toHaveCount(0);await expect(detailView.locator("dd")).toHaveCount(6);await expect(detailRefresh).toBeEnabled();
  }
  await page.route(`**${detailEndpoint}`,route=>route.fulfill({status:200,contentType:"application/json",body:'{"'}),{times:1});await detailRefresh.click();await expect(detailView.getByRole("alert")).toContainText("Invalid connection response");await expect(detailView.locator("time")).toHaveCount(0);
  await detailRefresh.click();await expect(detailView.locator("dd")).toHaveCount(6);await expect(detailRefresh).toBeEnabled();
  await page.screenshot({path:path.join(evidence,"connection-detail-desktop.png"),fullPage:true});await auditAccessibility(page,"connection-detail-desktop");
  await page.setViewportSize({width:375,height:812});await page.screenshot({path:path.join(evidence,"connection-detail-mobile.png"),fullPage:true});
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),"Connection detail page overflow");await auditAccessibility(page,"connection-detail-mobile");
  await page.getByRole("button",{name:"简体中文",exact:true}).click();const chineseDetail=page.getByRole("region",{name:"连接详情",exact:true});await expect(chineseDetail).toContainText("身份由节点 ID 与 CID 共同确定");await expect(chineseDetail.getByRole("button",{name:"刷新连接详情",exact:true})).toBeVisible();
  await expect(page.getByRole("region",{name:"连接订阅",exact:true})).toContainText("仅显式加载");await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1487,height:1058});
  await detailView.getByRole("link",{name:"Back to connections",exact:true}).click();await expect(page.getByLabel("Connections per page",{exact:true})).toHaveValue("25");
  await page.goBack();await expect(detailView.locator("dd")).toHaveCount(6);await page.goForward();await expect(page.getByLabel("Connections per page",{exact:true})).toHaveValue("25");
  page.off("request",observeSubscriptions);
  report.checks.push("connection-subscriptions-explicit-bounded-real-read-complete-local-filter-limit-clear-recovery-bilingual-responsive");
  report.checks.push("connection-detail-real-entry-explicit-read-return-page-size-browser-history-bilingual-responsive","connection-detail-failure-history-node-vs-cid-missing-denied-ambiguous-invalid-clear-recovery");
  await connectionsView.getByRole("link",{name:"Back to node",exact:true}).click();await expect(page.getByRole("heading",{name:`Node: ${nodeID}`,exact:true})).toBeVisible();
  report.checks.push("connections-real-node-entry-total-auto-refresh-history-query-back-bilingual-responsive","connections-injected-unavailable-history-ambiguous-missing-denied-clear-recover");
  await navigate("Audit");
  await page.getByLabel("Resource name",{exact:true}).fill("live_candidate");
  await page.getByLabel("Phase",{exact:true}).selectOption("outcome");
  await page.getByLabel("From (inclusive)",{exact:true}).fill("2000-01-01T00:00:00Z");
  await page.getByLabel("Until (exclusive)",{exact:true}).fill("2099-01-01T00:00:00Z");
  await page.getByRole("button",{name:"Search audit",exact:true}).click();
  const auditWindow=page.getByRole("region",{name:"Audit window",exact:true});
  await expect(auditWindow).toContainText("live_candidate");
  await auditWindow.locator("summary").first().click();
  await expect(auditWindow.locator("pre").first()).toContainText('"phase":"outcome"');
  await page.getByRole("button",{name:"Prepare filtered export",exact:true}).click();
  const downloadPending=page.waitForEvent("download");
  await page.getByRole("link",{name:"Download audit JSON",exact:true}).click();
  const auditDownload=await downloadPending;await auditDownload.saveAs(path.join(evidence,"audit-export.json"));
  const exportedAudit=JSON.parse(await readFile(path.join(evidence,"audit-export.json"),"utf8"));
  assert.equal(exportedAudit.filter.resource,"live_candidate");assert.equal(exportedAudit.filter.phase,"outcome");
  assert.ok(exportedAudit.windows.length>0);assert.equal(exportedAudit.windows.at(-1).page.nextBefore,null);
  assert.ok(exportedAudit.windows.flatMap(window=>window.page.items).every(event=>event.resourceName==="live_candidate"&&event.phase==="outcome"));
  report.checks.push("auditor-filtered-json-export-real-records-explicit-coverage");
  await page.getByLabel("Resource name",{exact:true}).fill("not-a-real-resource");
  await page.getByRole("button",{name:"Search audit",exact:true}).click();
  await expect(page.getByText("No matching records in this window.",{exact:true})).toBeVisible();
  await page.goBack();await expect(page.getByLabel("Resource name",{exact:true})).toHaveValue("live_candidate");
  await expect(page.getByLabel("From (inclusive)",{exact:true})).toHaveValue("2000-01-01T00:00:00Z");
  await expect(auditWindow).toContainText("live_candidate");
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  const chineseAudit=page.getByRole("region",{name:"审计事件",exact:true});
  await expect(chineseAudit.getByRole("region",{name:"审计导出",exact:true})).toBeVisible();await expect(chineseAudit.getByRole("region",{name:"审计窗口",exact:true})).toContainText("live_candidate");
  await page.getByRole("button",{name:"English",exact:true}).click();await expect(auditWindow).toContainText("live_candidate");
  report.checks.push("auditor-audit-page-filter-event-detail-empty-window-history");
  await navigate("Queue list");
  await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);
  await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  await navigate("Overview");const historyRegion=page.getByRole("region",{name:"Metric history",exact:true});await expect(historyRegion.getByRole("img")).toBeVisible();await expect(historyRegion).toContainText("gap");
  await historyRegion.getByRole("combobox",{name:/^Time window/}).selectOption("1h");await expect.poll(()=>report.prometheusQueries.some(item=>item.query==="rjs_jetstream_storage_bytes"&&item.step==="60")).toBe(true);
  assert.ok(report.prometheusQueries.every(item=>/^rjs_(jetstream_(storage|memory)_bytes|uptime_seconds|queue_(messages|bytes))/.test(item.query)),JSON.stringify(report.prometheusQueries));
  report.checks.push("prometheus-history-fixed-query-real-range-gap-window-no-browser-promql");
  await navigate("Operational alerts");const alertsRegion=page.getByRole("region",{name:"Operational alerts",exact:true});await expect(alertsRegion).toContainText("firing");await expect(alertsRegion).toContainText("rjs_queue_messages > 100000");await expect(alertsRegion).toContainText("900");await expect(alertsRegion.getByRole("alert")).toContainText("Incomplete alert coverage");
  await expect(alertsRegion.getByRole("link",{name:"Open Prometheus alerts",exact:true})).toHaveAttribute("href",`http://127.0.0.1:${prometheusPort}/alerts`);alertFiring=false;await alertsRegion.getByRole("button",{name:"Refresh alerts",exact:true}).click();await expect(alertsRegion).toContainText("recovered");await expect(alertsRegion).toContainText("does not send notifications");report.checks.push("prometheus-alerts-fixed-rules-partial-coverage-threshold-duration-firing-observed-recovery-explicit-external-link-no-notification-claim");
  await navigate("Diagnostics");
  const diagnosticsRegion=page.getByRole("region",{name:"Diagnostics",exact:true});
  await expect(diagnosticsRegion).toContainText("metadata-only ZIP");
  await diagnosticsRegion.getByRole("button",{name:"Create metadata bundle",exact:true}).click();
  const diagnosticJob=diagnosticsRegion.getByRole("region",{name:"Current diagnostic job",exact:true});
  await expect(diagnosticJob).toContainText("ready",{timeout:35000});
  await expect(diagnosticJob.getByRole("table")).toContainText("management-build");
  await diagnosticJob.getByRole("button",{name:"Prepare download",exact:true}).click();
  const diagnosticDownloadPending=page.waitForEvent("download");
  await diagnosticJob.getByRole("link",{name:"Save diagnostic ZIP",exact:true}).click();
  const diagnosticDownload=await diagnosticDownloadPending,diagnosticPath=path.join(evidence,"diagnostics.zip");await diagnosticDownload.saveAs(diagnosticPath);
  const diagnosticBytes=await readFile(diagnosticPath);assert.deepEqual([...diagnosticBytes.subarray(0,4)],[80,75,3,4]);assert.ok(diagnosticBytes.length>0&&diagnosticBytes.length<=8<<20);
  await page.getByRole("button",{name:"简体中文",exact:true}).click();await expect(page.getByRole("region",{name:"诊断包",exact:true})).toContainText("仅元数据");await page.getByRole("button",{name:"English",exact:true}).click();
  await auditAccessibility(page,"diagnostics-desktop");
  report.checks.push("diagnostics-operator-explicit-create-real-ready-manifest-bounded-zip-bilingual-a11y");
  await navigate("Consumer list");await page.getByRole("button",{name:"Collect new generation",exact:true}).click();
  await expect(page.getByRole("button",{name:"Collect new generation",exact:true})).toBeEnabled({timeout:35000});
  const globalConsumerAPI=await api("/api/v1/consumers");
  if(globalConsumerAPI.status!==200)throw new Error(`global Consumer collection status ${globalConsumerAPI.status}: ${JSON.stringify(await globalConsumerAPI.json())}`);
  const globalConsumerBody=await globalConsumerAPI.clone().json();
  if(globalConsumerBody.total!==3)throw new Error(`global Consumer collection total: ${JSON.stringify(globalConsumerBody)}`);
  const globalConsumerRegion=page.getByRole("region",{name:"Consumer list",exact:true});
  await expect(globalConsumerRegion.getByRole("rowheader")).toHaveCount(3,{timeout:35000});
  await expect(page.getByText("Filtered total: 3",{exact:false})).toBeVisible();
  await expect(globalConsumerRegion.getByRole("columnheader")).toHaveText(["Consumer","Stream","Queue","Mode","State","Ownership","Pending","Ack pending"]);
  await page.screenshot({path:path.join(evidence,"global-consumers-desktop.png"),fullPage:true});await auditAccessibility(page,"global-consumers-desktop");
  await page.getByLabel("Queue, Stream, Consumer or durable contains",{exact:true}).fill(primary.name);await page.getByRole("button",{name:"Filter",exact:true}).click();
  await expect(globalConsumerRegion.getByRole("rowheader")).toHaveCount(1);await expect(globalConsumerRegion.getByRole("rowheader")).toHaveText(primary.name);
  report.checks.push("global-consumer-operator-collects-complete-generation-then-server-filters-list");
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();
  const draft=page.getByLabel("Queue document (JSON)",{exact:true});await expect(draft).not.toHaveValue("");
  const edited=JSON.parse(await draft.inputValue());edited.spec.retention.maxMessages=200;
  await draft.fill(JSON.stringify(edited));await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
  const afterPreview=await (await api("/api/v1/queues/live_candidate")).json();assert.equal(afterPreview.document.spec.retention.maxMessages,100);
  const declarationRead=await api("/api/v1/queues/live_candidate");
  const capabilityRevision=capabilitiesResponse.headers.get("etag");assert.match(capabilityRevision,/^"rjs-capabilities-v1:[0-9a-f]{64}"$/);
  const wrongCapabilityRevision=`"rjs-capabilities-v1:${"0".repeat(64)}"`;assert.notEqual(wrongCapabilityRevision,capabilityRevision);
  for(const [method,suffix] of [["PUT",""],["DELETE",""],["POST","/preview"],["GET","/delete-preview"]]){
    const rejected=await api(`/api/v1/queues/live_candidate${suffix}`,{method,headers:{"X-RJS-If-Capabilities-Match":wrongCapabilityRevision,"If-Match":declarationRead.headers.get("etag"),"X-RJS-Confirm-Queue":"live_candidate"},...(method==="PUT"||method==="POST"?{body:JSON.stringify(edited)}:{})});
    assert.equal(rejected.status,412);assert.equal((await rejected.json()).error.code,"capabilities_changed");
  }
  assert.equal((await api("/api/v1/queues/live_candidate")).headers.get("etag"),declarationRead.headers.get("etag"));
  report.checks.push("real-receiving-server-contract-precondition-rejects-preview-put-delete-before-resource-change");
  const deletionURL="/api/v1/queues/live_candidate/delete-preview";
  const deletionResponse=await api(deletionURL,{headers:{"If-Match":declarationRead.headers.get("etag")}});
  assert.equal(deletionResponse.status,200);assert.equal(deletionResponse.headers.get("cache-control"),"no-store");
  const deletionPreview=await deletionResponse.json();
  assert.equal(deletionPreview.queue,"live_candidate");assert.equal(deletionPreview.stream,collection.stream);
  assert.equal(deletionPreview.base_revision,declarationRead.headers.get("etag"));
  assert.equal(deletionResponse.headers.get("etag"),deletionPreview.base_revision);
  assert.equal(deletionPreview.stream_present,true);assert.equal(deletionPreview.ownership,"matching");
  assert.equal(typeof deletionPreview.messages,"number");assert.ok(deletionPreview.consumers>=3);
  assert.equal(deletionPreview.requires_force,deletionPreview.messages>0);
  assert.equal(deletionPreview.blocked,deletionPreview.requires_force);assert.ok(Number.isFinite(Date.parse(deletionPreview.observed_at)));
  assert.equal((await api(deletionURL,{headers:{"If-Match":deletionPreview.base_revision,Authorization:`Bearer ${auditor}`}})).status,403);
  assert.equal((await api(deletionURL,{headers:{"If-Match":'"18446744073709551615"'}})).status,409);
  assert.equal((await api(deletionURL)).status,428);
  const deletionAfter=await api("/api/v1/queues/live_candidate");
  assert.equal(deletionAfter.headers.get("etag"),deletionPreview.base_revision);
  assert.deepEqual((await deletionAfter.json()).document,afterPreview.document);
  await writeFile(path.join(evidence,"delete-preview.json"),JSON.stringify(deletionPreview,null,2));
  report.checks.push("real-read-only-delete-preflight-ownership-counts-etag-auditor-denial-stale-conflict-no-delete");
  const declarationPreviewResponse=await api("/api/v1/queues/live_candidate/preview",{method:"POST",headers:{"If-Match":declarationRead.headers.get("etag")},body:JSON.stringify(edited)});
  assert.equal(declarationPreviewResponse.status,200);
  const declarationPreview=await declarationPreviewResponse.json();
  assert.equal(declarationPreview.base_revision,declarationRead.headers.get("etag"));
  assert.equal(declarationPreview.declaration_review.status,"available");
  assert.equal(declarationPreview.declaration_review.document.spec.retention.maxMessages,200);
  const declarationLimit=declarationPreview.declaration_review.diff.changes.find(change=>change.path==="spec.retention.maxMessages");
  assert.equal(declarationLimit.from,"100");assert.equal(declarationLimit.to,"200");
  report.checks.push("real-declaration-review-original-etag-normalized-target-and-semantic-limit-diff");
  await navigate("Queue list");await expect(page.getByRole("link",{name:"live_candidate",exact:true})).toBeVisible();
  await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await expect(page.getByRole("heading",{name:`Observed Stream: ${collection.stream}`,exact:true})).toBeVisible();
  await page.goBack();
  await page.goBack();await expect(draft).toHaveValue(JSON.stringify(edited));
  report.checks.push("queue-page-refresh-retains-consumer-query-and-existing-editor-draft");
  const remoteDocument=structuredClone(afterPreview.document);remoteDocument.metadata.labels={...remoteDocument.metadata.labels,concurrent:"preserve"};
  const remoteApply=await api("/api/v1/queues/live_candidate",{method:"PUT",headers:{"If-Match":etag},body:JSON.stringify(remoteDocument)});assert.equal(remoteApply.status,200);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("alert")).toContainText("Original declaration revision has changed");
  await page.getByRole("button",{name:"Read latest for comparison",exact:true}).click();
  const merged=page.getByLabel("Reviewed merged document (JSON)",{exact:true});await expect(merged).toHaveValue(JSON.stringify(edited));
  const mergeButton=page.getByRole("button",{name:"Use merged draft and new base",exact:true});await expect(mergeButton).toBeDisabled();
  edited.metadata.labels={...edited.metadata.labels,concurrent:"preserve"};await merged.fill(JSON.stringify(edited));
  await page.getByRole("checkbox",{name:"I reviewed all three versions and approve this merged draft.",exact:true}).check();await mergeButton.click();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
  const persistedAfterMerge=await (await api("/api/v1/queues/live_candidate")).json();assert.equal(persistedAfterMerge.document.spec.retention.maxMessages,100);assert.equal(persistedAfterMerge.document.metadata.labels.concurrent,"preserve");
  report.checks.push("real-concurrent-declaration-three-way-merge-confirmation-fresh-preview-no-write");
  await draft.fill("{broken");await expect(page.getByRole("button",{name:"Preview changes",exact:true})).toBeDisabled();
  await navigate("Access and settings");
  await page.getByRole("button",{name:"Clear this session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
  // Dynamic accessibility behavior (L-08): a dialog must pull focus into
  // itself on open and hand it back to the triggering control on close.
  await page.getByRole("button",{name:"Clear this session",exact:true}).click();
  const focusDialog=page.getByRole("alertdialog");
  await focusDialog.waitFor();
  assert.equal(await focusDialog.evaluate(element=>element.contains(document.activeElement)),true,"dialog open must move focus inside the dialog");
  await focusDialog.getByRole("button",{name:"Cancel",exact:true}).click();
  await expect(focusDialog).toHaveCount(0);
  assert.equal(await page.getByRole("button",{name:"Clear this session",exact:true}).evaluate(element=>element===document.activeElement),true,"dialog close must restore focus to the trigger");
  await page.goBack();await expect(draft).toHaveValue("{broken");
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();await expect(draft).toHaveValue("{broken");
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Clear session",exact:true}).click();await expect(page.getByLabel("Username",{exact:true})).toBeVisible();
  report.checks.push("operator-draft-preview-no-write-route-retention-invalid-json-confirmed-clear");
  report.checks.push("dialog-focus-moves-inside-on-open-and-restores-to-trigger-on-close");
  let initialReadCalls=0;
  const failInitialRead=route=>{initialReadCalls++;return route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable","message":"Injected initial read failure"}}'});};
  await page.route("**/api/v1/queues/live_candidate",failInitialRead);
  const putsBeforeInitialRetry=forwardedPuts;
  await page.goto(origin+"/admin/queues/by-name/live_candidate/edit");
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  const retryInitial=page.getByRole("button",{name:"Retry canonical read",exact:true});
  await expect(retryInitial).toBeVisible();await expect(draft).toHaveCount(0);assert.equal(initialReadCalls,1);
  await retryInitial.click();await expect(retryInitial).toBeVisible();assert.equal(initialReadCalls,2);
  await page.screenshot({path:path.join(evidence,"initial-editor-read-failure.png"),fullPage:true});
  await page.unroute("**/api/v1/queues/live_candidate",failInitialRead);
  await retryInitial.click();
  await expect(draft).not.toHaveValue("");
  await expect(retryInitial).toHaveCount(0);assert.equal(forwardedPuts,putsBeforeInitialRetry);
  report.checks.push("initial-editor-read-failure-explicit-read-only-retry-recovery-no-write");
  await page.getByText("Structured configuration fields",{exact:true}).click();
  const routingOriginal=await draft.inputValue(),routingPuts=forwardedPuts;
  const routingMode=page.getByLabel("Routing mode",{exact:true});
  const unsupportedLabels=schemaText.replace('"additionalProperties": {"type": "string"}','"additionalProperties": {"type": "object"}');
  assert.notEqual(unsupportedLabels,schemaText);
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:200,headers:{ETag:capabilities.queue.schema.etag},contentType:"application/schema+json",body:unsupportedLabels}),{times:1});
  await page.getByRole("button",{name:"Reload form schema",exact:true}).click();
  await expect(page.getByRole("status").filter({hasText:"Form schema unavailable or changed."})).toBeVisible();
  await expect(routingMode).toHaveCount(0);await expect(page.getByRole("button",{name:"Add label",exact:true})).toHaveCount(0);
  assert.equal(await draft.inputValue(),routingOriginal);assert.equal(forwardedPuts,routingPuts);
  await page.getByRole("button",{name:"Reload form schema",exact:true}).click();await expect(routingMode).toBeVisible();
  await expect(page.getByText("Schema label values: text.",{exact:false})).toBeVisible();
  await expect(page.getByText("Schema routing: choose one collection.",{exact:false})).toBeVisible();
  report.checks.push("unsupported-schema-label-type-disables-collection-controls-preserves-draft-reload-no-write");
  const mixedRouting=JSON.parse(routingOriginal);mixedRouting.spec.bindings=[{exchange:"other",type:"fanout"}];
  await draft.fill(JSON.stringify(mixedRouting));await expect(routingMode).toHaveValue("mixed");
  await routingMode.selectOption("subjects");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  const selectedRouting=JSON.parse(await draft.inputValue());
  assert.deepEqual(selectedRouting.spec.subjects,mixedRouting.spec.subjects);assert.equal(selectedRouting.spec.bindings,undefined);
  await draft.fill(routingOriginal);
  await routingMode.selectOption("bindings");await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
  await expect(routingMode).toHaveValue("subjects");assert.equal(await draft.inputValue(),routingOriginal);
  await routingMode.selectOption("bindings");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  await page.getByRole("button",{name:"Add Binding",exact:true}).click();
  await page.getByLabel("Exchange 1",{exact:true}).fill("events");
  await page.getByLabel("Binding type 1",{exact:true}).selectOption("topic");
  await expect(page.getByText("Schema minimum key count: 1",{exact:true})).toBeVisible();
  await page.getByRole("button",{name:"Add key to Binding 1",exact:true}).click();
  await page.getByLabel("Routing key 1.1",{exact:true}).fill("orders.#");
  const beforeFanout=await draft.inputValue();
  await page.getByLabel("Binding type 1",{exact:true}).selectOption("fanout");await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
  assert.equal(await draft.inputValue(),beforeFanout);
  await page.getByLabel("Binding type 1",{exact:true}).selectOption("fanout");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  assert.equal(JSON.parse(await draft.inputValue()).spec.bindings[0].keys,undefined);
  await expect(page.getByText("Schema key count: 0.",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Add key to Binding 1",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
  await page.screenshot({path:path.join(evidence,"structured-queue-routing.png"),fullPage:true});
  await draft.fill(routingOriginal);await expect(routingMode).toHaveValue("subjects");
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toHaveCount(0);
  assert.equal(forwardedPuts,routingPuts);
  report.checks.push("routing-mode-fanout-confirm-cancel-real-preview-shared-json-no-put");
  const labelsOriginal=await draft.inputValue(),labelsPuts=forwardedPuts;
  const addLabel=page.getByRole("button",{name:"Add label",exact:true});
  await addLabel.click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();assert.equal(await draft.inputValue(),labelsOriginal);
  await addLabel.click();await page.getByRole("alertdialog").getByRole("textbox").fill("owner");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  const ownerValue=page.getByLabel("Label value: owner",{exact:true});await expect(ownerValue).toBeFocused();
  await ownerValue.fill("team\nsecond line");
  const afterAddLabel=await draft.inputValue();
  await addLabel.click();await page.getByRole("alertdialog").getByRole("textbox").fill("owner");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  await expect(page.getByRole("alert").filter({hasText:"That label key already exists"})).toBeVisible();assert.equal(await draft.inputValue(),afterAddLabel);
  await page.getByRole("button",{name:"Rename label: owner",exact:true}).click();await page.getByRole("alertdialog").getByRole("textbox").fill("concurrent");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  assert.equal(await draft.inputValue(),afterAddLabel);
  await page.getByRole("button",{name:"Rename label: owner",exact:true}).click();await page.getByRole("alertdialog").getByRole("textbox").fill("Owner");await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  await expect(page.getByLabel("Label value: Owner",{exact:true})).toBeFocused();
  assert.equal(JSON.parse(await draft.inputValue()).metadata.labels.Owner,"team\nsecond line");
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
  await page.screenshot({path:path.join(evidence,"structured-queue-labels.png"),fullPage:true});
  await page.getByRole("button",{name:"Remove label: Owner",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
  await expect(page.getByLabel("Label value: Owner",{exact:true})).toHaveValue("team\nsecond line");
  await page.getByRole("button",{name:"Remove label: Owner",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  await expect(addLabel).toBeFocused();await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toHaveCount(0);
  assert.deepEqual(JSON.parse(await draft.inputValue()),JSON.parse(labelsOriginal));assert.equal(forwardedPuts,labelsPuts);
  report.checks.push("labels-exact-multiline-duplicate-rename-refused-confirmed-delete-focus-real-preview-no-put");
  const retainedLimit=page.getByLabel("Maximum retained messages",{exact:true});
  await expect(page.getByText("Control types, choices and bounds come from the verified server schema.",{exact:true})).toBeVisible();
  const formRaw=await draft.inputValue(),formPuts=forwardedPuts;
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await page.getByRole("button",{name:"Reload form schema",exact:true}).click();
  await expect(page.getByRole("status").filter({hasText:"Form schema unavailable or changed."})).toBeVisible();
  await expect(retainedLimit).toHaveCount(0);await expect(draft).toHaveValue(formRaw);assert.equal(forwardedPuts,formPuts);
  await page.getByRole("button",{name:"Reload form schema",exact:true}).click();
  await expect(retainedLimit).toBeVisible();
  await expect(page.getByText("Maximum: 9223372036854775807",{exact:false}).first()).toBeVisible();
  await expect(draft).toHaveValue(formRaw);assert.equal(forwardedPuts,formPuts);
  report.checks.push("schema-driven-form-exact-bound-read-failure-disables-controls-explicit-reload-preserves-json-no-write");
  const structuredOriginal=JSON.parse(await draft.inputValue()),structuredPuts=forwardedPuts;
  await retainedLimit.fill("9223372036854775808");
  assert.ok((await draft.inputValue()).includes('"maxMessages":9223372036854775808'));
  const invalidPreviewResponse=page.waitForResponse(response=>response.url().endsWith("/api/v1/queues/live_candidate/preview")&&response.request().method()==="POST");
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  const invalidPreview=await invalidPreviewResponse;assert.equal(invalidPreview.status(),400);
  const invalidPreviewBody=await invalidPreview.json();assert.equal(invalidPreviewBody.error.code,"invalid_queue");
  await expect(page.getByRole("alert").filter({hasText:"Operation failed."})).toBeVisible();
  const validationDiagnostic=page.getByRole("region",{name:"Server preview validation",exact:true});
  await expect(validationDiagnostic).toBeVisible();
  assert.equal(await validationDiagnostic.locator("pre").textContent(),invalidPreviewBody.error.message);
  await retainedLimit.fill("-1");
  const semanticResponse=page.waitForResponse(response=>response.url().endsWith("/api/v1/queues/live_candidate/preview")&&response.request().method()==="POST");
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  const semanticBody=await (await semanticResponse).json();
  assert.equal(semanticBody.error.issues_version,"rjs.queue-validation.v1");
  assert.equal(semanticBody.error.issues[0].path,"/spec/retention");
  await expect(page.getByRole("list",{name:"Server field diagnostics",exact:true})).toContainText("/spec/retention");
  assert.equal(forwardedPuts,structuredPuts);
  report.checks.push("real-semantic-preview-field-pointer-rendered-no-write");
  await page.getByRole("button",{name:"Locate field or JSON: /spec/retention",exact:true}).click();
  await expect(draft).toBeFocused();
  const beforeFocusDraft=await draft.inputValue(),focusDocument=JSON.parse(beforeFocusDraft);
  focusDocument.spec.subjects=["z.events","a..bad"];
  await draft.fill(JSON.stringify(focusDocument));
  await expect(page.getByRole("list",{name:"Server field diagnostics",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  const subjectDiagnostic=page.getByRole("button",{name:"Locate field or JSON: /spec/subjects/1",exact:true});
  await expect(subjectDiagnostic).toBeVisible();
  await page.getByText("Structured configuration fields",{exact:true}).click();
  const focusRaw=await draft.inputValue();
  await subjectDiagnostic.focus();await page.keyboard.press("Enter");
  await expect(page.getByLabel("Subject 2",{exact:true})).toBeFocused();
  await expect(page.getByLabel("Subject 2",{exact:true})).toHaveValue("a..bad");
  assert.equal(await draft.inputValue(),focusRaw);assert.equal(forwardedPuts,structuredPuts);
  await page.screenshot({path:path.join(evidence,"queue-diagnostic-field-focus.png"),fullPage:true});
  await draft.fill(beforeFocusDraft);
  await expect(subjectDiagnostic).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(validationDiagnostic).toBeVisible();
  report.checks.push("diagnostic-keyboard-focus-original-array-index-opens-details-group-json-fallback-edit-clears-stale-no-write");
  await page.screenshot({path:path.join(evidence,"queue-preview-validation.png"),fullPage:true});
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
  await retainedLimit.fill("9007199254740993");
  await expect(validationDiagnostic).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toBeDisabled();
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();
  await retainedLimit.fill("200");
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
  const structuredEdited=JSON.parse(await draft.inputValue());
  structuredOriginal.spec.retention.maxMessages=200;assert.deepEqual(structuredEdited,structuredOriginal);
  assert.equal(forwardedPuts,structuredPuts);
  await page.screenshot({path:path.join(evidence,"structured-queue-fields.png"),fullPage:true});
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  report.checks.push("structured-fields-exact-int64-server-validation-shared-draft-invalidates-review-no-put");
  const operationReview=page.getByRole("region",{name:"Observed-to-desired resource changes",exact:true});
  await expect(operationReview).toBeVisible();
  const declarationReviewPanel=page.getByRole("region",{name:"Declaration change review",exact:true});
  const declarationDiffRow=declarationReviewPanel.getByRole("row").filter({has:page.getByRole("rowheader",{name:"spec.retention.maxMessages",exact:true})});
  await expect(declarationDiffRow).toHaveCount(1);
  await expect(declarationDiffRow.getByRole("cell").nth(0)).toHaveText("100");
  await expect(declarationDiffRow.getByRole("cell").nth(1)).toHaveText("200");
  const beforeNormalizedView=await draft.inputValue();
  await page.getByText("Normalized target Queue document",{exact:true}).click();
  await expect(declarationReviewPanel.locator("details").filter({has:page.getByText("Normalized target Queue document",{exact:true})}).locator("pre")).toContainText('"maxMessages":200');
  assert.equal(await draft.inputValue(),beforeNormalizedView);
  await page.getByText("Normalized target Queue document",{exact:true}).click();
  const maxMessagesRow=operationReview.getByRole("row").filter({has:page.getByRole("rowheader",{name:"maxMessages",exact:true})});
  await expect(maxMessagesRow).toHaveCount(1);
  await expect(maxMessagesRow.getByRole("cell").nth(0)).toHaveText("100");
  await expect(maxMessagesRow.getByRole("cell").nth(1)).toHaveText("200");
  await page.getByText("Generated plan for this preview",{exact:true}).click();
  await expect(page.locator("details").filter({has:page.getByText("Generated plan for this preview",{exact:true})})).toContainText('"maxMessages":200');
  await page.getByText("Generated plan for this preview",{exact:true}).click();
  await page.screenshot({path:path.join(evidence,"queue-preview-operations.png"),fullPage:true});
  report.checks.push("observed-to-desired-preview-resource-field-values-and-generated-plan");
  await page.route("**/api/v1/queues/live_candidate/preview",async route=>{
    const response=await route.fetch();const body=await response.json();body.result.operations=[{resource:"stream",name:"S",changes:null}];
    await route.fulfill({response,json:body});
  },{times:1});
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByText(/^Preview response is incomplete or inconsistent\./)).toBeVisible();
  await expect(operationReview).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
  await expect(page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(maxMessagesRow).toHaveCount(1);
  report.checks.push("malformed-preview-operation-rows-hide-confirmation-and-apply-until-fresh-preview");
  await page.route("**/api/v1/queues/live_candidate/preview",async route=>{
    const response=await route.fetch();const body=await response.json();body.declaration_review.document.metadata.name="wrong_queue";
    await route.fulfill({response,json:body});
  },{times:1});
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByText(/^Preview response is incomplete or inconsistent\./)).toBeVisible();
  await expect(declarationReviewPanel).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(declarationDiffRow).toHaveCount(1);
  report.checks.push("declaration-review-separated-values-normalized-view-retains-draft-mismatched-identity-blocks-confirmation");
  const applyButton=page.getByRole("button",{name:"Apply reviewed draft",exact:true});await expect(applyButton).toBeDisabled();
  const beforeCapabilityPuts=forwardedPuts,capabilityDraft=await draft.inputValue();
  const changedCapabilities={...capabilities,deployment:{profile:"cluster",source:"configuration"}};
  await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,headers:{ETag:capabilitiesResponse.headers.get("etag")},contentType:"application/json",body:JSON.stringify(changedCapabilities)}),{times:1});
  await navigate("Access and settings");await expect(page.getByRole("region",{name:"Server capabilities",exact:true})).toContainText("cluster");
  await page.goBack();await expect(page.getByRole("alert")).toContainText("No write request was sent");
  await expect(draft).toHaveValue(capabilityDraft);await expect(applyButton).toHaveCount(0);assert.equal(forwardedPuts,beforeCapabilityPuts);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();await expect(applyButton).toBeDisabled();
  report.checks.push("settings-capability-observation-invalidates-retained-editor-before-submit-navigation-no-write");
  await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,headers:{ETag:capabilitiesResponse.headers.get("etag")},contentType:"application/json",body:JSON.stringify(changedCapabilities)}),{times:1});
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByRole("alert")).toContainText("No write request was sent");
  assert.equal(forwardedPuts,beforeCapabilityPuts);await expect(draft).toHaveValue(capabilityDraft);await expect(applyButton).toHaveCount(0);
  await page.screenshot({path:path.join(evidence,"capability-change-before-apply.png"),fullPage:true});
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();await expect(applyButton).toBeDisabled();
  report.checks.push("capability-change-before-apply-invalidates-preview-confirmation-preserves-draft-no-put");
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable","message":"Injected schema read failure"}}'}),{times:1});
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByRole("alert")).toContainText("No write request was sent");
  assert.equal(forwardedPuts,beforeCapabilityPuts);await expect(draft).toHaveValue(capabilityDraft);await expect(applyButton).toHaveCount(0);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();await expect(applyButton).toBeDisabled();
  report.checks.push("schema-unavailable-before-apply-preserves-draft-clears-confirmation-no-put");
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  assert.equal((await (await api("/api/v1/queues/live_candidate")).json()).document.spec.retention.maxMessages,200);
  const firstAcceptedID=await page.locator("code").last().textContent();
  const editNext=page.getByRole("button",{name:"Read latest declaration to edit again",exact:true});
  const nextReadFailure=route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable","message":"Injected next edit read failure"}}'});
  const beforeNextReadPuts=forwardedPuts;
  await page.route("**/api/v1/queues/live_candidate",nextReadFailure);
  await editNext.click();await expect(page.getByRole("alert")).toContainText("Latest edit base unavailable");
  await expect(draft).toBeDisabled();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  assert.equal(await page.locator("code").last().textContent(),firstAcceptedID);
  await page.unroute("**/api/v1/queues/live_candidate",nextReadFailure);
  const concurrentNext=await api("/api/v1/queues/live_candidate"),concurrentNextBody=await concurrentNext.json();
  concurrentNextBody.document.spec.retention.maxMessages=225;
  const concurrentNextWrite=await api("/api/v1/queues/live_candidate",{method:"PUT",headers:{"If-Match":concurrentNext.headers.get("etag")},body:JSON.stringify(concurrentNextBody.document)});
  assert.equal(concurrentNextWrite.status,200);await concurrentNextWrite.arrayBuffer();
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await editNext.click();await expect(draft).toBeEnabled();
  const nextFormDetails=page.locator("details").filter({has:page.getByText("Structured configuration fields",{exact:true})});
  if(!await nextFormDetails.evaluate(element=>element.open))await page.getByText("Structured configuration fields",{exact:true}).click();
  await expect(page.getByRole("status").filter({hasText:"Form schema unavailable or changed."})).toBeVisible();
  assert.equal(JSON.parse(await draft.inputValue()).spec.retention.maxMessages,225);
  assert.equal(forwardedPuts,beforeNextReadPuts+1);
  await page.getByRole("button",{name:"Reload form schema",exact:true}).click();
  await expect(page.getByText("Control types, choices and bounds come from the verified server schema.",{exact:true})).toBeVisible();
  report.checks.push("next-edit-reloads-schema-failure-preserves-fresh-base-receipt-explicit-recovery-no-write");
  assert.equal(JSON.parse(await draft.inputValue()).spec.retention.maxMessages,225);
  await expect(applyButton).toHaveCount(0);assert.equal(forwardedPuts,beforeNextReadPuts+1);
  await page.getByText("Previous accepted requests (session memory)",{exact:true}).click();
  await expect(page.locator("details").filter({has:page.getByText("Previous accepted requests (session memory)",{exact:true})})).toContainText(firstAcceptedID);
  const secondDraft=JSON.parse(await draft.inputValue());secondDraft.spec.retention.maxMessages=250;
  await draft.fill(JSON.stringify(secondDraft));await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(applyButton).toBeDisabled();
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  assert.equal((await (await api("/api/v1/queues/live_candidate")).json()).document.spec.retention.maxMessages,250);
  assert.notEqual(await page.locator("code").last().textContent(),firstAcceptedID);
  await page.screenshot({path:path.join(evidence,"repeat-edit-accepted.png"),fullPage:true});
  report.checks.push("accepted-edit-fresh-base-failed-read-retains-receipt-concurrent-writer-fresh-preview-second-confirmed-apply");
  const acceptedDownloadPending=page.waitForEvent("download");
  await page.getByRole("link",{name:"Download editor evidence JSON",exact:true}).click();
  const acceptedDownload=await acceptedDownloadPending,acceptedPath=path.join(evidence,"accepted-editor-evidence.json");
  await acceptedDownload.saveAs(acceptedPath);
  const acceptedEvidence=JSON.parse(await readFile(acceptedPath,"utf8"));
  assert.equal(acceptedEvidence.phase,"accepted");assert.equal(acceptedEvidence.acceptedOperations.length,1);
  assert.equal(acceptedEvidence.acceptedOperations[0].requestId,firstAcceptedID);
  assert.equal(acceptedEvidence.document.spec.retention.maxMessages,250);
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();
  const testSessionExpiry=process.env.RJS_TEST_SESSION_EXPIRY==="1";
  if(testSessionExpiry){
    // UI lifecycle fixture only: the server still verifies its real static token.
    // Inject expiry into this one session response, not resource/write responses.
    // Install the clock before the next document loads so captured Date.now
    // references in the application also use the controlled browser clock.
    await page.clock.install();
    await page.goto(origin+"/admin/queues/by-name/live_candidate/edit");
    await page.route("**/api/v1/session",async route=>{
      const response=await route.fetch();assert.equal(response.status(),200);
      const body=await response.json();body.expires_at=new Date(Date.now()+60000).toISOString();
      await route.fulfill({response,json:body});
    },{times:1});
  }
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();await expect(draft).not.toHaveValue("");
  const unknownDraft=JSON.parse(await draft.inputValue());unknownDraft.spec.retention.maxMessages=300;
  await draft.fill(JSON.stringify(unknownDraft));await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();const putsBeforeUnknown=forwardedPuts;dropNextApplyResponse=true;await applyButton.click();
  await expect(page.getByText(/Write outcome unknown\./)).toBeVisible();await expect(draft).toBeDisabled();await expect(applyButton).toHaveCount(0);
  await auditAccessibility(page,"unknown-write");
  await expect(page.getByRole("region",{name:"Server preview validation",exact:true})).toHaveCount(0);
  assert.equal((await (await api("/api/v1/queues/live_candidate")).json()).document.spec.retention.maxMessages,300);
  if(faultMode!=="socket-reset")assert.equal(forwardedPuts,putsBeforeUnknown+1);
  if(faultMode==="attempt-evidence"){
    await expect(page.getByRole("region",{name:"Receiving-attempt evidence",exact:true})).toContainText("No retry is enabled.");
    await expect(page.getByRole("region",{name:"Receiving-attempt evidence",exact:true})).toContainText("None reported; audit/lock metadata excluded.");
    await page.screenshot({path:path.join(evidence,"mutation-attempt-evidence.png"),fullPage:true});
    report.checks.push("synthetic-attempt-none-after-real-put-remains-unknown-no-retry");
  }
  const requestID=report.faultRequests[0].requestId;
  const auditResponse=await api(`/api/v1/audit/requests/${requestID}`);
  assert.equal(auditResponse.status,200);const auditEvidence=await auditResponse.json();
  assert.ok(auditEvidence.items.every(event=>event.requestId===requestID));
  assert.ok(auditEvidence.items.some(event=>event.phase==="outcome"&&event.httpStatus===200));
  assert.ok(auditEvidence.scanned<=256);report.checks.push("real-request-audit-retained-evidence");
  const windowResponse=await api(`/api/v1/audit/windows?${new URLSearchParams({resource:"live_candidate",phase:"outcome",requestId:requestID})}`,{headers:{Authorization:`Bearer ${auditor}`}});
  assert.equal(windowResponse.status,200);const globalWindow=await windowResponse.json();
  assert.ok(globalWindow.scanned<=256);assert.ok(globalWindow.items.length>0);
  assert.ok(globalWindow.items.every(event=>event.requestId===requestID&&event.resourceName==="live_candidate"&&event.phase==="outcome"));
  assert.equal(globalWindow.filter.requestId,requestID);
  report.checks.push("real-global-audit-window-auditor-exact-combined-filter");
  if(process.env.RJS_TEST_AUDIT_PAGING==="1"){
    // Add newer unrelated audit records only on this isolated broker. Each
    // no-op apply records its own intent/outcome, moving the target past 256.
    for(let index=0;index<130;index++){
      const current=await api("/api/v1/queues/live_candidate"),currentETag=current.headers.get("etag"),currentBody=await current.json();
      const result=await api("/api/v1/queues/live_candidate",{method:"PUT",headers:{"If-Match":currentETag,"X-Request-ID":`audit-window-fixture-${index}`},body:JSON.stringify(currentBody.document)});
      assert.equal(result.status,200);await result.arrayBuffer();
    }
  }
  const putsBeforeInspection=forwardedPuts;
  await page.getByRole("button",{name:"Inspect current state (read-only)",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Current evidence — not outcome attribution",exact:true})).toBeVisible();assert.equal(droppedApplies,1);assert.equal(forwardedPuts,putsBeforeInspection);await expect(draft).toBeDisabled();
  const editorDownloadPending=page.waitForEvent("download");
  await page.getByRole("link",{name:"Download editor evidence JSON",exact:true}).click();
  const editorDownload=await editorDownloadPending;
  const editorEvidencePath=path.join(evidence,"queue-editor-evidence.json");
  await editorDownload.saveAs(editorEvidencePath);
  const editorEvidenceText=await readFile(editorEvidencePath,"utf8"),editorEvidence=JSON.parse(editorEvidenceText);
  assert.equal(editorEvidence.schema,"rjs.queue-editor-evidence.v1");
  assert.equal(editorEvidence.phase,"uncertain");assert.equal(editorEvidence.requestId,requestID);
  if(faultMode==="attempt-evidence")assert.deepEqual(editorEvidence.error.mutation,syntheticMutation);
  assert.equal(editorEvidence.raw,await draft.inputValue());
  assert.equal(editorEvidence.inspection.declaration.status,"available");
  assert.equal(editorEvidence.inspection.audit.status,"available");
  assert.equal(editorEvidence.acceptedOperations,undefined);
  assert.equal(editorEvidenceText.includes(operator),false);assert.equal(editorEvidenceText.includes(auditor),false);
  assert.equal(forwardedPuts,putsBeforeInspection);await expect(draft).toBeDisabled();
  await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
  report.checks.push("unknown-write-local-evidence-download-preserves-request-history-and-lock-no-token");
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("link",{name:"Review deletion impact",exact:true}).click();
  await expect(page.getByRole("button",{name:"Archive this Queue editor for fresh preflight",exact:true})).toBeDisabled();
  await expect(page.getByRole("button",{name:"Read deletion preflight",exact:true})).toBeDisabled();
  await expect(page.getByRole("alert").filter({hasText:"Operation pending or outcome unknown; handoff is blocked"})).toBeVisible();
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();await expect(draft).toBeDisabled();
  assert.equal(forwardedPuts,putsBeforeInspection);
  report.checks.push("unknown-editor-cannot-handoff-to-delete-disabled-controls-no-write");
  if(process.env.RJS_TEST_AUDIT_PAGING==="1"){
    const first=await (await api(`/api/v1/audit/requests/${requestID}`)).json();assert.equal(first.items.length,0);assert.notEqual(first.nextBefore,null);
    await page.getByRole("button",{name:"Read older audit window",exact:true}).click();
    await expect(page.getByText(/Reached the observed retained lower boundary\./)).toBeVisible();
    const auditBlock=page.getByRole("heading",{name:"audit",exact:true}).locator("..");
    await expect(auditBlock).toContainText('"phase":"outcome"');assert.equal(forwardedPuts,putsBeforeInspection);await expect(draft).toBeDisabled();
    report.checks.push("empty-audit-window-older-page-finds-outcome-no-write-unlock");
  }
  if(testSessionExpiry){
    const putsBeforeExpiry=forwardedPuts;
    await page.clock.fastForward(61000);
    await expect(page.getByRole("alert").filter({hasText:"The verified credential expired"})).toBeVisible();
    await expect(page.getByRole("button",{name:"Skip to page content",exact:true})).toHaveCount(0);await expect(page.locator(".route-announcement,#console-content")).toHaveCount(0);
    await expect(page.getByRole("button",{name:"Inspect current state (read-only)",exact:true})).toHaveCount(0);
    await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
    await page.getByText("Retained draft and request evidence: live_candidate",{exact:true}).click();
    const retained=page.locator("details").filter({hasText:"Retained draft and request evidence: live_candidate"});
    await expect(retained).toContainText(requestID);await expect(retained).toContainText('"phase":"uncertain"');
    await expect(retained).toContainText('maxMessages');assert.equal(forwardedPuts,putsBeforeExpiry);
    const expiredDownloadPending=page.waitForEvent("download");
    await retained.getByRole("link",{name:"Download editor evidence JSON",exact:true}).click();
    const expiredDownload=await expiredDownloadPending,expiredPath=path.join(evidence,"expired-editor-evidence.json");
    await expiredDownload.saveAs(expiredPath);
    const expiredEvidence=JSON.parse(await readFile(expiredPath,"utf8"));
    assert.equal(expiredEvidence.requestId,requestID);assert.equal(expiredEvidence.phase,"uncertain");
    assert.equal(forwardedPuts,putsBeforeExpiry);
    await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
    await expect(retained).toBeVisible();
    report.checks.push("synthetic-session-expiry-real-unknown-write-retained-evidence-no-extra-put");
  }
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Clear session",exact:true}).click();await expect(page.getByLabel("Username",{exact:true})).toBeFocused();
  report.checks.push("confirmed-browser-apply-real-write",`${faultMode}-apply-response-unknown-locked-inspection-no-additional-put`);
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  await navigate("Create Queue");
  const creationSchemaPuts=forwardedPuts;
  await page.getByLabel("New Queue name",{exact:true}).fill("schema_retained_input");
  await page.getByLabel("Subjects (one per line)",{exact:true}).fill("schema.events");
  await page.getByLabel("Requested replicas",{exact:true}).selectOption("1");
  await page.getByLabel("Storage type",{exact:true}).selectOption("file");
  await page.getByLabel("Maximum stored messages",{exact:true}).fill("9007199254740993");
  await page.route("**/api/v1/console/queue-schema",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"unavailable"}}'}),{times:1});
  await page.getByRole("button",{name:"Reload creation schema",exact:true}).click();
  await expect(page.getByRole("alert")).toContainText("Creation schema unavailable or changed.");
  await expect(page.getByRole("button",{name:"Prepare creation draft",exact:true})).toBeDisabled();
  await expect(page.getByLabel("Requested replicas",{exact:true})).toBeDisabled();
  await expect(page.getByLabel("Requested replicas",{exact:true})).toHaveValue("1");
  await expect(page.getByLabel("Storage type",{exact:true})).toHaveValue("file");
  await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("schema_retained_input");
  await expect(page.getByLabel("Maximum stored messages",{exact:true})).toHaveValue("9007199254740993");
  await page.getByRole("button",{name:"Reload creation schema",exact:true}).click();
  await expect(page.getByRole("button",{name:"Prepare creation draft",exact:true})).toBeEnabled();
  await expect(page.getByLabel("Maximum stored messages",{exact:true})).toHaveValue("9007199254740993");
  assert.equal(forwardedPuts,creationSchemaPuts);
  await page.screenshot({path:path.join(evidence,"creation-schema-recovered.png"),fullPage:true});
  report.checks.push("creation-schema-failure-disables-prepare-retains-explicit-choices-exact-input-reload-no-write");
  const prepareCreation=async(name="new",preview=true)=>{
    await page.getByLabel("New Queue name",{exact:true}).fill(name);
    await page.getByLabel("Subjects (one per line)",{exact:true}).fill(`${name}.events`);
    await page.getByLabel("Requested replicas",{exact:true}).selectOption("1");
    await page.getByLabel("Storage type",{exact:true}).selectOption("file");
    await page.getByLabel("Maximum stored messages",{exact:true}).fill("100");
    await page.getByRole("button",{name:"Prepare creation draft",exact:true}).click();
    if(preview)await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  };
  await prepareCreation();await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();assert.equal((await api("/api/v1/queues/new")).status,404);
  await expect(page.getByRole("region",{name:"Declaration change review",exact:true})).toContainText("Create-only: no previous declaration");
  const submittedCreation=await draft.inputValue(),beforeNormalizationPuts=forwardedPuts;
  await page.getByText("Submitted versus normalized fields",{exact:true}).click();
  const normalizationTable=page.getByRole("region",{name:"Normalization field comparison",exact:true});
  const ackDefault=normalizationTable.getByRole("row").filter({has:page.getByRole("rowheader",{name:"/spec/delivery/ackWait",exact:true})});
  await expect(ackDefault.getByRole("cell").nth(0)).toHaveText("Not supplied");
  await expect(ackDefault.getByRole("cell").nth(1)).toHaveText('"30s"');
  await expect(ackDefault.getByRole("cell").nth(2)).toHaveText("Added by normalization");
  const deliveriesDefault=normalizationTable.getByRole("row").filter({has:page.getByRole("rowheader",{name:"/spec/delivery/maxDeliver",exact:true})});
  await expect(deliveriesDefault.getByRole("cell").nth(1)).toHaveText("5");
  assert.equal(await draft.inputValue(),submittedCreation);assert.equal(forwardedPuts,beforeNormalizationPuts);
  await page.screenshot({path:path.join(evidence,"queue-normalization-review.png"),fullPage:true});
  await page.getByText("Submitted versus normalized fields",{exact:true}).click();
  report.checks.push("real-server-normalized-defaults-visible-without-draft-replacement-or-put");
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  const createdResponse=await api("/api/v1/queues/new"),createdETag=createdResponse.headers.get("etag");assert.equal(createdResponse.status,200);assert.equal((await createdResponse.json()).document.spec.replicas,1);
  const firstCreationID=await page.locator("code").last().textContent(),putsBeforeAnother=forwardedPuts;
  const another=page.getByRole("button",{name:"Create another Queue",exact:true});
  await another.click();await expect(page.getByLabel("New Queue name",{exact:true})).toBeFocused();
  for(const label of ["New Queue name","Subjects (one per line)","Requested replicas","Storage type","Maximum stored messages"])await expect(page.getByLabel(label,{exact:true})).toHaveValue("");
  await page.getByText("Accepted creation requests (session memory)",{exact:true}).click();
  await expect(page.locator(".creation-history")).toContainText(firstCreationID);
  await prepareCreation("new",false);await expect(page.getByRole("alert")).toContainText("already has a retained creation request");
  await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("new");assert.equal(forwardedPuts,putsBeforeAnother);
  await prepareCreation("new_second");await expect(another).toHaveCount(0);await expect(applyButton).toBeDisabled();
  assert.equal((await api("/api/v1/queues/new_second")).status,404);
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();assert.equal(forwardedPuts,putsBeforeAnother+1);
  assert.notEqual(await page.locator("code").last().textContent(),firstCreationID);
  assert.equal((await api("/api/v1/queues/new")).headers.get("etag"),createdETag);
  assert.equal((await api("/api/v1/queues/new_second")).status,200);
  await another.click();await navigate("Queue list");await navigate("Create Queue");
  await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("");
  await page.getByText("Accepted creation requests (session memory)",{exact:true}).click();
  await expect(page.locator(".creation-history h3")).toHaveText(["new","new_second"]);
  await expect(page.locator(".creation-history")).toContainText(firstCreationID);
  await page.getByText("Accepted creation requests (session memory)",{exact:true}).click();
  await page.screenshot({path:path.join(evidence,"create-another.png"),fullPage:true});
  report.checks.push("multiple-creations-explicit-blank-settings-retained-receipts-name-reuse-rejected-second-create-navigation");
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  await prepareCreation();await expect(page.getByText(/Creation conflicts with current resource state\./)).toBeVisible();await expect(applyButton).toHaveCount(0);await expect(page.getByRole("button",{name:"Read latest for comparison",exact:true})).toHaveCount(0);
  assert.equal((await api("/api/v1/queues/new")).headers.get("etag"),createdETag);
  const beforeParkPuts=forwardedPuts,conflictedRaw=await draft.inputValue();
  await page.getByRole("button",{name:"Keep draft and choose another name",exact:true}).click();
  await expect(page.getByLabel("New Queue name",{exact:true})).toBeFocused();
  for(const label of ["New Queue name","Subjects (one per line)","Requested replicas","Storage type","Maximum stored messages"])await expect(page.getByLabel(label,{exact:true})).toHaveValue("");
  await page.getByText("Unsubmitted creation drafts (read-only records)",{exact:true}).click();
  const parked=page.locator(".parked-creations");
  assert.equal(JSON.parse(await parked.locator("pre").textContent()).raw,conflictedRaw);
  assert.equal(JSON.parse(await parked.locator("pre").textContent()).error.status,409);
  await navigate("Queue list");await navigate("Create Queue");
  await page.getByText("Unsubmitted creation drafts (read-only records)",{exact:true}).click();
  await expect(parked.getByRole("heading",{name:"new",exact:true})).toBeVisible();
  await page.getByLabel("New Queue name",{exact:true}).fill("unsaved_form");
  const resumeDraft=page.getByRole("button",{name:"Resume creation draft: new",exact:true});
  await resumeDraft.click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
  await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("unsaved_form");
  await resumeDraft.click();await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
  await expect(draft).toHaveValue(conflictedRaw);await expect(draft).toBeFocused();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toHaveCount(0);
  await expect(applyButton).toHaveCount(0);assert.equal(forwardedPuts,beforeParkPuts);
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByText(/Creation conflicts with current resource state\./)).toBeVisible();
  await page.getByRole("button",{name:"Keep draft and choose another name",exact:true}).click();
  report.checks.push("resume-retained-create-draft-confirm-form-discard-exact-raw-focus-fresh-preview-no-write");
  await prepareCreation("new",false);await expect(page.getByRole("alert")).toContainText("already has a retained creation request");
  assert.equal(forwardedPuts,beforeParkPuts);
  await prepareCreation("recovered_create");await expect(applyButton).toBeDisabled();
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  assert.equal((await api("/api/v1/queues/recovered_create")).status,200);
  assert.equal((await api("/api/v1/queues/new")).headers.get("etag"),createdETag);
  assert.equal(forwardedPuts,beforeParkPuts+1);
  await expect(page.getByRole("button",{name:"Keep draft and choose another name",exact:true})).toHaveCount(0);
  await page.screenshot({path:path.join(evidence,"creation-conflict-recovered.png"),fullPage:true});
  report.checks.push("pre-write-create-conflict-parked-readonly-evidence-name-change-fresh-preview-no-overwrite");
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Clear session",exact:true}).click();
  report.checks.push("browser-create-new-name-explicit-settings-preview-no-write-duplicate-no-overwrite");
  // Destructive verification is restricted to these two newly created fixtures
  // inside this harness's isolated loopback broker, never an existing service.
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  let deletionWrites=0;
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();await expect(draft).not.toHaveValue("");
  const unrelatedHandoffRaw="{unrelated draft retained across handoff";await draft.fill(unrelatedHandoffRaw);
  for(const [name,unknown] of [["live_delete_fixture",false],["live_delete_unknown",true]]){
    const endpoint=`/api/v1/queues/${name}`;
    const doc={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:[`${name}.events`],replicas:1,storage:"file",retention:{maxMessages:100}}};
    assert.equal((await api(endpoint,{method:"PUT",headers:{"If-None-Match":"*"},body:JSON.stringify(doc)})).status,200);
    await navigate("Queue list");await page.getByRole("link",{name,exact:true}).click();
    await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();await expect(draft).not.toHaveValue("");
    let handoffRaw;
    if(unknown){handoffRaw="{invalid draft retained for deletion handoff";await draft.fill(handoffRaw);}
    else{
      const handoffDoc=JSON.parse(await draft.inputValue());handoffDoc.spec.retention.maxMessages=101;handoffRaw=JSON.stringify(handoffDoc);await draft.fill(handoffRaw);
      await page.getByRole("button",{name:"Preview changes",exact:true}).click();
      await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await applyButton.click();
      await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    }
    await navigate("Queue list");await page.getByRole("link",{name,exact:true}).click();
    await page.getByRole("link",{name:"Review deletion impact",exact:true}).click();
    await page.getByRole("button",{name:"简体中文",exact:true}).click();
    await expect(page.getByRole("region",{name:"Queue 删除",exact:true})).toContainText(name);
    await expect(page.getByRole("region",{name:"编辑器转删除交接",exact:true})).toContainText("结束此 Queue 的编辑审阅");
    await page.getByRole("button",{name:"English",exact:true}).click();
    const preflight=page.getByRole("button",{name:"Read deletion preflight",exact:true});await expect(preflight).toBeDisabled();
    const handoff=page.getByRole("button",{name:"Archive this Queue editor for fresh preflight",exact:true});
    await handoff.click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();await expect(preflight).toBeDisabled();
    const handoffBefore=await api(endpoint),handoffETag=handoffBefore.headers.get("etag");await handoffBefore.arrayBuffer();
    await handoff.click();await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();await expect(preflight).toBeEnabled();
    assert.equal((await api(endpoint)).headers.get("etag"),handoffETag);
    await page.getByText("Archived editor evidence: 1",{exact:true}).click();
    const handoffDownloadPending=page.waitForEvent("download");await page.getByRole("link",{name:"Download editor evidence JSON",exact:true}).click();
    const handoffDownload=await handoffDownloadPending,handoffPath=path.join(evidence,`${name}-archived-editor.json`);await handoffDownload.saveAs(handoffPath);
    const archivedEditor=JSON.parse(await readFile(handoffPath,"utf8"));assert.equal(archivedEditor.raw,handoffRaw);assert.equal(archivedEditor.phase,unknown?"editing":"accepted");
    if(!unknown)assert.ok(archivedEditor.requestId);
    await page.screenshot({path:path.join(evidence,`${name}-handoff.png`),fullPage:true});
    await page.getByText("Archived editor evidence: 1",{exact:true}).click();
    await preflight.click();
    const deleteButton=page.getByRole("button",{name:"Delete this Queue",exact:true});await expect(deleteButton).toBeDisabled();
    const typed=page.getByLabel("Type exact Queue name",{exact:true}),ack=page.getByLabel("I reviewed this impact and understand that deletion is destructive.",{exact:true});
    await typed.fill(` ${name}`);await ack.check();await expect(deleteButton).toBeDisabled();
    await typed.fill(name);await ack.check();await expect(deleteButton).toBeEnabled();
    await preflight.click();await expect(typed).toHaveValue("");await expect(ack).not.toBeChecked();
    await page.getByLabel("Force deletion with messages",{exact:true}).check();await typed.fill(name);await ack.check();
    await page.getByLabel("Force deletion with messages",{exact:true}).uncheck();await expect(typed).toHaveValue("");await expect(ack).not.toBeChecked();
    await typed.fill(name);await ack.check();
    await page.screenshot({path:path.join(evidence,`${name}-review.png`),fullPage:true});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    const intercept=async route=>{
      if(route.request().method()!=="DELETE")return route.continue();
      deletionWrites++;const headers=route.request().headers();assert.equal(headers["x-rjs-confirm-queue"],name);assert.ok(headers["if-match"]);assert.ok(headers["x-request-id"]);
      const result=await route.fetch();assert.equal(result.status(),200);
      if(unknown)return route.fulfill({status:faultMode==="attempt-evidence"?503:200,contentType:"application/json",body:faultMode==="attempt-evidence"?syntheticMutationError:"unreadable response after real deletion"});
      await route.fulfill({response:result});
    };
    await page.route(`**${endpoint}?force=false`,intercept);
    const beforeCapabilityDeletes=deletionWrites;
    await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,headers:{ETag:capabilitiesResponse.headers.get("etag")},contentType:"application/json",body:JSON.stringify(changedCapabilities)}),{times:1});
    await navigate("Access and settings");await expect(page.getByRole("region",{name:"Server capabilities",exact:true})).toContainText("cluster");
    await page.goBack();await expect(page.getByRole("alert")).toContainText("No deletion was sent");
    await expect(deleteButton).toHaveCount(0);assert.equal(deletionWrites,beforeCapabilityDeletes);
    await preflight.click();await expect(typed).toHaveValue("");await expect(ack).not.toBeChecked();await typed.fill(name);await ack.check();
    await page.route("**/api/v1/console/capabilities",route=>route.fulfill({status:200,headers:{ETag:capabilitiesResponse.headers.get("etag")},contentType:"application/json",body:JSON.stringify(changedCapabilities)}),{times:1});
    await deleteButton.click();await expect(page.getByRole("alert")).toContainText("No deletion was sent");
    assert.equal(deletionWrites,beforeCapabilityDeletes);assert.equal((await api(endpoint)).status,200);await expect(deleteButton).toHaveCount(0);
    await preflight.click();await expect(typed).toHaveValue("");await expect(ack).not.toBeChecked();
    await typed.fill(name);await ack.check();
    await deleteButton.click();
    if(unknown)await expect(page.getByRole("alert")).toContainText("Deletion outcome unknown");
    else await expect(page.getByRole("region",{name:"Queue deletion",exact:true}).getByRole("status")).toContainText("Server acknowledged deletion");
    if(unknown&&faultMode==="attempt-evidence"){
      await expect(page.getByRole("region",{name:"Receiving-attempt evidence",exact:true})).toContainText("No retry is enabled.");
      report.checks.push("synthetic-attempt-none-after-real-delete-remains-unknown-no-retry");
    }
    await page.unroute(`**${endpoint}?force=false`,intercept);
    await expect(deleteButton).toHaveCount(0);await expect(preflight).toHaveCount(0);
    assert.equal((await api(endpoint)).status,404);assert.equal((await api(`/api/v1/streams/RJSQ_${name}`)).status,404);
    // Move this deletion beyond the first 256-record scan with unrelated
    // audited no-op applies, exclusively on the harness-owned fixture.
    for(let index=0;index<130;index++){
      const current=await api("/api/v1/queues/live_candidate"),currentETag=current.headers.get("etag"),currentBody=await current.json();
      const applied=await api("/api/v1/queues/live_candidate",{method:"PUT",headers:{"If-Match":currentETag,"X-Request-ID":`${name}-audit-padding-${index}`},body:JSON.stringify(currentBody.document)});
      assert.equal(applied.status,200);assert.equal((await applied.json()).status,"noop");
    }
    await page.getByRole("button",{name:"Read deletion outcome evidence",exact:true}).click();
    await expect(page.getByText("Audit evidence is a bounded window, not necessarily complete history. Reads never unlock retry.",{exact:true})).toBeVisible();
    if(unknown)await expect(page.getByRole("alert")).toContainText("Deletion outcome unknown");
    const receiptID=await page.locator("code").last().textContent();
    const olderDeletion=page.getByRole("button",{name:"Read older deletion audit window",exact:true});
    let deletionWindows=1;
    while(await olderDeletion.count()){
      assert.ok(deletionWindows<16,"Unexpectedly large fixture audit history");
      await olderDeletion.click();deletionWindows++;
      await expect(page.getByText(new RegExp(`^Audit windows read: ${deletionWindows}\\.`))).toBeVisible();
    }
    assert.ok(deletionWindows>=2,"Fixture must exercise real cursor traversal");
    await page.getByRole("button",{name:"Read deletion outcome evidence",exact:true}).click();
    await expect(page.getByText("Previous deletion inspections: 1",{exact:true})).toBeVisible();
    const deleteDownloadPending=page.waitForEvent("download");
    await page.getByRole("link",{name:"Download deletion evidence JSON",exact:true}).click();
    const deleteDownload=await deleteDownloadPending,deleteEvidencePath=path.join(evidence,`${name}-evidence.json`);
    await deleteDownload.saveAs(deleteEvidencePath);
    const deleteEvidenceText=await readFile(deleteEvidencePath,"utf8"),deleteEvidence=JSON.parse(deleteEvidenceText);
    assert.equal(deleteEvidence.schema,"rjs.queue-delete-evidence.v1");assert.equal(deleteEvidence.requestId,receiptID);
    assert.equal(deleteEvidence.phase,unknown?"uncertain":"accepted");assert.equal(deleteEvidence.inspection.declaration.status,"missing");
    if(unknown&&faultMode==="attempt-evidence")assert.deepEqual(deleteEvidence.error.mutation,syntheticMutation);
    assert.equal(deleteEvidence.inspection.stream.status,"missing");assert.equal(deleteEvidence.inspection.audit.windows.length,1);
    assert.equal(deleteEvidence.inspectionHistory[0].audit.windows.length,deletionWindows);
    assert.equal(deleteEvidence.inspectionHistory[0].audit.windows[0].body.items.length,0);
    assert.ok(deleteEvidence.inspectionHistory[0].audit.windows.slice(1).some(window=>window.body.items.some(item=>item.requestId===receiptID)));
    assert.equal(deleteEvidence.inspectionHistory[0].audit.windows.at(-1).body.nextBefore,null);
    assert.equal(deleteEvidenceText.includes(operator),false);assert.equal(deleteEvidenceText.includes(auditor),false);
    await expect(deleteButton).toHaveCount(0);await expect(preflight).toHaveCount(0);
    await navigate("Queue list");await page.goBack();await expect(page.locator("code").last()).toHaveText(receiptID);
    await page.screenshot({path:path.join(evidence,`${name}-outcome.png`),fullPage:true});
  }
  assert.equal(deletionWrites,2);
  report.checks.push("settings-capability-observation-invalidates-retained-deletion-confirmation-before-submit");
  report.checks.push("capability-change-before-delete-invalidates-review-no-delete-fresh-preflight-and-confirm-required");
  await navigate("Queue list");await page.getByRole("link",{name:"live_candidate",exact:true}).click();
  await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();await expect(draft).toHaveValue(unrelatedHandoffRaw);
  report.checks.push("editor-delete-handoff-cancel-accepted-and-invalid-archive-download-token-retained-other-draft-unchanged");
  report.checks.push("real-deletion-audit-cursor-traversal-refresh-history-native-json-download-no-retry");
  report.checks.push("isolated-delete-ui-exact-confirm-refresh-force-reset-single-dispatch-real-delete-unknown-response-readonly-evidence-navigation");
  for(const name of ["live_dlq_target","live_dlq_source"]){
    const value={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:[`${name}.events`],replicas:clusterMode?3:1,storage:"file",retention:{maxMessages:100},...(name==="live_dlq_source"?{deadLetter:{queue:"live_dlq_target"}}:{})}};
    const created=await api(`/api/v1/queues/${name}`,{method:"PUT",headers:{"If-None-Match":"*"},body:JSON.stringify(value)});assert.equal(created.status,200);await created.arrayBuffer();
  }
  const beforeDLQReads=forwardedPuts;
  const dlqReadEvents=new Map();report.dlqReads=[];
  page.on("request",request=>{
    const endpoint=new URL(request.url()).pathname;
    if(request.method()!=="GET"||!["/api/v1/controller","/api/v1/queues/live_dlq_target","/api/v1/streams/RJSQ_live_dlq_target"].includes(endpoint))return;
    const entry={endpoint,startedAt:Date.now()};dlqReadEvents.set(request,entry);report.dlqReads.push(entry);
  });
  page.on("response",response=>{const entry=dlqReadEvents.get(response.request());if(entry){entry.status=response.status();entry.headersAfterMs=Date.now()-entry.startedAt;}});
  page.on("requestfinished",request=>{const entry=dlqReadEvents.get(request);if(entry)entry.finishedAfterMs=Date.now()-entry.startedAt;});
  page.on("requestfailed",request=>{const entry=dlqReadEvents.get(request);if(entry){entry.failedAfterMs=Date.now()-entry.startedAt;entry.failed=true;}});
  await navigate("Queue list");await page.getByRole("link",{name:"live_dlq_source",exact:true}).click();
  await page.getByRole("navigation",{name:"Queue detail tabs",exact:true}).getByRole("link",{name:"Configuration",exact:true}).click();
  const dlq=page.getByRole("region",{name:"DLQ diagnosis",exact:true}),targetDeclaration=page.getByRole("region",{name:"DLQ target declaration",exact:true}),targetStream=page.getByRole("region",{name:"DLQ target Stream",exact:true});
  await expect(targetDeclaration).toContainText("Observed in this read");await expect(targetStream).toContainText("Observed in this read");
  await expect(dlq).toContainText("Controller process — all Queues");await expect(dlq).toContainText("Per-Queue transfer history is unavailable");
  await expect(dlq).toContainText("processing attempts, not unique messages");
  await expect(dlq).toContainText("Last successful run does not prove every transfer succeeded");
  const ignoredCounter=dlq.locator("dt").filter({hasText:/^Reported ignored$/}).locator("+ dd");
  await expect(ignoredCounter).toHaveText("0");
  await page.route("**/api/v1/queues/live_dlq_target",route=>route.fulfill({status:404,contentType:"application/json",body:'{"error":{"code":"not_found"}}'}),{times:1});
  await dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true}).click();
  await expect(targetDeclaration).toContainText("Not found in this read");await expect(targetStream).toContainText("Not observed");
  const saveDLQEvidence=async label=>{
    // The target panels can settle before the independent controller read. Its
    // production transport timeout is 10 seconds, so the readiness assertion
    // must cover that bound rather than Playwright's shorter default.
    try{await expect(dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true})).toBeEnabled({timeout:15000});}
    catch(error){
      report.dlqReadinessFailure={stage:label,observedAt:new Date().toISOString(),statuses:await dlq.locator('[role="status"], [role="alert"]').allTextContents()};
      await dlq.screenshot({path:path.join(evidence,"dlq-readiness-failure.png")});throw error;
    }
    const requests=[],observe=request=>{if(new URL(request.url()).pathname.startsWith("/api/"))requests.push(request.method());};
    page.on("request",observe);const pending=page.waitForEvent("download");
    await dlq.getByRole("link",{name:"Download DLQ evidence JSON",exact:true}).click();
    const download=await pending,file=path.join(evidence,`dlq-evidence-${label}.json`);await download.saveAs(file);page.off("request",observe);assert.deepEqual(requests,[]);
    const text=await readFile(file,"utf8"),value=JSON.parse(text);assert.equal(text.includes(operator),false);assert.equal(text.includes(auditor),false);
    assert.equal(value.schema,"rjs.dlq-diagnostic-evidence.v1");assert.equal(value.source.queue,"live_dlq_source");assert.equal(value.source.target.queue,"live_dlq_target");
    assert.equal(value.source.readAt,await page.locator(".declaration-meta time").getAttribute("datetime"));
    assert.ok(value.source.declarationETag);assert.ok(value.limitations.some(text=>text.includes("all Queues")));return value;
  };
  const missingDLQ=await saveDLQEvidence("missing");assert.equal(missingDLQ.target.phase,"missing");assert.equal(missingDLQ.stream.phase,"unobserved");assert.equal(missingDLQ.target.value,undefined);
  await dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true}).click();
  await expect(targetDeclaration).toContainText("Observed in this read");await expect(targetStream).toContainText("Observed in this read");
  const availableDLQ=await saveDLQEvidence("available");assert.equal(availableDLQ.target.phase,"available");assert.equal(availableDLQ.stream.phase,"available");assert.ok(availableDLQ.target.etag);assert.equal(availableDLQ.source.declarationETag,missingDLQ.source.declarationETag);
  assert.equal(availableDLQ.controller.value.dlqIgnored,0);
  await page.route("**/api/v1/controller",async route=>{const response=await route.fetch(),body=await response.json();delete body.dlqIgnored;await route.fulfill({response,json:body});},{times:1});
  await dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true}).click();await expect(ignoredCounter).toHaveText("Unreported");
  const oldControllerDLQ=await saveDLQEvidence("older-controller");assert.equal(Object.hasOwn(oldControllerDLQ.controller.value,"dlqIgnored"),false);
  await dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true}).click();await expect(ignoredCounter).toHaveText("0");
  report.checks.push("dlq-ignored-real-zero-old-server-missing-unreported-export-preserves-absence-recovery");
  const controllerEvidence=page.getByRole("region",{name:"DLQ controller evidence",exact:true}),refreshDLQ=dlq.getByRole("button",{name:"Refresh DLQ evidence",exact:true});
  for(const invalidKind of ["lastRun","lastSuccess","json"]){
    await page.route("**/api/v1/controller",async route=>{
      if(invalidKind==="json"){await route.fulfill({status:200,contentType:"application/json",body:'{"'});return;}
      const response=await route.fetch(),body=await response.json();body[invalidKind]="2026-02-30T00:00:00Z";await route.fulfill({response,json:body});
    },{times:1});
    await refreshDLQ.click();await expect(controllerEvidence).toContainText("Incompatible response");
    const invalid=await saveDLQEvidence(`invalid-${invalidKind}`);assert.equal(invalid.controller.phase,"invalid");assert.equal(invalid.controller.value,undefined);assert.equal(invalid.target.phase,"available");assert.equal(invalid.stream.phase,"available");
    await refreshDLQ.click();await expect(controllerEvidence).toContainText("Observed in this read");await expect(refreshDLQ).toBeEnabled();
  }
  report.checks.push("dlq-invalid-controller-dates-and-json-clear-export-values-and-recover");
  for(const mode of ["release-1","release-2","timeout"]){
    let release,entered=false,finished;
    const gate=new Promise(resolve=>{release=resolve;}),handled=new Promise(resolve=>{finished=resolve;});
    await page.route("**/api/v1/controller",async route=>{
      entered=true;await gate;
      try{if(mode==="timeout")await route.abort();else await route.continue();}finally{finished();}
    },{times:1});
    try{
      await refreshDLQ.click();await expect.poll(()=>entered).toBe(true);
      await expect(controllerEvidence).toContainText("Reading…");await expect(refreshDLQ).toBeDisabled();
      await expect(targetStream).toContainText("Observed in this read");
      if(mode==="timeout"){
        // The production transport deadline is 10s; this is a fault assertion,
        // not a relaxation of the normal five-second readiness assertion.
        await expect(controllerEvidence).toContainText("Read unavailable",{timeout:15000});await expect(refreshDLQ).toBeEnabled();
      }
    }finally{release();if(entered)await handled;}
    if(mode==="timeout"){
      const timedOut=await saveDLQEvidence("controller-timeout");assert.equal(timedOut.controller.phase,"unavailable");assert.equal(timedOut.controller.value,undefined);
      await refreshDLQ.click();
    }
    await expect(controllerEvidence).toContainText("Observed in this read");await expect(refreshDLQ).toBeEnabled();
  }
  report.checks.push("dlq-controlled-controller-delay-repeat-deadline-and-recovery");
  assert.equal(forwardedPuts,beforeDLQReads);
  await page.screenshot({path:path.join(evidence,"dlq-diagnostics.png"),fullPage:true});
  await auditAccessibility(page,"dlq-diagnostics");
  report.checks.push("dlq-real-target-declaration-stream-one-hop-missing-injection-recovery-process-counter-scope-no-write");
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Clear session",exact:true}).click();
  if(metadataDrift){
    const name="live_metadata_check",endpoint=`/api/v1/queues/${name}`;
    const metadataDocument={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name,labels:{empty:""}},spec:{subjects:[`${name}.events`],replicas:1,maxPriority:2,storage:"file",retention:{maxMessages:100}}};
    const created=await api(endpoint,{method:"PUT",headers:{"If-None-Match":"*"},body:JSON.stringify(metadataDocument)});
    assert.equal(created.status,200);await created.arrayBuffer();
    const original=await api(endpoint),originalETag=original.headers.get("etag");await original.arrayBuffer();
    const helper=async mode=>{
      const child=start(`metadata-${mode}`,metadataBinary,["-nats",`nats://127.0.0.1:${nodeSpecs[0].natsPort}`,"-mode",mode]);
      const [code]=await once(child,"close");assert.equal(code,0,(logs.get(`metadata-${mode}`)??[]).join(""));
    };
    await helper("inject");
    const previewResponse=await api(`${endpoint}/preview`,{method:"POST",headers:{"If-Match":originalETag},body:JSON.stringify(metadataDocument)});
    assert.equal(previewResponse.status,200);const previewBody=await previewResponse.json();
    assert.equal(previewBody.declaration_review.status,"available");assert.deepEqual(previewBody.declaration_review.diff.changes,[]);
    assert.equal(previewBody.result.status,"ready");assert.equal(previewBody.result.operations.length,4);
    for(const operation of previewBody.result.operations){
      assert.equal(operation.action,"update");assert.equal(operation.changes.length,2);
      const removed=operation.changes.find(change=>change.path==="metadata.rabbit-jetstream.io/label.removed");
      const added=operation.changes.find(change=>change.path==="metadata.rabbit-jetstream.io/label.empty");
      assert.equal(removed.from,'""');assert.equal(removed.to,"(absent)");
      assert.equal(added.from,"(absent)");assert.equal(added.to,'""');
    }
    assert.equal((await api(endpoint)).headers.get("etag"),originalETag);
    await writeFile(path.join(evidence,"metadata-drift-preview.json"),JSON.stringify(previewBody,null,2));
    const repaired=await api(endpoint,{method:"PUT",headers:{"If-Match":originalETag},body:JSON.stringify(metadataDocument)});
    assert.equal(repaired.status,200);await repaired.arrayBuffer();await helper("verify");
    const latest=await api(endpoint);const latestETag=latest.headers.get("etag");await latest.arrayBuffer();
    const converged=await api(`${endpoint}/preview`,{method:"POST",headers:{"If-Match":latestETag},body:JSON.stringify(metadataDocument)});
    assert.equal(converged.status,200);assert.equal((await converged.json()).result.status,"noop");
    report.checks.push("real-broker-metadata-drift-empty-declaration-diff-resource-repair-preserves-external-stream-primary-priorities");
    await helper("conflict");
    const conflictPreview=await api(`${endpoint}/preview`,{method:"POST",headers:{"If-Match":latestETag},body:JSON.stringify(metadataDocument)});
    assert.equal(conflictPreview.status,200);const conflictBody=await conflictPreview.json();
    assert.equal(conflictBody.result.status,"blocked");assert.ok(conflictBody.result.operations.every(operation=>operation.blocked&&operation.reason.includes("ownership")));
    const refused=await api(endpoint,{method:"PUT",headers:{"If-Match":latestETag},body:JSON.stringify(metadataDocument)});
    assert.equal(refused.status,409);assert.equal((await refused.json()).blocked,true);
    assert.equal((await api(endpoint)).headers.get("etag"),latestETag);
    await helper("verify-conflict");
    await writeFile(path.join(evidence,"metadata-ownership-conflict.json"),JSON.stringify(conflictBody,null,2));
    report.checks.push("real-broker-foreign-resource-ownership-blocks-preview-and-apply-no-adoption-or-declaration-write");
  }
  await routingBindingChecks({page,api,origin,auditor,expect,assert,evidence,path,report});
  await queueExportChecks({page,api,origin,auditor,expect,assert,evidence,path,report});
  await queueTemplateChecks({page,api,origin,operator,expect,assert,evidence,path,report});
  await queueImportChecks({page,api,origin,operator,expect,assert,evidence,path,report});
  await batchImportChecks({page,api,origin,operator,expect,assert,evidence,path,report});
  await bulkChangeChecks({page,api,origin,operator,expect,assert,report});
  await page.goto(origin+"/admin/queues/by-name/live_candidate");
  await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(auditor);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();await tabs.getByRole("link",{name:"Consumers",exact:true}).click();await expect(region).toBeVisible();
  const beforeBrokerLossRows=await region.getByRole("row").allTextContents(),beforeBrokerLossTime=await queueConsumerView.locator("time").getAttribute("datetime");
  for(const spec of nodeSpecs)await stop(spec.child);
  await page.getByRole("button",{name:"Refresh Consumers",exact:true}).click();
  await expect(page.getByRole("alert")).toContainText("unavailable",{timeout:15000});await expect(region).toBeVisible();
  assert.deepEqual(await region.getByRole("row").allTextContents(),beforeBrokerLossRows);await expect(queueConsumerView.locator("time")).toHaveAttribute("datetime",beforeBrokerLossTime);
  await expect(queueConsumerView).toContainText("Stale collection observation");await expect(queueConsumerView).toContainText("not current observations");
  report.checks.push("real-broker-loss-retains-explicitly-historical-consumer-rows-and-original-time");
  await navigate("Node list");
  const unavailableNodes=page.getByRole("region",{name:"Node collection",exact:true});
  await expect(unavailableNodes).toContainText("Monitoring unavailable",{timeout:15000});
  await expect(unavailableNodes.getByRole("link")).toHaveCount(0);
  report.checks.push("broker-loss-preserves-unavailable-endpoint-without-stale-node-id");
  await navigate("Overview");
  await expect(page.getByRole("region",{name:"Management and account",exact:true}).getByRole("alert")).toBeVisible({timeout:15000});
  await expect(page.getByRole("region",{name:"Monitoring summary",exact:true})).toContainText("Unavailable");
  report.checks.push("overview-broker-loss-account-error-monitoring-evidence-retained");
  await navigate("Compatibility");await expect(page.getByRole("region",{name:"Management build",exact:true})).toContainText("Go runtime");
  await expect(page.getByRole("region",{name:"Native SDK contract",exact:true})).toContainText("native-sdk-implemented-unreleased");
  report.checks.push("compatibility-build-and-sdk-readable-after-real-broker-loss");
  assert.deepEqual(errors,[]);
  if(accessibility)assert.deepEqual(report.accessibility.flatMap(check=>check.violations.map(issue=>({page:check.name,...issue}))),[],"Automatic accessibility violations require fixes; incomplete findings need manual review");
  assert.deepEqual(await snapshotCandidate(assets),candidateSnapshot,"Candidate file set or content changed during regression");
  for(const input of report.inputs)assert.deepEqual(await fingerprint(path.join(root,input.path)),input,"Candidate input changed during regression");
  report.inputsVerifiedAt=new Date().toISOString();report.passed=true;
}catch(error){report.error=error.message.replaceAll(operator,"[redacted]").replaceAll(auditor,"[redacted]");
  try{const livePage=browser?.contexts()?.[0]?.pages()?.[0];report.failureURL=livePage?.url();await livePage?.screenshot({path:path.join(evidence,"failure.png"),fullPage:true});report.dialogContext=await livePage?.evaluate(()=>({backdrops:document.querySelectorAll(".dialog-backdrop").length,panels:[...document.querySelectorAll(".dialog-panel")].map(panel=>panel.textContent)}));}catch{}
  throw new Error(report.error);}
finally {
  await browser?.close();
  if(front){front.closeAllConnections();await new Promise(resolve=>front.close(resolve));}
  if(prometheusServer){prometheusServer.closeAllConnections();await new Promise(resolve=>prometheusServer.close(resolve));}
  for(const child of children.toReversed())await stop(child);
  for(const [name,chunks] of logs)await writeFile(path.join(evidence,`${name}.log`),chunks.join("").replaceAll(operator,"[redacted]").replaceAll(auditor,"[redacted]"));
  report.finishedAt=new Date().toISOString();await writeFile(path.join(evidence,"report.json"),JSON.stringify(report,null,2));
  console.log(`Real-service smoke ${report.passed?"passed":"failed"}; evidence: ${evidence}`);
}
