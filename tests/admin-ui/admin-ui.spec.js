const { test, expect } = require("@playwright/test");
const AxeBuilder = require("@axe-core/playwright").default;

const queueName = "browser_e2e";

async function openEditor(page) {
  await page.getByRole("button", { name: "Create queue" }).click();
  await expect(page.getByRole("heading", { name: "Create or update queue" })).toBeVisible();
}

async function setDocument(page, document) {
  await page.locator("#queue-document").fill(JSON.stringify(document, null, 2));
}

function queueDocument(maxMessages = 100) {
  return {
    apiVersion: "rabbit-jetstream.io/v1alpha1",
    kind: "Queue",
    metadata: { name: queueName, labels: { suite: "browser" } },
    spec: {
      subjects: [`${queueName}.events`],
      replicas: 1,
      storage: "file",
      maxPriority: 2,
      retention: { maxAge: "1h", maxBytes: "16MiB", maxMessages },
      delivery: { ackWait: "30s", maxDeliver: 5 }
    }
  };
}

test.beforeEach(async ({ page, request }) => {
  const current = await request.get(`/api/v1/queues/${queueName}`);
  if (current.ok()) {
    await request.delete(`/api/v1/queues/${queueName}?force=true`, {
      headers: {
        Authorization: "Bearer browser-test-token",
        "If-Match": current.headers().etag,
        "X-RJS-Confirm-Queue": queueName
      }
    });
  }
  await page.goto("/admin/");
  await expect(page.locator("#service-state")).toHaveText("Management ready");
});

test("creates, updates, filters, inspects and deletes a priority queue", async ({ page }) => {
  await openEditor(page);
  await page.locator("#editor-token").fill("browser-test-token");
  await setDocument(page, queueDocument());
  await page.getByRole("button", { name: "Apply queue" }).click();

  const row = page.locator(`tr[data-queue="${queueName}"]`);
  await expect(row).toBeVisible();
  await expect(row).toContainText("Ready");
  await page.locator("#queue-filter").fill("browser_e2");
  await expect(row).toBeVisible();
  await page.locator("#queue-filter").fill("does-not-exist");
  await expect(page.getByText("No queues match this filter.")).toBeVisible();
  await page.locator("#queue-filter").fill("");

  await row.press("Enter");
  await expect(page.getByRole("heading", { name: queueName })).toBeVisible();
  await page.getByRole("button", { name: "Manage declaration" }).click();
  const updated = queueDocument(200);
  await setDocument(page, updated);
  await page.getByRole("button", { name: "Apply queue" }).click();
  await expect(row).toBeVisible();

  await row.click();
  await page.getByRole("button", { name: "Manage declaration" }).click();
  await page.locator("#delete-confirm").fill("wrong-name");
  await page.getByRole("button", { name: "Delete queue" }).click();
  await expect(page.locator("#editor-alert")).toContainText("must exactly match");
  await page.locator("#delete-confirm").fill(queueName);
  await page.getByRole("button", { name: "Delete queue" }).click();
  await expect(row).toHaveCount(0);
});

test("keeps credentials in memory and surfaces authorization and conflict failures", async ({ page }) => {
  await openEditor(page);
  await setDocument(page, queueDocument());
  await page.getByRole("button", { name: "Apply queue" }).click();
  await expect(page.locator("#editor-alert")).toContainText("Enter an operator bearer token");

  await page.locator("#editor-token").fill("invalid-token");
  await page.getByRole("button", { name: "Apply queue" }).click();
  await expect(page.locator("#editor-alert")).toContainText(/valid bearer|unauthorized|401/i);

  await page.locator("#editor-token").fill("browser-test-token");
  await page.route(`**/api/v1/queues/${queueName}`, async route => {
    if (route.request().method() === "PUT") {
      await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { message: "revision conflict" } }) });
    } else {
      await route.continue();
    }
  });
  await page.getByRole("button", { name: "Apply queue" }).click();
  await expect(page.locator("#editor-alert")).toContainText("revision conflict");

  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length, cookies: document.cookie }))).toEqual({ local: 0, session: 0, cookies: "" });
  await page.reload();
  await expect(page.locator("#operator-token")).toHaveValue("");
});

test("reports partial API failure and remains usable on a narrow viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route("**/api/v1/nodes", route => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { message: "monitor unavailable" } }) }));
  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.locator("#alert")).toContainText("monitor unavailable");
  await expect(page.getByRole("button", { name: "Create queue" })).toBeVisible();
  await expect(page.locator("body")).not.toHaveJSProperty("scrollWidth", 0);
});

test("has no serious or critical automated accessibility violations", async ({ page }) => {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  const blocking = results.violations.filter(item => ["serious", "critical"].includes(item.impact));
  expect(blocking, blocking.map(item => `${item.id}: ${item.help}`).join("\n")).toEqual([]);
});
