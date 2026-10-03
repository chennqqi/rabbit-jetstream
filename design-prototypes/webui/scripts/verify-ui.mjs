import { chromium, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import sharp from "sharp";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

const root = process.cwd(),
  output = path.join(root, "evidence");
const target = "http://127.0.0.1:18224/";
const reference = path.resolve(
  root,
  "../../docs/design/webui/queue-detail-selected.png",
);
await mkdir(output, { recursive: true });
const report = {
  target,
  reference,
  viewport: { width: 1487, height: 1058 },
  deviceScaleFactor: 1,
  checks: [],
  consoleErrors: [],
  externalRequests: [],
  a11y: [],
};
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: report.viewport,
  deviceScaleFactor: 1,
});
await context.route("**/*", async (route) => {
  const url = route.request().url();
  if (url.startsWith(target) || url.startsWith("data:")) await route.continue();
  else {
    report.externalRequests.push(url);
    await route.abort();
  }
});
const page = await context.newPage();
page.on("pageerror", (e) => report.consoleErrors.push(e.message));
page.on("console", (m) => {
  if (m.type() === "error") report.consoleErrors.push(m.text());
});
async function check(name, fn) {
  await fn();
  report.checks.push(name);
}
async function screenshot(name, fullPage = false) {
  await page.evaluate(() => document.fonts.ready);
  if (fullPage) await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: path.join(output, name), fullPage });
}
async function a11y(state) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  report.a11y.push({
    state,
    violations: results.violations.map((v) => ({
      id: v.id,
      impact: v.impact,
      description: v.description,
      nodes: v.nodes.map((n) => n.target),
    })),
  });
}
try {
  await page.goto(target);
  await expect(
    page.getByRole("heading", { name: "orders_events", exact: true }),
  ).toBeVisible();
  await screenshot("desktop.png");
  report.geometry = await page.evaluate(() =>
    Object.fromEntries(
      [
        ".sidebar",
        ".topbar",
        ".resource-heading",
        ".warning",
        ".tabs",
        ".evidence-panel",
        ".config-panel",
        ".diagnostic-strip",
      ].map((s) => {
        const r = document.querySelector(s).getBoundingClientRect();
        return [s, { x: r.x, y: r.y, width: r.width, height: r.height }];
      }),
    ),
  );
  await sharp({
    create: { width: 2974, height: 1058, channels: 3, background: "#fff" },
  })
    .composite([
      { input: reference, left: 0, top: 0 },
      { input: path.join(output, "desktop.png"), left: 1487, top: 0 },
    ])
    .png()
    .toFile(path.join(output, "comparison.png"));
  const sourceCrop = await sharp(reference)
    .extract({ left: 280, top: 350, width: 1180, height: 570 })
    .toBuffer();
  const renderedCrop = await sharp(path.join(output, "desktop.png"))
    .extract({ left: 280, top: 350, width: 1180, height: 570 })
    .toBuffer();
  await sharp({
    create: { width: 2360, height: 570, channels: 3, background: "#fff" },
  })
    .composite([
      { input: sourceCrop, left: 0, top: 0 },
      { input: renderedCrop, left: 1180, top: 0 },
    ])
    .png()
    .toFile(path.join(output, "comparison-detail.png"));
  await a11y("desktop overview");
  await check(
    "complete same-data fixture and unknown replica lag",
    async () => {
      await expect(page.locator(".metrics strong")).toHaveText([
        "12,480",
        "8,420",
        "240",
      ]);
      await expect(page.locator(".replica-table tbody tr")).toHaveCount(3);
      await expect(
        page.locator(".replica-table tbody tr").last().locator("td").last(),
      ).toHaveText("—");
      await expect(page.locator(".config-panel dl>div")).toHaveCount(6);
    },
  );
  await check(
    "five tabs, empty event state, routing and consumer filter",
    async () => {
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(page.getByText("当前会话尚无配置变更。")).toBeVisible();
      await page.getByRole("tab", { name: "路由", exact: true }).click();
      await expect(
        page.getByText("orders.events", { exact: true }).first(),
      ).toBeVisible();
      await page.getByRole("tab", { name: "消费者", exact: true }).click();
      await page.getByRole("searchbox").fill("absent");
      await expect(page.getByText("没有匹配的 Consumer。")).toBeVisible();
      await page.getByRole("searchbox").fill("orders");
      await expect(page.getByText("8,420", { exact: true })).toBeVisible();
      await page.getByRole("tab", { name: "配置", exact: true }).click();
      await expect(page.locator(".tab-surface")).toContainText("24h");
      await page.getByRole("tab", { name: "概况", exact: true }).click();
    },
  );
  await check("keyboard tab selection", async () => {
    await page.getByRole("tab", { name: "概况", exact: true }).focus();
    await page.keyboard.press("ArrowRight");
    await expect(
      page.getByRole("tab", { name: "配置", exact: true }),
    ).toBeFocused();
    await page.keyboard.press("Home");
    await expect(
      page.getByRole("tab", { name: "概况", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
  });
  await check("refresh loading and unchanged offline state", async () => {
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "刷新中", exact: true }),
    ).toBeDisabled();
    await expect(
      page.getByRole("button", { name: "刷新", exact: true }),
    ).toBeEnabled();
    await expect(page.locator(".warning")).toContainText("nats-3 离线");
  });
  await check(
    "invalid edit, review diff, mock apply and session audit",
    async () => {
      await page.getByRole("button", { name: "编辑配置", exact: true }).click();
      await page.locator("input[name=ackWait]").fill("bad");
      await page.getByRole("button", { name: "复核修改", exact: true }).click();
      await expect(page.locator("input[name=ackWait]")).toHaveAttribute(
        "aria-invalid",
        "true",
      );
      await page.locator("input[name=ackWait]").fill("45s");
      await page.getByRole("button", { name: "复核修改", exact: true }).click();
      await expect(page.getByRole("dialog")).toContainText("30s");
      await expect(page.getByRole("dialog")).toContainText("45s");
      await screenshot("review.png");
      await a11y("edit review");
      await page
        .getByRole("button", { name: "应用模拟修改", exact: true })
        .click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.locator(".revision")).toContainText("13");
      await expect(page.locator(".config-panel")).toContainText("45s");
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(
        page.getByText("queue.apply", { exact: true }),
      ).toBeVisible();
      await page.getByRole("tab", { name: "概况", exact: true }).click();
    },
  );
  await check("discard confirmation and focus restoration", async () => {
    const edit = page.getByRole("button", { name: "编辑配置", exact: true });
    await edit.click();
    await page.locator("input[name=retention]").fill("48h");
    await page.keyboard.press("Escape");
    await expect(
      page.getByRole("heading", { name: "放弃修改？", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "继续编辑", exact: true }).click();
    await expect(page.locator("input[name=retention]")).toHaveValue("48h");
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await page.getByRole("button", { name: "放弃修改", exact: true }).click();
    await expect(edit).toBeFocused();
    await expect(page.locator(".config-panel")).toContainText("24h");
  });
  await check("navigation scope and Queue return path", async () => {
    await page.getByRole("button", { name: "节点", exact: true }).click();
    await expect(page.getByRole("dialog")).toContainText("尚未实现");
    await page.keyboard.press("Escape");
    await page
      .locator(".breadcrumbs")
      .getByRole("button", { name: "Queues", exact: true })
      .click();
    await page.getByRole("button", { name: "查看详情", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "orders_events", exact: true }),
    ).toBeVisible();
  });
  await check("English interface", async () => {
    await page.locator(".language select").selectOption("en");
    await expect(
      page.getByRole("tab", { name: "Overview", exact: true }),
    ).toBeVisible();
    await screenshot("english.png");
    await a11y("English overview");
    await page.locator(".language select").selectOption("zh");
  });
  await page.reload();
  await check("reload discards synthetic changes", async () => {
    await expect(page.locator(".revision")).toContainText("12");
    await expect(page.locator(".config-panel")).toContainText("30s");
  });
  await page.setViewportSize({ width: 375, height: 844 });
  await screenshot("mobile.png", true);
  await a11y("375px overview");
  await check("375px layout and menu", async () => {
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    );
    expect(overflow).toBe(false);
    await page.getByRole("button", { name: "打开导航", exact: true }).click();
    await expect(page.locator(".sidebar")).toBeVisible();
    await page.getByRole("button", { name: "节点", exact: true }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "编辑配置", exact: true }).click();
    await screenshot("mobile-edit.png");
    await a11y("375px editor");
    await page.keyboard.press("Escape");
  });
  await check("1440px and 768px layout", async () => {
    for (const width of [1440, 768]) {
      await page.setViewportSize({ width, height: 1024 });
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
      ).toBe(false);
      await screenshot(`viewport-${width}.png`, true);
    }
  });
  await check("375px English and accessible access entry", async () => {
    await page.setViewportSize({ width: 375, height: 844 });
    await page.locator(".language select").selectOption("en");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
    ).toBe(false);
    await page
      .getByRole("button", { name: "Prototype access", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText(
      "Do not enter credentials",
    );
    await page.keyboard.press("Escape");
    await screenshot("mobile-english.png", true);
    await a11y("375px English overview");
  });
  await page.setViewportSize(report.viewport);
  await page.locator(".language select").selectOption("zh");
  await page.goto(`${target}#consumers`);
  await check(
    "consumer collection, full-dataset search, page and detail restoration",
    async () => {
      await expect(
        page.getByRole("heading", { name: "Consumer 列表", exact: true }),
      ).toBeVisible();
      await page.locator(".fixture-controls summary").click();
      await page
        .getByLabel("Queue 样例", { exact: true })
        .selectOption("priority7");
      await expect(page.locator(".consumer-table tbody tr")).toHaveCount(5);
      await page.getByRole("button", { name: "下一页", exact: true }).click();
      await expect(page.locator(".consumer-table tbody tr")).toHaveCount(3);
      await screenshot("consumers-page2.png", true);
      await a11y("consumer list page 2");
      await page
        .getByRole("button", { name: "RJSQC_orders_events_P7", exact: true })
        .click();
      await expect(
        page.getByRole("heading", { name: "Consumer 详情", exact: true }),
      ).toBeFocused();
      await expect(page.locator(".consumer-detail")).toContainText(
        "rjs.q.orders_events.p.7",
      );
      await page.reload();
      await expect(page.locator(".consumer-detail")).toContainText(
        "RJSQC_orders_events_P7",
      );
      await screenshot("consumer-detail.png", true);
      await a11y("consumer detail");
      await page
        .getByRole("button", { name: "返回 Consumer 列表", exact: true })
        .click();
      await expect(page.locator(".consumer-pagination")).toContainText("2 / 2");
      await page.getByRole("searchbox").fill("p.7");
      await expect(page.locator(".consumer-table tbody tr")).toHaveCount(1);
      await expect(page.locator(".consumer-pagination")).toContainText("1 / 1");
      await expect(page.locator(".consumer-table")).toContainText(
        "RJSQC_orders_events_P7",
      );
      await page.getByRole("searchbox").fill("absent");
      await expect(page.locator(".empty")).toContainText("没有匹配");
      await page.getByRole("button", { name: "清除筛选", exact: true }).click();
      await page.getByLabel("投递模式", { exact: true }).selectOption("push");
      await expect(page.locator(".empty")).toContainText("没有匹配");
      await page.getByLabel("投递模式", { exact: true }).selectOption("all");
    },
  );
  await check("list URL, reload and browser history", async () => {
    await page
      .locator(".breadcrumbs")
      .getByRole("button", { name: "Queues", exact: true })
      .click();
    await expect(page).toHaveURL(/#queues/);
    await expect(page.locator(".breadcrumbs")).not.toContainText(
      "orders_events",
    );
    await page.reload();
    await expect(page.locator(".list-page")).toBeVisible();
    await page.getByRole("button", { name: "查看详情", exact: true }).click();
    await page.goBack();
    await expect(page.locator(".list-page")).toBeVisible();
    await page.goForward();
    await expect(page.locator(".evidence-panel")).toBeVisible();
  });
  await page.goto(`${target}#consumers`);
  await check(
    "accepted declaration remains separate until successful observation",
    async () => {
      await page
        .getByRole("button", { name: "RJSQC_orders_events", exact: true })
        .click();
      await page.getByRole("button", { name: "编辑配置", exact: true }).click();
      await page.locator("input[name=ackWait]").fill("45s");
      await page.getByRole("button", { name: "复核修改", exact: true }).click();
      await expect(page.getByRole("dialog")).toContainText(
        "RJSQC_orders_events",
      );
      await page
        .getByRole("button", { name: "应用模拟修改", exact: true })
        .click();
      await expect(page.locator(".consumer-detail")).toContainText("30s");
      await expect(page.locator(".consumer-panel")).toContainText("等待新观测");
      await page.locator(".fixture-controls summary").click();
      await page.getByLabel("读取场景", { exact: true }).selectOption("failed");
      const before = await page.locator(".header-time").innerText();
      await page.getByRole("button", { name: "刷新", exact: true }).click();
      await expect(
        page.getByRole("button", { name: "刷新", exact: true }),
      ).toBeEnabled();
      await expect(page.locator(".consumer-detail")).toContainText("30s");
      expect(await page.locator(".header-time").innerText()).toBe(before);
      await screenshot("consumer-stale.png", true);
      await a11y("failed observation with retained data");
      await page
        .getByLabel("读取场景", { exact: true })
        .selectOption("success");
      await page.getByRole("button", { name: "刷新", exact: true }).click();
      await expect(page.locator(".consumer-detail")).toContainText("45s");
      await expect(page.locator(".consumer-panel")).not.toContainText(
        "等待新观测",
      );
    },
  );
  await check(
    "forbidden, missing collection and missing detail are not zero",
    async () => {
      await page
        .getByLabel("读取场景", { exact: true })
        .selectOption("forbidden");
      await expect(page.getByRole("alert")).toContainText("无权读取");
      await expect(page.locator(".consumer-detail")).toHaveCount(0);
      await page.getByLabel("读取场景", { exact: true }).selectOption("empty");
      await expect(page.getByRole("alert")).toContainText("未找到");
      await page
        .getByRole("button", { name: "返回 Consumer 列表", exact: true })
        .click();
      await expect(page.locator(".empty")).toContainText("预期托管资源缺失");
      await page
        .getByLabel("读取场景", { exact: true })
        .selectOption("success");
    },
  );
  await page.goto(`${target}#overview`);
  await page.reload();
  await check(
    "equivalent duration produces no mutation or audit event",
    async () => {
      await page.getByRole("button", { name: "编辑配置", exact: true }).click();
      await page.locator("input[name=ackWait]").fill("0.5m");
      await page.getByRole("button", { name: "复核修改", exact: true }).click();
      await expect(page.getByRole("dialog")).toContainText("没有配置变更");
      await expect(
        page.getByRole("button", { name: "应用模拟修改", exact: true }),
      ).toBeDisabled();
      await page.keyboard.press("Escape");
      await expect(page.locator(".revision")).toContainText("12");
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(page.getByText("当前会话尚无配置变更。")).toBeVisible();
    },
  );
  await check(
    "mobile Consumer list/detail reflow in both languages",
    async () => {
      await page.goto(`${target}#consumers?fixture=priority7`);
      await page.setViewportSize({ width: 375, height: 844 });
      await screenshot("mobile-consumers.png", true);
      await a11y("375px Consumer list");
      for (const lang of ["zh", "en"]) {
        await page.locator(".language select").selectOption(lang);
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth > innerWidth,
          ),
        ).toBe(false);
      }
      await page
        .getByRole("button", { name: "RJSQC_orders_events_P0", exact: true })
        .click();
      await screenshot("mobile-consumer-detail.png", true);
      await a11y("375px English Consumer detail");
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
      ).toBe(false);
    },
  );
  await page.setViewportSize(report.viewport);
  await page.locator(".language select").selectOption("zh");
  async function openWriteScenario(scenario) {
    await page.goto(`${target}#overview`);
    await page.reload();
    await page.getByRole("button", { name: "编辑配置", exact: true }).click();
    await page.getByText("写入模拟场景", { exact: true }).click();
    await page
      .getByLabel("下次提交场景", { exact: true })
      .selectOption(scenario);
    await page.locator("input[name=ackWait]").fill("45s");
    await page.locator("input[name=retention]").fill("48h");
    await page.getByRole("button", { name: "复核修改", exact: true }).click();
    await page
      .getByRole("button", { name: "应用模拟修改", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "提交中，请勿重复提交", exact: true }),
    ).toBeDisabled();
    await expect(
      page.getByRole("heading", { name: "核查写入结果", exact: true }),
    ).toBeVisible();
  }
  async function submitRecoveredDraft() {
    await page.getByRole("button", { name: "复核修改", exact: true }).click();
    await page
      .getByRole("button", { name: "应用模拟修改", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  }
  await check(
    "conflict compares base/local/current and requires explicit reviewed recovery",
    async () => {
      await openWriteScenario("conflict");
      await expect(page.locator(".mutation-outcome")).toContainText("30s");
      await expect(page.locator(".mutation-outcome")).toContainText("45s");
      await expect(page.locator(".mutation-outcome")).toContainText("90s");
      await expect(
        page.getByRole("button", { name: "应用模拟修改", exact: true }),
      ).toHaveCount(0);
      await screenshot("mutation-conflict.png");
      await a11y("mutation conflict");
      await page.setViewportSize({ width: 375, height: 844 });
      await screenshot("mobile-mutation-conflict.png");
      await a11y("375px mutation conflict");
      await page.setViewportSize(report.viewport);
      await page
        .getByRole("button", { name: "保留本地改动并重新编辑", exact: true })
        .click();
      await expect(page.locator("input[name=ackWait]")).toHaveValue("45s");
      await submitRecoveredDraft();
      await expect(page.locator(".revision")).toContainText("14");
    },
  );
  await check(
    "permission and expiry retain drafts without committing",
    async () => {
      for (const scenario of ["forbidden", "expired"]) {
        await openWriteScenario(scenario);
        await expect(page.locator(".revision")).toContainText("12");
        await expect(page.locator(".config-panel")).toContainText("30s");
        await expect(page.locator(".mutation-outcome")).toContainText(
          "未执行修改",
        );
        await page
          .getByRole("button", { name: "模拟恢复访问并重新编辑", exact: true })
          .click();
        await expect(page.locator("input[name=retention]")).toHaveValue("48h");
        await submitRecoveredDraft();
        await expect(page.locator(".revision")).toContainText("13");
      }
    },
  );
  await check(
    "unknown outcome survives closing and only inspection resolves it",
    async () => {
      await openWriteScenario("unknown");
      await screenshot("mutation-unknown.png");
      await a11y("unknown mutation outcome");
      await page.getByRole("button", { name: "关闭结果", exact: true }).click();
      await expect(
        page.getByRole("button", { name: "刷新", exact: true }),
      ).toBeDisabled();
      await page
        .getByRole("button", { name: "查看未决操作", exact: true })
        .click();
      await expect(page.locator(".mutation-outcome")).toContainText(
        "mock-op-1",
      );
      await page
        .getByRole("button", { name: "核查模拟资源与审计", exact: true })
        .click();
      await expect(page.locator(".mutation-outcome")).toContainText(
        "这不是重新提交",
      );
      await page.getByRole("button", { name: "完成核查", exact: true }).click();
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(page.getByText("queue.apply", { exact: true })).toHaveCount(
        1,
      );
      await expect(
        page.getByText("resource.inspect", { exact: true }),
      ).toHaveCount(1);
    },
  );
  await check(
    "partial application preserves observed changes without advancing declaration",
    async () => {
      await openWriteScenario("partial");
      await page
        .getByRole("button", { name: "核查模拟资源与审计", exact: true })
        .click();
      await expect(page.locator(".mutation-outcome")).toContainText("48h");
      await expect(page.locator(".mutation-outcome")).toContainText("30s");
      await expect(page.locator(".revision")).toContainText("12");
      await screenshot("mutation-partial.png");
      await a11y("partial application inspection");
      await page
        .getByRole("button", { name: "按当前声明人工复核修复", exact: true })
        .click();
      await expect(page.locator("input[name=ackWait]")).toHaveValue("45s");
      await submitRecoveredDraft();
      await expect(page.locator(".revision")).toContainText("13");
    },
  );
  await check(
    "missing audit outcome stays distinct from confirmed resources",
    async () => {
      await openWriteScenario("audit-failed");
      await page
        .getByRole("button", { name: "核查模拟资源与审计", exact: true })
        .click();
      await expect(page.locator(".mutation-outcome")).toContainText(
        "审计结果仍缺失",
      );
      await page.getByRole("button", { name: "完成核查", exact: true }).click();
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(page.locator(".tab-surface")).toContainText("审计结果缺失");
      await expect(page.getByText("queue.apply", { exact: true })).toHaveCount(
        1,
      );
    },
  );
  await check(
    "duplicate submit dispatch creates only one mock write",
    async () => {
      await page.goto(`${target}#overview`);
      await page.reload();
      await page.getByRole("button", { name: "编辑配置", exact: true }).click();
      await page.locator("input[name=ackWait]").fill("45s");
      await page.getByRole("button", { name: "复核修改", exact: true }).click();
      await page
        .getByRole("button", { name: "应用模拟修改", exact: true })
        .evaluate((button) => {
          button.click();
          button.click();
        });
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.locator(".revision")).toContainText("13");
      await page.getByRole("tab", { name: "事件", exact: true }).click();
      await expect(page.getByText("queue.apply", { exact: true })).toHaveCount(
        1,
      );
    },
  );
  await check(
    "history navigation cannot replace a dirty editor context",
    async () => {
      await page.getByRole("tab", { name: "配置", exact: true }).click();
      await page
        .getByRole("button", { name: "编辑配置", exact: true })
        .first()
        .click();
      await page.locator("input[name=ackWait]").fill("60s");
      await page.goBack();
      await expect(page).toHaveURL(/#configuration/);
      await expect(page.locator("input[name=ackWait]")).toHaveValue("60s");
      await page.keyboard.press("Escape");
      await page.getByRole("button", { name: "放弃修改", exact: true }).click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
    },
  );
  await check("no external calls or browser errors", async () => {
    expect(report.externalRequests).toEqual([]);
    expect(report.consoleErrors).toEqual([]);
  });
  const serious = report.a11y.flatMap((r) =>
    r.violations.filter((v) => ["serious", "critical"].includes(v.impact)),
  );
  report.result = serious.length ? "a11y-blocked" : "checks-passed";
  console.log(JSON.stringify(report, null, 2));
  if (serious.length) process.exitCode = 1;
} catch (error) {
  report.result = "failed";
  report.error = error.stack;
  console.error(error);
  process.exitCode = 1;
} finally {
  await writeFile(
    path.join(output, "results.json"),
    JSON.stringify(report, null, 2),
  );
  await browser.close();
}
