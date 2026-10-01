import {parseJSON} from "../../admin-ui/src/api.mjs";

export async function queueTemplateChecks({page,api,origin,operator,expect,assert,evidence,path,report}){
  const name="live_template_priority",endpoint=`/api/v1/queues/${name}`,requests=[];
  const observe=request=>{if(new URL(request.url()).pathname.startsWith(endpoint))requests.push({method:request.method(),none:request.headers()["if-none-match"]});};
  page.on("request",observe);
  try{
    await page.goto(origin+"/admin/queues/new");
    await page.locator(".recovery-login summary").click();
    await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);
    await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
    await page.getByLabel("New Queue name",{exact:true}).fill(name);
    await page.getByLabel("Subjects (one per line)",{exact:true}).fill("template.priority.events");
    await page.getByLabel("Requested replicas",{exact:true}).selectOption("1");
    await page.getByLabel("Storage type",{exact:true}).selectOption("file");
    await page.getByLabel("Maximum stored messages",{exact:true}).fill("9007199254740993");
    const select=page.getByLabel("Template",{exact:true});
    await select.selectOption("dead-letter");
    await page.getByLabel("Existing DLQ Queue name",{exact:true}).fill("not_created_by_template");
    await page.getByText("Review generated template document",{exact:true}).click();
    const panel=page.getByRole("group",{name:"Declaration templates (local drafts)",exact:true});
    let document=parseJSON(await panel.locator("pre").textContent());
    assert.equal(document.spec.deadLetter.queue,"not_created_by_template");
    await select.selectOption("priority");
    await page.getByLabel("Maximum priority (1–255)",{exact:true}).fill("2");
    document=parseJSON(await panel.locator("pre").textContent());
    assert.equal(document.spec.deadLetter,undefined);assert.equal(document.spec.maxPriority,2);
    assert.equal(document.spec.retention.maxMessages,9007199254740993n);
    assert.equal(requests.length,0,"template review must be local only");
    await page.getByRole("button",{name:"简体中文",exact:true}).click();
    await page.setViewportSize({width:375,height:900});
    await page.getByRole("group",{name:"声明模板（本地草稿）",exact:true}).screenshot({path:path.join(evidence,"queue-templates-mobile-zh.png")});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
    await page.getByRole("button",{name:"Prepare creation draft",exact:true}).click();
    const draft=page.getByLabel("Queue document (JSON)",{exact:true});
    await expect(draft).toBeVisible();assert.deepEqual(parseJSON(await draft.inputValue()),document);
    assert.equal(requests.length,0);assert.equal((await api(endpoint)).status,404);
    await page.getByRole("button",{name:"Preview changes",exact:true}).click();
    await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
    const apply=page.getByRole("button",{name:"Apply reviewed draft",exact:true});await expect(apply).toBeDisabled();
    await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();
    await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    const saved=await api(endpoint);assert.equal(saved.status,200);const result=parseJSON(await saved.text());
    assert.equal(result.document.spec.maxPriority,2);assert.equal(result.document.spec.deadLetter,undefined);
    assert.equal(result.document.spec.retention.maxMessages,9007199254740993n);
    assert.equal((await api("/api/v1/queues/not_created_by_template")).status,404);
    assert.deepEqual(requests.map(r=>r.method),["POST","PUT"]);assert.ok(requests.every(r=>r.none==="*"));
    await page.getByRole("button",{name:"Create another Queue",exact:true}).click();
    await expect(select).toHaveValue("");
    report.checks.push("queue-templates-local-review-switch-no-field-leak-mobile-exact-priority-explicit-create-only-reset");
  }finally{page.off("request",observe);}
}
