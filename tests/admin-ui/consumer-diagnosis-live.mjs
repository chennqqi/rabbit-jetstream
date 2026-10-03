import {parseJSON,stringifyJSON} from "../../admin-ui/src/api.mjs";

export async function consumerDiagnosisChecks({page,api,consumerView,stream,name,expect,assert,evidence,path,report}) {
  const endpoint=`/api/v1/streams/${stream}/consumers/${name}`,pattern=`**${endpoint}`;
  const realResponse=await api(endpoint);assert.equal(realResponse.status,200);
  const real=parseJSON(await realResponse.text());
  const nodeSnapshot=parseJSON(await (await api("/api/v1/nodes")).text()),configuredNode=nodeSnapshot.nodes.find(node=>node?.name&&node?.id);
  assert.ok(configuredNode,"owned configured node identity required");
  // The owned fixture omits MaxAckPending; the pinned broker applies 1000.
  assert.equal(real.max_ack_pending,1000);
  const panel=page.getByRole("region",{name:"Consumer backlog observations",exact:true});
  const refresh=consumerView.getByRole("button",{name:"Refresh Consumer",exact:true});
  await refresh.click();await expect(panel.locator("dt").filter({hasText:"Configured ACK-pending limit"}).locator("+ dd")).toHaveText("1000");
  const methods=[],observe=request=>{if(new URL(request.url()).pathname===endpoint)methods.push(request.method());};
  page.on("request",observe);
  try {
    const synthetic={...real,pending:18446744073709551615n,ack_pending:9007199254740993n,max_ack_pending:9007199254740993n,redelivered:1,waiting:0,cluster:{leader:configuredNode.name,replicas:[{name:"diagnostic-follower",current:false,offline:true,lag:9007199254740993n,active_nanos:1000000000}]}};
    await page.route(pattern,route=>route.fulfill({status:200,contentType:"application/json",body:stringifyJSON(synthetic)}),{times:1});
    await refresh.click();await expect(panel).toContainText("ACK-pending count meets or exceeds");
    await expect(panel).toContainText("zero waiting pull requests");await expect(panel).toContainText("The observation includes redelivery");
    await expect(panel.locator("dt").filter({hasText:/^Pending messages$/}).locator("+ dd")).toHaveText("18446744073709551615");
    await expect(panel.locator("dt").filter({hasText:/^ACK-pending messages$/}).locator("+ dd")).toHaveText("9007199254740993");
    const replicaHints=panel.getByRole("list",{name:"Consumer replica investigation hints",exact:true});
    await expect(replicaHints).toContainText("Follower reported offline");await expect(replicaHints).toContainText("Follower reported not current");await expect(replicaHints).toContainText("9007199254740993");
    const nodeCorrelation=panel.getByRole("region",{name:"Consumer peer node correlation",exact:true});
    await expect(nodeCorrelation.getByRole("row").filter({has:page.getByRole("rowheader",{name:configuredNode.name,exact:true})})).toContainText("Matched configured endpoint");
    await expect(nodeCorrelation.getByRole("row").filter({has:page.getByRole("rowheader",{name:"diagnostic-follower",exact:true})})).toContainText("Unresolved; absence not established");
    await page.getByRole("button",{name:"简体中文",exact:true}).click();await page.setViewportSize({width:375,height:900});
    const chinese=page.getByRole("region",{name:"Consumer 积压观测",exact:true});await expect(chinese).toContainText("待确认数达到或超过");
    await chinese.screenshot({path:path.join(evidence,"consumer-diagnosis-synthetic-mobile-zh.png")});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"Consumer diagnosis mobile overflow");
    const rawSummary=consumerView.getByText("原始观测（精确整数）",{exact:true});
    const replicaRegion=chinese.getByRole("region",{name:"副本观测",exact:true});
    assert.equal(await replicaRegion.evaluate(node=>node.scrollWidth>node.clientWidth),true,"localized mobile replica table must scroll internally");
    await expect(replicaRegion.getByRole("row").filter({has:page.getByRole("rowheader",{name:configuredNode.name,exact:true})}).locator("td").first()).toHaveText("Leader（主节点）");
    await expect(replicaRegion.getByRole("row").filter({has:page.getByRole("rowheader",{name:"diagnostic-follower",exact:true})}).locator("td").first()).toHaveText("Follower（副本节点）");
    assert.equal(await replicaRegion.getByRole("row").filter({has:page.getByRole("rowheader",{name:"diagnostic-follower",exact:true})}).locator("td").first().evaluate(node=>node.getBoundingClientRect().width>=100),true,"localized replica role must not collapse into a narrow column");
    await rawSummary.focus();await page.keyboard.press("Tab");await expect(replicaRegion).toBeFocused();
    for(let step=0;step<30;step++)await page.keyboard.press("ArrowRight");
    await expect.poll(()=>replicaRegion.evaluate(node=>node.scrollLeft+node.clientWidth>=node.scrollWidth-2)).toBe(true);
    const lastCell=replicaRegion.getByRole("row").filter({has:page.getByRole("rowheader",{name:"diagnostic-follower",exact:true})}).locator("td").last();
    await expect(lastCell).toHaveText("1000000000");
    assert.equal(await lastCell.evaluate(node=>{const cell=node.getBoundingClientRect(),region=node.closest('[role="region"]').getBoundingClientRect();return cell.left>=region.left-2&&cell.right<=region.right+2;}),true,"rightmost replica metric must be inside the scroll viewport");
    await replicaRegion.screenshot({path:path.join(evidence,"consumer-replica-keyboard-right-columns-zh.png")});
    await page.keyboard.press("Shift+Tab");await expect(rawSummary).toBeFocused();
    await page.getByRole("button",{name:"English",exact:true}).click();await page.setViewportSize({width:1440,height:1000});
    await page.route(pattern,route=>route.fulfill({status:200,contentType:"application/json",body:stringifyJSON(synthetic)}),{times:2});
    await page.route("**/api/v1/nodes",route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"monitoring_unavailable"}}'}),{times:1});
    await refresh.click();await expect(panel).toContainText("Node monitoring evidence is stale or its latest refresh failed");await expect(nodeCorrelation).toHaveCount(0);
    await refresh.click();await expect(panel.getByRole("region",{name:"Consumer peer node correlation",exact:true})).toBeVisible();
    const previousTime=await consumerView.locator("time").getAttribute("datetime");
    await page.route(pattern,route=>route.fulfill({status:503,contentType:"application/json",body:'{"error":{"code":"jetstream_unavailable"}}'}),{times:1});
    await refresh.click();await expect(panel).toContainText("Current diagnosis unavailable");await expect(panel.locator("dl")).toHaveCount(0);await expect(panel.locator("li")).toHaveCount(0);
    await expect(consumerView.locator("time")).toHaveAttribute("datetime",previousTime);
    await page.route(pattern,route=>route.fulfill({status:200,contentType:"application/json",body:stringifyJSON({...synthetic,max_ack_pending:"unknown"})}),{times:1});
    await refresh.click();await expect(panel).toContainText("Unavailable or unsupported evidence fields");await expect(panel).not.toContainText("ACK-pending count meets or exceeds");
    await page.route(pattern,route=>route.fulfill({status:200,contentType:"application/json",body:stringifyJSON({...synthetic,cluster:{leader:"duplicate",replicas:[{name:"duplicate",offline:true}]}})}),{times:1});
    await refresh.click();await expect(panel).toContainText("Invalid replica identity or topology response");await expect(replicaHints).toHaveCount(0);await expect(panel.getByRole("region",{name:"Replica observations",exact:true})).toHaveCount(0);
    await refresh.click();await expect(panel.locator("dt").filter({hasText:"Configured ACK-pending limit"}).locator("+ dd")).toHaveText("1000");
    assert.ok(methods.length>=4&&methods.every(method=>method==="GET"),"diagnosis must not mutate Consumers");
    report.checks.push("consumer-diagnosis-real-default-limit-synthetic-exact-thresholds-stale-clear-invalid-limit-recovery-mobile-read-only");
    report.checks.push("consumer-diagnosis-synthetic-replica-hints-exact-lag-duplicate-topology-rejected-no-health-inference");
    report.checks.push("consumer-diagnosis-independent-node-correlation-match-unresolved-failure-recovery");
    report.checks.push("consumer-replica-localized-accessible-name-roles-mobile-keyboard-entry-rightmost-metric-visible-and-exit");
  } finally {page.off("request",observe);await page.unroute(pattern);}
}
