const {test,expect}=require("@playwright/test");
const AxeBuilder=require("@axe-core/playwright").default;

const queueName="browser_e2e",token="browser-test-token",deploymentProfile=process.env.RJS_EXPECTED_DEPLOYMENT_PROFILE??"standalone";
if(!["standalone","cluster"].includes(deploymentProfile))throw new Error("Unsupported expected deployment profile");

async function authenticate(page,value=token) {
  await page.goto("/admin/");
  await page.locator(".recovery-login summary").click();
  await page.getByLabel("Recovery bearer token",{exact:true}).fill(value);
  const sessionResponse=page.waitForResponse(response=>new URL(response.url()).pathname==="/api/v1/session");
  await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
  const response=await sessionResponse;
  if(value===token)expect(response.status(),await response.text()).toBe(200);
  await page.waitForTimeout(100);
}

async function removeFixture(request) {
  const current=await request.get(`/api/v1/queues/${queueName}`,{headers:{Authorization:`Bearer ${token}`}});
  if(!current.ok())return;
  const removed=await request.delete(`/api/v1/queues/${queueName}?force=true`,{headers:{Authorization:`Bearer ${token}`,"If-Match":current.headers().etag,"X-RJS-Confirm-Queue":queueName}});
  expect(removed.ok(),await removed.text()).toBeTruthy();
}

async function navigate(page,name) {
  const navigation=page.getByRole("navigation",{name:"Primary navigation",exact:true});
  const toggle=page.getByRole("button",{name:"Navigation menu",exact:true});
  if(await toggle.isVisible())await toggle.click();
  await navigation.getByRole("link",{name,exact:true}).click();
}

test.beforeEach(async({page,request})=>{
  await removeFixture(request);
  await authenticate(page);
  await expect(page.getByRole("navigation",{name:"Primary navigation",exact:true})).toBeVisible();
  await navigate(page,"Overview");
  await expect(page.getByRole("heading",{name:"Overview",exact:true})).toBeVisible();
});

test.afterEach(async({request})=>{await removeFixture(request);});

test("creates, updates, finds and deletes a Queue through reviewed workflows",async({page,request})=>{
  test.slow();
  await navigate(page,"Access and settings");
  const capabilities=page.getByRole("region",{name:"Server capabilities",exact:true});
  await expect(capabilities.getByText(deploymentProfile,{exact:true})).toBeVisible();
  await expect(capabilities.getByText("configuration",{exact:true})).toBeVisible();
  await navigate(page,"Create Queue");
  await page.getByLabel("New Queue name",{exact:true}).fill(queueName);
  await page.getByLabel("Subjects (one per line)",{exact:true}).fill(`${queueName}.events`);
  const requestedReplicas=deploymentProfile==="cluster"?"3":"1";
  await page.getByLabel("Requested replicas",{exact:true}).selectOption(requestedReplicas);
  await page.getByLabel("Storage type",{exact:true}).selectOption("file");
  await page.getByLabel("Maximum stored messages",{exact:true}).fill("100");
  await page.getByRole("button",{name:"Prepare creation draft",exact:true}).click();
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
  expect((await request.get(`/api/v1/queues/${queueName}`,{headers:{Authorization:`Bearer ${token}`}})).status()).toBe(404);
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();
  await page.getByRole("button",{name:"Apply reviewed draft",exact:true}).click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
  const created=await request.get(`/api/v1/queues/${queueName}`,{headers:{Authorization:`Bearer ${token}`}});
  expect(created.status(),await created.text()).toBe(200);
  expect((await created.json()).document.spec.replicas).toBe(Number(requestedReplicas));

  await navigate(page,"Queue list");
  await expect(page.getByRole("link",{name:queueName,exact:true})).toBeVisible();
  await page.getByRole("link",{name:queueName,exact:true}).click();
  await page.getByRole("link",{name:"Edit draft and preview",exact:true}).click();
  const draft=page.getByLabel("Queue document (JSON)",{exact:true});
  const document=JSON.parse(await draft.inputValue());document.spec.retention.maxMessages=200;
  await draft.fill(JSON.stringify(document));
  await page.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(page.getByRole("region",{name:"Declaration change review",exact:true})).toContainText("spec.retention.maxMessages");
  await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();
  await page.getByRole("button",{name:"Apply reviewed draft",exact:true}).click();
  await expect(page.getByText(/^Apply accepted\./)).toBeVisible();

  page.once("dialog",dialog=>dialog.accept());
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();
  await authenticate(page);
  await navigate(page,"Queue list");
  await page.getByRole("link",{name:queueName,exact:true}).click();
  await page.getByRole("link",{name:"Review deletion impact",exact:true}).click();
  await page.getByRole("button",{name:"Read deletion preflight",exact:true}).click();
  await page.getByLabel("Type exact Queue name",{exact:true}).fill(queueName);
  await page.getByLabel("I reviewed this impact and understand that deletion is destructive.",{exact:true}).check();
  const deleteResponse=page.waitForResponse(response=>response.request().method()==="DELETE"&&new URL(response.url()).pathname===`/api/v1/queues/${queueName}`);
  await page.getByRole("button",{name:"Delete this Queue",exact:true}).click();
  const deleted=await deleteResponse;
  expect(deleted.status(),await deleted.text()).toBe(200);
  await expect(page.getByText(/^Server acknowledged deletion\./)).toBeVisible();
  await expect.poll(async()=>(await request.get(`/api/v1/queues/${queueName}`,{headers:{Authorization:`Bearer ${token}`}})).status()).toBe(404);
});

test("keeps bearer credentials in memory and reports authorization failures",async({page})=>{
  page.once("dialog",dialog=>dialog.accept());
  await page.getByRole("button",{name:"Clear local session",exact:true}).click();
  await authenticate(page,"invalid-token");
  await expect(page.getByRole("alert")).toContainText(/identity|authorization|credential|401/i);
  expect(await page.evaluate(()=>({local:Object.values(localStorage),session:Object.values(sessionStorage),cookies:document.cookie}))).toEqual({local:[],session:[],cookies:""});
  await page.reload();
  await page.locator(".recovery-login summary").click();
  await expect(page.getByLabel("Recovery bearer token",{exact:true})).toHaveValue("");
});

test("distinguishes an unavailable Node collection on a narrow viewport",async({page})=>{
  await page.setViewportSize({width:390,height:844});
  await page.route("**/api/v1/nodes**",route=>route.fulfill({status:503,contentType:"application/json",body:JSON.stringify({error:{code:"monitoring_unavailable",message:"Monitoring unavailable"}})}));
  await navigate(page,"Node list");
  await expect(page.getByRole("heading",{name:"Nodes",exact:true})).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("unavailable");
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.getByRole("button",{name:"Refresh nodes",exact:true})).toBeVisible();
});

test("has no serious or critical automated accessibility violations after authentication",async({page})=>{
  const results=await new AxeBuilder({page}).withTags(["wcag2a","wcag2aa"]).analyze();
  const blocking=results.violations.filter(item=>["serious","critical"].includes(item.impact));
  expect(blocking,blocking.map(item=>`${item.id}: ${item.help}`).join("\n")).toEqual([]);
});
