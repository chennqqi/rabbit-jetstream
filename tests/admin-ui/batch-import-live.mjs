import {parseJSON,stringifyJSON} from "../../admin-ui/src/api.mjs";
import {readFile} from "node:fs/promises";

export async function batchImportChecks({page,api,origin,operator,expect,assert,evidence,path,report}){
  const endpoint="/api/v1/queues/import-plan",requests=[];
  const document=(name,dependency)=>({apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:[`${name}.events`],replicas:1,storage:"file",retention:{maxMessages:9223372036854775807n},...(dependency?{deadLetter:{queue:dependency}}:{})}});
  const upload=(value,name)=>({name:name??`${value.metadata.name}.json`,mimeType:"application/json",buffer:Buffer.from(stringifyJSON(value))});
  const observe=request=>{if(new URL(request.url()).pathname.startsWith("/api/v1/queues/")&&request.method()!=="GET")requests.push({path:new URL(request.url()).pathname,method:request.method(),body:request.postData(),headers:request.headers()});};
  page.on("request",observe);
  try{
    await page.goto(origin+"/admin/queues/new");await page.getByLabel("Bearer token",{exact:true}).fill(operator);await page.getByRole("button",{name:"Verify identity",exact:true}).click();
    const panel=page.getByRole("region",{name:"Batch import planning",exact:true}),files=panel.getByLabel("Queue JSON files",{exact:true}),plan=panel.getByRole("button",{name:"Plan import only",exact:true});
    await files.setInputFiles([upload(document("batch_source","batch_target")),{name:"bad.json",mimeType:"application/json",buffer:Buffer.from("{")}]);
    await expect(panel.getByRole("alert")).toContainText("None will be silently skipped");await expect(plan).toBeDisabled();assert.equal(requests.length,0);
    const source=document("batch_source","batch_target"),target=document("batch_target");
    await files.setInputFiles([upload(source,"source-"+"x".repeat(160)+".json"),upload(target)]);await expect(plan).toBeEnabled();assert.equal(requests.length,0);
    await plan.click();await expect(panel.getByRole("status")).toContainText("Internally ordered only");
    assert.equal(requests.length,1);assert.equal(requests[0].path,endpoint);assert.equal(requests[0].method,"POST");assert.ok(requests[0].headers["x-rjs-if-capabilities-match"]);
    const sent=parseJSON(requests[0].body);assert.deepEqual(Object.keys(sent),["documents"]);assert.equal(sent.documents[0].spec.retention.maxMessages,9223372036854775807n);
    const order=panel.getByRole("heading",{name:"Dependency-first order",exact:true}).locator("+ ol");await expect(order.locator("li").nth(0)).toContainText("batch_target");await expect(order.locator("li").nth(1)).toContainText("batch_source");
    for(const name of ["batch_source","batch_target"])assert.equal((await api(`/api/v1/queues/${name}`)).status,404,"planning must not create resources");
    await page.getByRole("button",{name:"简体中文",exact:true}).click();await page.setViewportSize({width:375,height:900});
    const chinese=page.getByRole("region",{name:"批次导入规划",exact:true});await chinese.screenshot({path:path.join(evidence,"batch-import-mobile-zh.png")});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"batch mobile overflow");
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
    await files.setInputFiles([upload(document("batch_external","live_imported")),upload(document("batch_missing","batch_absent")),upload(document("batch_duplicate"),"duplicate-1.json"),upload(document("batch_duplicate"),"duplicate-2.json"),upload({...document("batch_invalid"),unsupported:true})]);
    await expect(panel.getByText("Internally ordered only; destination preview and explicit apply are still required.",{exact:true})).toHaveCount(0);
    await plan.click();await expect(panel.getByRole("status")).toContainText("Some items are blocked");await expect(panel).toContainText("Duplicate Queue name");await expect(panel).toContainText("Invalid declaration");
    const external=panel.getByRole("heading",{name:"External declaration observations",exact:true}).locator("+ ul");await expect(external).toContainText("live_imported — Declaration present");await expect(external).toContainText("batch_absent — Declaration missing");await expect(order.locator("li")).toHaveCount(0);
    await page.route("**/api/v1/queues/import-plan",route=>route.fulfill({status:200,contentType:"application/json",body:"{"}));
    try{await plan.click();await expect(panel.getByRole("alert")).toContainText("Invalid planning evidence");await expect(order).toHaveCount(0);}finally{await page.unroute("**/api/v1/queues/import-plan");}
    await plan.click();await expect(panel.getByRole("status")).toContainText("Some items are blocked");
    assert.ok(requests.every(request=>request.path===endpoint&&request.method==="POST"),"batch UI must never apply resources");
    await panel.getByRole("button",{name:"Clear batch files and plan",exact:true}).click();await expect(plan).toBeDisabled();await expect(order).toHaveCount(0);
    report.checks.push("batch-import-local-selection-indexed-dependency-order-exact-values-no-creation-bilingual-mobile");
    report.checks.push("batch-import-duplicate-invalid-external-present-missing-blocked-invalid-response-clear-retry");
    await files.setInputFiles([upload(source),upload(target)]);await plan.click();await expect(panel.getByRole("status")).toContainText("Internally ordered only");
    const start=panel.getByRole("button",{name:"Prepare per-item review",exact:true});await expect(start).toBeDisabled();await panel.getByRole("checkbox").check();await start.click();
    const execution=page.getByRole("region",{name:"Batch execution",exact:true});
    const sourceButton=execution.getByRole("button",{name:"Prepare item: batch_source",exact:true});
    await expect(sourceButton).toBeDisabled();await execution.getByRole("button",{name:"Prepare item: batch_target",exact:true}).click();
    const preview=page.getByRole("button",{name:"Preview changes",exact:true}),apply=page.getByRole("button",{name:"Apply reviewed draft",exact:true});
    await expect(page.getByLabel("Queue document (JSON)",{exact:true})).toBeVisible();assert.equal(requests.filter(row=>row.method==="PUT").length,0);
    await preview.click();await expect(apply).toBeDisabled();assert.equal((await api("/api/v1/queues/batch_target")).status,404);
    await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    await expect(sourceButton).toBeEnabled();await sourceButton.click();await preview.click();await expect(apply).toBeDisabled();assert.equal((await api("/api/v1/queues/batch_source")).status,404);
    // A switch to the accepted prerequisite and back invalidates the dependent's preview.
    await execution.getByRole("button",{name:"Review item: batch_target",exact:true}).click();await execution.getByRole("button",{name:"Review item: batch_source",exact:true}).click();await expect(apply).toHaveCount(0);
    await page.getByRole("link",{name:"Queue list",exact:true}).click();await page.getByRole("link",{name:"Create Queue",exact:true}).click();await expect(execution).toBeVisible();await expect(page.getByLabel("Queue document (JSON)",{exact:true})).toBeVisible();
    assert.equal(parseJSON(await page.getByLabel("Queue document (JSON)",{exact:true}).inputValue()).metadata.name,"batch_source");
    await preview.click();await expect(apply).toBeDisabled();await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    const writes=requests.filter(row=>row.method==="PUT");assert.deepEqual(writes.map(row=>row.path),["/api/v1/queues/batch_target","/api/v1/queues/batch_source"]);assert.ok(writes.every(row=>row.headers["if-none-match"]==="*"&&!row.headers["if-match"]));
    for(const name of ["batch_target","batch_source"]){const saved=await api(`/api/v1/queues/${name}`);assert.equal(saved.status,200);assert.equal(parseJSON(await saved.text()).document.spec.retention.maxMessages,9223372036854775807n);}
    await page.getByRole("button",{name:"简体中文",exact:true}).click();await page.setViewportSize({width:375,height:900});await page.getByRole("region",{name:"批次执行",exact:true}).screenshot({path:path.join(evidence,"batch-execution-mobile-zh.png")});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"batch execution mobile overflow");
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
    await execution.getByRole("button",{name:"Keep batch and return to creation",exact:true}).click();await page.getByRole("button",{name:"Resume retained batch",exact:true}).click();await expect(execution).toContainText("Apply accepted; not health proof");
    report.checks.push("batch-execution-prerequisite-first-independent-confirmations-two-create-only-writes-exact-values");
    report.checks.push("batch-execution-switch-invalidates-preview-navigation-resume-accepted-results-bilingual-mobile");
    const archive=execution.getByRole("button",{name:"Archive batch and start another",exact:true}),beforeArchive=requests.length;
    page.once("dialog",dialog=>dialog.dismiss());await archive.click();await expect(execution).toBeVisible();assert.equal(requests.length,beforeArchive);
    page.once("dialog",dialog=>dialog.accept());await archive.click();await expect(execution).toHaveCount(0);assert.equal(requests.length,beforeArchive,"archival must not call Queue APIs");
    const history=page.getByRole("region",{name:"Archived batches",exact:true});await expect(history).toBeVisible();await history.locator("summary").first().click();await history.getByText("Read-only files, plan and outcome evidence",{exact:true}).click();
    const evidenceText=await history.locator("pre").innerText(),record=parseJSON(evidenceText);assert.equal(record.schema,"rjs.batch-execution-evidence.v1");assert.equal(record.scope,"archived-batch-not-write-authorization");assert.deepEqual(record.items.map(item=>item.outcome.phase),["accepted","accepted"]);assert.equal(record.items[0].document.spec.retention.maxMessages,9223372036854775807n);assert.ok(record.items.every(item=>item.outcome.requestId));
    const pendingEvidence=page.waitForEvent("download");await history.getByRole("link",{name:"Download itemized batch evidence JSON",exact:true}).click();const evidenceDownload=await pendingEvidence;
    const downloaded=parseJSON(await readFile(await evidenceDownload.path(),"utf8"));assert.equal(downloaded.schema,"rjs.batch-execution-evidence.v1");assert.equal(downloaded.items[0].document.spec.retention.maxMessages,9223372036854775807n);assert.ok(downloaded.limitations.some(value=>value.includes("never atomic")));assert.ok(downloaded.limitations.some(value=>value.includes("excludes messages")));
    await expect(history.locator("ol > li").first()).toContainText("Apply accepted; not health proof");
    await page.getByRole("link",{name:"Queue list",exact:true}).click();await page.getByRole("link",{name:"Create Queue",exact:true}).click();await expect(history).toBeVisible();
    await files.setInputFiles([upload(target),upload(document("batch_next"))]);await plan.click();await expect(panel.getByRole("status")).toContainText("Internally ordered only");await panel.getByRole("checkbox").check();await start.click();
    await execution.getByRole("button",{name:"Prepare item: batch_target",exact:true}).click();await expect(execution.getByRole("alert")).toContainText("already has a retained");assert.equal(requests.filter(row=>row.method==="PUT").length,2);
    await execution.getByRole("button",{name:"Prepare item: batch_next",exact:true}).click();await expect(page.getByLabel("Queue document (JSON)",{exact:true})).toBeVisible();assert.equal((await api("/api/v1/queues/batch_next")).status,404);
    page.once("dialog",dialog=>dialog.accept());await archive.click();await expect(history.locator(":scope > details")).toHaveCount(2);
    await history.locator(":scope > details").first().locator(":scope > summary").click();await history.getByText("Read-only files, plan and outcome evidence",{exact:true}).first().click();assert.equal(await history.locator("pre").first().innerText(),evidenceText,"earlier archive must remain unchanged");
    await history.locator(":scope > details").first().locator("details > summary").click();
    await page.getByRole("button",{name:"简体中文",exact:true}).click();await page.setViewportSize({width:375,height:900});await page.getByRole("region",{name:"已归档批次",exact:true}).screenshot({path:path.join(evidence,"batch-history-mobile-zh.png")});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"batch history mobile overflow");
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});assert.equal(requests.filter(row=>row.method==="PUT").length,2);
    report.checks.push("batch-archive-decline-confirm-no-io-versioned-download-exact-int64-limitations-navigation-new-batch-retained-name-mobile");
    await files.setInputFiles([upload(document("batch_external_child","batch_external_root")),upload(document("batch_external_root","batch_external_target"))]);await plan.click();await expect(panel.getByRole("status")).toContainText("Some items are blocked");
    await expect(order.locator("li")).toHaveCount(0);
    const reviewOrder=panel.getByRole("heading",{name:"Destination preview order — not approval",exact:true}).locator("+ ol");await expect(reviewOrder.locator("li").nth(0)).toContainText("batch_external_root");await expect(reviewOrder.locator("li").nth(1)).toContainText("batch_external_child");
    await expect(panel).toContainText("batch_external_target — Declaration missing");await panel.getByRole("checkbox").check();await start.click();
    const child=execution.getByRole("button",{name:"Prepare item: batch_external_child",exact:true});await expect(child).toBeDisabled();await execution.getByRole("button",{name:"Prepare item: batch_external_root",exact:true}).click();
    await preview.click();await expect(page.getByRole("alert").filter({hasText:"Operation failed."})).toBeVisible();await expect(apply).toHaveCount(0);assert.equal((await api("/api/v1/queues/batch_external_root")).status,404);assert.equal(requests.filter(row=>row.method==="PUT").length,2);
    // Explicit fixture repair in the harness-owned broker, not a UI side effect.
    const repaired=await api("/api/v1/queues/batch_external_target",{method:"PUT",headers:{"If-None-Match":"*"},body:stringifyJSON(document("batch_external_target"))});assert.equal(repaired.status,200);
    await preview.click();await expect(apply).toBeDisabled();assert.equal((await api("/api/v1/queues/batch_external_root")).status,404);await expect(child).toBeDisabled();
    await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();await expect(child).toBeEnabled();await child.click();await preview.click();await expect(apply).toBeDisabled();
    await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();await expect(page.getByText(/^Apply accepted\./)).toBeVisible();
    const externalWrites=requests.filter(row=>row.method==="PUT").slice(2);assert.deepEqual(externalWrites.map(row=>row.path),["/api/v1/queues/batch_external_root","/api/v1/queues/batch_external_child"]);assert.ok(externalWrites.every(row=>row.headers["if-none-match"]==="*"&&!row.headers["if-match"]));
    for(const name of ["batch_external_root","batch_external_child"]){const saved=await api(`/api/v1/queues/${name}`);assert.equal(saved.status,200);assert.equal(parseJSON(await saved.text()).document.spec.retention.maxMessages,9223372036854775807n);}
    report.checks.push("batch-external-missing-preview-rejected-no-put-target-repaired-fresh-preview-separate-confirmation");
    report.checks.push("batch-external-transitive-prerequisite-gate-two-create-only-writes-exact-values");
    page.once("dialog",dialog=>dialog.accept());await archive.click();
    await files.setInputFiles([upload(document("batch_unknown_child","batch_unknown_root")),upload(document("batch_unknown_root"))]);await plan.click();await expect(panel.getByRole("status")).toContainText("Internally ordered only");await panel.getByRole("checkbox").check();await start.click();
    const unknownChild=execution.getByRole("button",{name:"Prepare item: batch_unknown_child",exact:true});await execution.getByRole("button",{name:"Prepare item: batch_unknown_root",exact:true}).click();await preview.click();await expect(apply).toBeDisabled();
    let intercepted=0;
    await page.route("**/api/v1/queues/batch_unknown_root",async route=>{
      if(route.request().method()!=="PUT")return route.continue();
      intercepted++;const response=await route.fetch();assert.equal(response.status(),200);
      await route.fulfill({response,contentType:"application/json",body:"{"});
    });
    try{await page.getByRole("checkbox",{name:"I reviewed this preview and authorize applying this draft.",exact:true}).check();await apply.click();await expect(page.getByRole("alert").filter({hasText:"Write outcome unknown."})).toBeVisible();}finally{await page.unroute("**/api/v1/queues/batch_unknown_root");}
    assert.equal(intercepted,1);assert.equal((await api("/api/v1/queues/batch_unknown_root")).status,200);assert.equal((await api("/api/v1/queues/batch_unknown_child")).status,404);await expect(unknownChild).toBeDisabled();await expect(archive).toBeDisabled();await expect(apply).toHaveCount(0);
    const requestCode=page.locator("p").filter({hasText:"Request ID (not an idempotency key)"}).locator("code");const unknownRequest=await requestCode.innerText();assert.ok(unknownRequest);
    await page.getByRole("button",{name:"Inspect current state (read-only)",exact:true}).click();await expect(page.getByRole("heading",{name:"Current evidence — not outcome attribution",exact:true})).toBeVisible();await expect(archive).toBeDisabled();await expect(unknownChild).toBeDisabled();await expect(apply).toHaveCount(0);await expect(requestCode).toHaveText(unknownRequest);
    const inspection=page.getByRole("region",{name:"Current evidence — not outcome attribution",exact:true});
    for(const source of ["declaration","consumers","audit"]){
      const row=inspection.locator(":scope > div").filter({has:page.getByRole("heading",{name:source,exact:true})});await expect(row.locator(":scope > p")).toHaveText(/^available · /);
    }
    await page.getByRole("link",{name:"Queue list",exact:true}).click();await page.getByRole("link",{name:"Create Queue",exact:true}).click();await expect(page.getByRole("alert").filter({hasText:"Write outcome unknown."})).toBeVisible();await expect(requestCode).toHaveText(unknownRequest);await expect(archive).toBeDisabled();await expect(unknownChild).toBeDisabled();
    assert.equal(requests.filter(row=>row.method==="PUT"&&row.path==="/api/v1/queues/batch_unknown_root").length,1);assert.equal(requests.filter(row=>row.method==="PUT"&&row.path==="/api/v1/queues/batch_unknown_child").length,0);
    report.checks.push("batch-unknown-committed-write-malformed-response-no-retry-no-dependent-no-archive-inspection-navigation-retains-request");
    for(const [name,dependency] of [["cycle_a",undefined],["cycle_b","cycle_a"],["cycle_c","cycle_b"]]){
      const created=await api(`/api/v1/queues/${name}`,{method:"PUT",headers:{"If-None-Match":"*"},body:stringifyJSON(document(name,dependency))});assert.equal(created.status,200);
    }
    const beforeResponse=await api("/api/v1/queues/cycle_a"),before=parseJSON(await beforeResponse.text()),etag=beforeResponse.headers.get("etag");assert.ok(etag);
    const closed=document("cycle_a","cycle_c");
    for(const [method,suffix] of [["POST","/preview"],["PUT",""]]){
      const rejected=await api(`/api/v1/queues/cycle_a${suffix}`,{method,headers:{"If-Match":etag},body:stringifyJSON(closed)});assert.equal(rejected.status,400);const rejection=await rejected.json();assert.equal(rejection.error.code,"dlq_dependency_cycle");assert.match(rejection.error.message,/DLQ dependency cycle/);
      const after=await api("/api/v1/queues/cycle_a");assert.equal(after.status,200);assert.equal(after.headers.get("etag"),etag);assert.equal(stringifyJSON(parseJSON(await after.text()).document),stringifyJSON(before.document));
    }
    report.checks.push("dlq-transitive-cycle-real-chain-preview-and-put-rejected-original-declaration-etag-unchanged");
    const cycleContext=await page.context().browser().newContext({locale:"en-US",viewport:{width:1440,height:1000}}),cycleRequests=[];
    try{
      const cyclePage=await cycleContext.newPage();
      cyclePage.on("request",request=>{if(new URL(request.url()).pathname.startsWith("/api/v1/queues/cycle_a")&&request.method()!=="GET")cycleRequests.push(request.method());});
      await cyclePage.goto(origin+"/admin/queues/by-name/cycle_a/edit");
      await cyclePage.getByLabel("Bearer token",{exact:true}).fill(operator);await cyclePage.getByRole("button",{name:"Verify identity",exact:true}).click();
      const cycleDraft=cyclePage.getByLabel("Queue document (JSON)",{exact:true});await expect(cycleDraft).not.toHaveValue("");
      await cycleDraft.fill(stringifyJSON(closed));await cyclePage.getByRole("button",{name:"Preview changes",exact:true}).click();
      await expect(cyclePage.getByText("DLQ dependency cycle detected.",{exact:false})).toBeVisible();
      await expect(cyclePage.getByRole("button",{name:"Apply reviewed draft",exact:true})).toHaveCount(0);
      await expect(cycleDraft).toHaveValue(stringifyJSON(closed));await expect(cycleDraft).toBeEnabled();
      await cyclePage.getByRole("button",{name:"简体中文",exact:true}).click();await cyclePage.setViewportSize({width:375,height:900});
      await expect(cyclePage.getByText("检测到 DLQ 依赖循环。",{exact:false})).toBeVisible();
      await cyclePage.screenshot({path:path.join(evidence,"dlq-cycle-preview-mobile-zh.png"),fullPage:true});
      assert.equal(await cyclePage.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"cycle diagnostic mobile overflow");
      assert.deepEqual(cycleRequests,["POST"]);
      const unchanged=await api("/api/v1/queues/cycle_a");assert.equal(unchanged.headers.get("etag"),etag);
      assert.equal(stringifyJSON(parseJSON(await unchanged.text()).document),stringifyJSON(before.document));
      report.checks.push("dlq-cycle-preview-bilingual-actionable-diagnostic-draft-retained-no-browser-put-mobile");
    }finally{await cycleContext.close();}
  }finally{page.off("request",observe);}
}
