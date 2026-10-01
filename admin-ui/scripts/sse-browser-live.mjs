import assert from "node:assert/strict";
import {chromium} from "playwright";

const origin=process.env.RJS_TEST_ORIGIN??"http://127.0.0.1:18223",token=process.env.RJS_TEST_TOKEN??"s4-visual-qa-token",tenant=process.env.RJS_TEST_TENANT??"local";
const browser=await chromium.launch({headless:true});
try{
  const page=await browser.newPage();let eventRequest,auditReads=0;
  page.on("request",request=>{const url=new URL(request.url());if(url.pathname==="/api/v1/events")eventRequest=request;if(url.pathname==="/api/v1/audit/windows")auditReads++;});
  await page.goto(`${origin}/admin/`);await page.locator(".recovery-login summary").click();await page.locator(".recovery-login input").fill(token);await page.locator(".recovery-login button").click();await page.locator("nav.primary-nav").waitFor();
  const connected=page.waitForRequest(request=>new URL(request.url()).pathname==="/api/v1/events"),loaded=page.waitForResponse(response=>new URL(response.url()).pathname==="/api/v1/audit/windows");await page.locator('nav.primary-nav a[href="/admin/audit"]').click();await Promise.all([connected,loaded]);
  assert.equal(new URL(eventRequest.url()).search,"");assert.equal(eventRequest.headers()["authorization"],`Bearer ${token}`);assert.ok([tenant,undefined].includes(eventRequest.headers()["x-rjs-tenant"]));
  const before=auditReads,name=`sse_browser_${Date.now()}`,requestID=`sse-browser-${Date.now()}`,document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:[`${name}.events`],replicas:1,storage:"file",retention:{maxMessages:100}}};const refreshed=page.waitForResponse(response=>new URL(response.url()).pathname==="/api/v1/audit/windows");
  const mutation=await fetch(`${origin}/api/v1/queues/${name}`,{method:"PUT",headers:{Authorization:`Bearer ${token}`,"X-RJS-Tenant":tenant,"Content-Type":"application/json","If-None-Match":"*","X-Request-ID":requestID},body:JSON.stringify(document)});assert.equal(mutation.status,200,await mutation.text());
  await refreshed;assert.ok(auditReads>before,`audit reads did not advance from ${before}`);assert.equal(await page.locator('[role="alert"]').count(),0);
  process.stdout.write(JSON.stringify({status:"passed",browser:"chromium",eventHeaders:eventRequest.headers()["x-rjs-tenant"]?"memory-bearer+explicit-tenant":"memory-bearer+implicit-single-tenant",auditReadsBefore:before,auditReadsAfter:auditReads,requestID})+"\n");
}finally{await browser.close();}
