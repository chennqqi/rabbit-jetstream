import assert from "node:assert/strict";
import {createServer} from "node:http";
import {once} from "node:events";
import {createRequire} from "node:module";
import {readFile} from "node:fs/promises";
import {fileURLToPath} from "node:url";
import path from "node:path";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"../..");
const assets=path.join(root,"admin-ui/build-candidate"),index=await readFile(path.join(assets,"index.html"));
const require=createRequire(path.join(root,"admin-ui/package.json"));
const {chromium,expect}=require("@playwright/test");
const requests=[];
const server=createServer(async(req,res)=>{
  const url=new URL(req.url,"http://fixture.invalid");
  if(url.pathname.startsWith("/api/")){
    let status=200,body={};
    if(url.pathname==="/api/v1/oidc/config"){status=404;body={error:{code:"not_found",message:"disabled"}};}
    else if(url.pathname==="/api/v1/auth/login")body={access_token:"local-access-token",token_type:"Bearer",expires_at:"2026-09-12T00:00:00Z",actor:"local:alice",role:"operator",tenants:["alpha","beta"]};
    else if(url.pathname==="/api/v1/session")body={actor:"local:alice",role:"operator",permissions:["queue:read"],expires_at:"2026-09-12T00:00:00Z",resource_read_policy:"authenticated",tenants:["alpha","beta"]};
    else if(url.pathname==="/api/v1/queues")body={items:[],total:0,offset:Number(url.searchParams.get("offset")??0),limit:Number(url.searchParams.get("limit")??50)};
    else {status=404;body={error:{code:"not_found",message:"not found"}};}
    requests.push({path:url.pathname,tenant:req.headers["x-rjs-tenant"]??"",authorization:req.headers.authorization??""});
    res.writeHead(status,{"content-type":"application/json","cache-control":"no-store"});res.end(JSON.stringify(body));return;
  }
  if(req.method!=="GET"||!url.pathname.startsWith("/admin/")){res.writeHead(404);res.end();return;}
  const asset=url.pathname.startsWith("/admin/assets/")?url.pathname.slice(7):"index.html";
  if(!/^(index\.html|assets\/[a-zA-Z0-9_-]+\.(js|css))$/.test(asset)){res.writeHead(404);res.end();return;}
  res.writeHead(200,{"content-type":asset.endsWith(".js")?"text/javascript":asset.endsWith(".css")?"text/css":"text/html"});res.end(asset==="index.html"?index:await readFile(path.join(assets,asset)));
});

let browser;
try{
  server.listen(0,"127.0.0.1");await once(server,"listening");const origin=`http://127.0.0.1:${server.address().port}`;
  browser=await chromium.launch({headless:true,...(process.env.RJS_PLAYWRIGHT_CHANNEL?{channel:process.env.RJS_PLAYWRIGHT_CHANNEL}:{})});
  const page=await browser.newPage({locale:"en-US"});
  await page.goto(origin+"/admin/queues");
  await page.getByLabel("Username",{exact:true}).fill("alice");await page.getByLabel("Password",{exact:true}).fill("correct horse battery staple");await page.getByRole("button",{name:"Sign in",exact:true}).click();
  await expect(page).toHaveURL(origin+"/admin/tenants/alpha/queues");
  await page.locator(".session-controls summary").click();await page.locator("#active-tenant").selectOption("beta");
  await expect(page).toHaveURL(origin+"/admin/tenants/beta/queues");
  await expect.poll(()=>requests.filter(r=>r.path==="/api/v1/queues"&&r.tenant==="beta").length).toBeGreaterThan(0);
  const resources=requests.filter(r=>r.path==="/api/v1/queues");
  assert.ok(resources.some(r=>r.tenant==="alpha")&&resources.some(r=>r.tenant==="beta"));
  assert.ok(resources.every(r=>r.authorization==="Bearer local-access-token"&&["alpha","beta"].includes(r.tenant)));
  assert.ok(requests.filter(r=>["/api/v1/auth/login","/api/v1/session"].includes(r.path)).every(r=>r.tenant===""));
  console.log("Tenant URL/header isolation browser check passed");
}finally{await browser?.close();server.closeAllConnections();if(server.listening)await new Promise(resolve=>server.close(resolve));}
