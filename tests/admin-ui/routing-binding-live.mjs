// Called only by the isolated real-service harness; the fixture stays in its
// test-owned broker and is discarded when that broker is stopped.
export async function routingBindingChecks({page,api,origin,auditor,expect,assert,evidence,path,report}) {
  const name="live_routing_bindings",url=`/api/v1/queues/${name}`;
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{replicas:1,bindings:[
    {exchange:"direct_events",type:"direct",keys:["created"]},
    {exchange:"topic_events",type:"topic",keys:["orders.#"]},
    {exchange:"fanout_events",type:"fanout"}
  ]}};
  const created=await api(url,{method:"PUT",headers:{"If-None-Match":"*"},body:JSON.stringify(document)});
  assert.equal(created.status,200,await created.text());
  await page.goto(`${origin}/admin/queues/by-name/${name}?tab=routing`);
  await page.getByLabel("Bearer token",{exact:true}).fill(auditor);
  await page.getByRole("button",{name:"Verify identity",exact:true}).click();
  const panel=page.getByRole("region",{name:"Routing probe",exact:true});
  await panel.getByLabel("Probe mode",{exact:true}).selectOption("exchange");
  for(const [type,exchange,key]of [["direct","direct_events","created"],["topic","topic_events","orders"],["fanout","fanout_events",""]]){
    await panel.getByLabel("Exchange",{exact:true}).fill(exchange);
    await panel.getByLabel("Exchange type",{exact:true}).selectOption(type);
    if(type!=="fanout")await panel.getByLabel("Routing key",{exact:true}).fill(key);
    else await expect(panel.getByLabel("Routing key",{exact:true})).toBeDisabled();
    await panel.getByRole("button",{name:"Check routing",exact:true}).click();
    await expect(panel.getByRole("status")).toContainText("Generated Stream Subject matched");
    const rows=panel.getByRole("table",{name:"Binding translation and matches",exact:true}).locator("tbody tr");
    await expect(rows).toHaveCount(3);
    for(const other of ["direct_events","topic_events","fanout_events"]){
      const row=rows.filter({has:page.getByRole("cell",{name:other,exact:true})});
      if(other===exchange)await expect(row.locator("td").nth(4)).toContainText(`rjs.q.${name}.x.${exchange}.${type}`);
      else await expect(row.locator("td").nth(4)).toHaveText("None");
    }
  }
  await page.getByRole("button",{name:"简体中文",exact:true}).click();
  const chinese=page.getByRole("region",{name:"路由探测",exact:true});
  const chineseTable=chinese.getByRole("table",{name:"绑定翻译与匹配",exact:true});
  await expect(chineseTable).toBeVisible();
  await expect(chineseTable.getByRole("columnheader")).toHaveText(["交换机","类型","路由键","生成的 Subjects","匹配的 Subjects"]);
  await expect(chinese.getByLabel("交换机",{exact:true})).toHaveValue("fanout_events");
  await expect(page.getByRole("columnheader",{name:"交换机",exact:true})).toHaveCount(2);
  await expect(chinese.getByRole("status")).toContainText("这不是投递成功的证据");
  await page.setViewportSize({width:375,height:900});
  const bindingLayout=await chinese.locator(".table-scroll").evaluate(node=>({viewport:node.clientWidth,content:node.scrollWidth,table:node.querySelector("table").getBoundingClientRect().width}));
  assert.ok(bindingLayout.table>=880&&bindingLayout.content>bindingLayout.viewport,"binding table must scroll rather than squeeze text into narrow columns");
  await page.screenshot({path:path.join(evidence,"routing-bindings-mobile-zh.png"),fullPage:true});
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,"binding table page overflow");
  const scrollRegion=chinese.getByRole("region",{name:"绑定结果——可横向滚动",exact:true});
  await chinese.getByRole("button",{name:"检查路由",exact:true}).focus();
  await page.keyboard.press("Tab");await expect(scrollRegion).toBeFocused();
  for(let step=0;step<30;step++)await page.keyboard.press("ArrowRight");
  await expect.poll(()=>scrollRegion.evaluate(node=>node.scrollLeft+node.clientWidth>=node.scrollWidth-2)).toBe(true);
  await expect(scrollRegion).toBeFocused();
  await page.screenshot({path:path.join(evidence,"routing-bindings-keyboard-zh.png"),fullPage:true});
  await page.keyboard.press("Shift+Tab");
  await expect(chinese.getByRole("button",{name:"检查路由",exact:true})).toBeFocused();
  await page.setViewportSize({width:1440,height:1000});
  await page.getByRole("button",{name:"English",exact:true}).click();

  // Change the real saved declaration while the page holds its old ETag.
  const before=await api(url),oldETag=before.headers.get("ETag");
  document.metadata.labels={probe_revision:"changed"};
  const changed=await api(url,{method:"PUT",headers:{"If-Match":oldETag},body:JSON.stringify(document)});
  assert.equal(changed.status,200,await changed.text());
  await panel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(panel.getByRole("alert")).toContainText("Declaration changed");
  await expect(panel.locator("table,time")).toHaveCount(0);
  await page.getByRole("button",{name:"Refresh Queue page",exact:true}).click();
  await panel.getByLabel("Subject",{exact:true}).fill(`rjs.q.${name}.x.fanout_events.fanout`);
  await panel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(panel.getByRole("status")).toContainText("Generated Stream Subject matched");

  let release,entered=false;
  const gate=new Promise(resolve=>{release=resolve;});
  const hold=async route=>{entered=true;await gate;await route.continue().catch(()=>{});};
  const pattern=`**/api/v1/queues/${name}/routing-probe?*`;
  await page.route(pattern,hold,{times:1});
  try {
    await panel.getByRole("button",{name:"Check routing",exact:true}).click();await expect.poll(()=>entered).toBe(true);
    await panel.getByRole("button",{name:"Cancel",exact:true}).click();
  } finally { release(); }
  await expect(panel.locator("table,time")).toHaveCount(0);
  await page.unroute(pattern,hold);
  await panel.getByRole("button",{name:"Check routing",exact:true}).click();
  await expect(panel.getByRole("status")).toContainText("Generated Stream Subject matched");
  for(const [status,body,expected]of [
    [404,'{"error":{"code":"proxy_not_found"}}',"Invalid response"],
    [404,'<html>not found</html>',"Invalid response"],
    [409,'{"error":{"code":"proxy_conflict"}}',"Invalid response"],
    [409,'{"error":',"Invalid response"],
    [403,'<html>denied</html>',"Read denied"],
    [400,'<html>bad request</html>',"Invalid response"],
  ]){
    await page.route(pattern,route=>route.fulfill({status,body,contentType:"application/json"}),{times:1});
    await panel.getByRole("button",{name:"Check routing",exact:true}).click();
    await expect(panel.getByRole("alert")).toContainText(expected);
    await expect(panel.locator("table,time")).toHaveCount(0);
    await panel.getByRole("button",{name:"Check routing",exact:true}).click();
    await expect(panel.getByRole("status")).toContainText("Generated Stream Subject matched");
  }
  report.checks.push("routing-bindings-real-direct-topic-zero-token-fanout-isolation-bilingual-mobile");
  report.checks.push("routing-bindings-real-declaration-change-clears-refresh-recovers-cancel-retry");
  report.checks.push("routing-bindings-mobile-tab-arrow-scroll-last-column-and-focus-exit");
  report.checks.push("routing-proxy-unknown-and-malformed-errors-no-false-resource-conclusions-recovery");
}
