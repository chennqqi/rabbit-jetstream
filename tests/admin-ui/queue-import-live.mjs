import {parseJSON,stringifyJSON} from "../../admin-ui/src/api.mjs";

export async function queueImportChecks({page,api,origin,operator,expect,assert,evidence,path,report}){
  const name="live_imported",endpoint=`/api/v1/queues/${name}`;
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name,labels:{owner:"import-fixture"}},spec:{replicas:1,storage:"file",subjects:["live_imported.events"],maxPriority:0,retention:{maxMessages:9223372036854775807n},unsupported_import_field:true}};
  const requests=[];
  const observe=request=>{const url=new URL(request.url());if(url.pathname===endpoint||url.pathname===endpoint+"/preview")requests.push({method:request.method(),none:request.headers()["if-none-match"],match:request.headers()["if-match"]});};
  page.on("request",observe);
  try{
    await page.goto(origin+"/admin/queues/new");
    await page.locator(".recovery-login summary").click();await page.getByLabel("Recovery bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify recovery token",exact:true}).click();
    const panel=page.getByRole("region",{name:"Import Queue declaration",exact:true});
    const file=panel.getByLabel("Queue JSON file",{exact:true});
    const prepare=panel.getByRole("button",{name:"Prepare imported creation draft",exact:true});
    await page.getByLabel("New Queue name",{exact:true}).fill("keep_this_form");
    await file.setInputFiles({name:"bad.json",mimeType:"application/json",buffer:Buffer.from("{")});
    await expect(panel.getByRole("alert")).toContainText("Invalid UTF-8 JSON");
    await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("keep_this_form");
    await file.setInputFiles({name:"queue.json",mimeType:"application/json",buffer:Buffer.from(stringifyJSON(document))});
    await expect(panel).toContainText("Loaded file: queue.json");
    await expect(prepare).toBeDisabled();await panel.getByRole("checkbox").check();await expect(prepare).toBeEnabled();
    assert.equal(requests.length,0,"file selection must not read or write the Queue API");
    await prepare.click();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();
    await expect(page.getByLabel("New Queue name",{exact:true})).toHaveValue("keep_this_form");await expect(prepare).toBeVisible();
    await page.getByRole("button",{name:"简体中文",exact:true}).click();
    const chinese=page.getByRole("region",{name:"导入 Queue 声明",exact:true});
    await page.setViewportSize({width:375,height:900});await chinese.screenshot({path:path.join(evidence,"queue-import-mobile-zh.png")});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"import mobile overflow");
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
    await prepare.click();await page.getByRole("alertdialog").getByRole("button",{name:"Confirm",exact:true}).click();
    const draft=page.getByLabel("Queue document (JSON)",{exact:true});await expect(draft).toBeVisible();await expect(draft).toBeFocused();
    assert.equal(parseJSON(await draft.inputValue()).spec.retention.maxMessages,9223372036854775807n);
    assert.equal(requests.length,0,"draft preparation must not dispatch Queue IO");
    assert.equal((await api(endpoint)).status,404);
    await page.getByRole("button",{name:"Preview changes",exact:true}).click();
    await expect(page.getByRole("heading",{name:"Server preview validation",exact:true})).toBeVisible();
    await expect(page.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
    assert.equal(requests.length,1);assert.equal(requests[0].method,"POST");assert.equal(requests[0].none,"*");assert.equal(requests[0].match,undefined);
    delete document.spec.unsupported_import_field;await draft.fill(stringifyJSON(document));
    await page.getByRole("button",{name:"Preview changes",exact:true}).click();
    await expect(page.getByRole("heading",{name:"Advisory preview — not applied",exact:true})).toBeVisible();
    assert.equal((await api(endpoint)).status,404);
    const apply=page.getByRole("button",{name:"Apply reviewed draft",exact:true});await expect(apply).toBeDisabled();
    await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();
    await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    const saved=await api(endpoint);assert.equal(saved.status,200);const result=parseJSON(await saved.text());
    assert.equal(result.document.spec.retention.maxMessages,9223372036854775807n);assert.equal(result.document.spec.maxPriority,0);assert.equal(result.document.metadata.labels.owner,"import-fixture");
    const writes=requests.filter(row=>row.method==="PUT");assert.equal(writes.length,1);assert.equal(writes[0].none,"*");assert.equal(writes[0].match,undefined);
    await page.getByRole("button",{name:"Create another Queue",exact:true}).click();
    await file.setInputFiles({name:"same.json",mimeType:"application/json",buffer:Buffer.from(stringifyJSON(document))});await panel.getByRole("checkbox").check();await prepare.click();
    await expect(panel.getByRole("alert")).toContainText("retained request");assert.equal(requests.filter(row=>row.method==="PUT").length,1);
    report.checks.push("queue-import-real-file-invalid-preserves-form-discard-confirmation-local-only-bilingual-mobile");
    report.checks.push("queue-import-server-rejects-unknown-spec-repreview-explicit-create-only-exact-values-retained-name");
  }finally{page.off("request",observe);}
}
