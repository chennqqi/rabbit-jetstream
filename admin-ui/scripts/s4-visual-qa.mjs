import {chromium, firefox} from "playwright";
import {mkdir, writeFile} from "node:fs/promises";
import path from "node:path";

const baseURL = process.env.RJS_ADMIN_UI_URL ?? "http://127.0.0.1:18223";
const output = path.resolve(import.meta.dirname, "../../artifacts/s4-visual-qa");
const widths = [1440, 1280, 1024, 375];
const languages = ["en", "zh"];
const themes = ["light", "dark"];
const states = ["normal", "empty", "error", "stale", "long-identifier", "maximum-column"];
const token = "s4-visual-qa-token";

await mkdir(output, {recursive: true});

const row = (index, long=false) => ({
  stream: long ? `stream-${"s".repeat(180)}` : `stream-${String(index).padStart(3, "0")}`,
  name: long ? `consumer-${"c".repeat(180)}` : `consumer-${String(index).padStart(3, "0")}`,
  queue: long ? `queue-${"q".repeat(180)}` : `queue-${String(index).padStart(3, "0")}`,
  durable: `durable-${index}`, mode: index % 2 ? "push" : "pull",
  status: "present", ownership: "matching",
  pending: 1000000 + index, ack_pending: index,
});

function responseFor(state) {
  if (state === "error") return {status: 503, body: {error: {code: "consumer_collection_unavailable", message: "simulated visual fixture"}}};
  const items = state === "empty" ? [] : state === "long-identifier" ? [row(0, true)] : Array.from({length: state === "maximum-column" ? 25 : 8}, (_, index) => row(index));
  return {status: 200, body: {state: state === "stale" ? "stale" : "ready", generation_id: "s4-fixed-generation", started_at: "2026-09-13T00:00:00Z", completed_at: "2026-09-13T00:00:01Z", items, total: items.length, offset: 0, limit: 200}};
}

async function authenticate(page) {
  await page.goto(`${baseURL}/admin/`);
  await page.waitForTimeout(250);
  const recovery=page.locator(".recovery-login summary");
  if (!await recovery.count()) throw new Error(`Recovery login unavailable. Page text: ${(await page.locator("body").innerText()).slice(0,1000)}`);
  await recovery.click();
  await page.locator(".recovery-login input").fill(token);
  await page.locator(".recovery-login button").click();
  try { await page.locator("nav.primary-nav").waitFor({timeout:5000}); }
  catch { throw new Error(`Authentication did not enter console. Page text: ${(await page.locator("body").innerText()).slice(0,1200)}`); }
}
async function navigateConsumers(page) {
  const link=page.locator('nav.primary-nav a[href="/admin/consumers"]');
  if (!await link.isVisible()) await page.locator(".navigation-toggle").click();
  await link.click();
}

const evidence = {baseURL, generated_at: new Date().toISOString(), chromium: [], firefox: []};

const chromiumBrowser = await chromium.launch({headless: true});
try {
  for (const language of languages) {
    const context = await chromiumBrowser.newContext({
      viewport: {width: 1440, height: 900},
      locale: language === "zh" ? "zh-CN" : "en-US",
    });
    await context.addInitScript(value => localStorage.setItem("rjs.language", value), language);
    const page = await context.newPage();
    let fixture = "normal";
    await page.route(/\/api\/v1\/consumers(?:\?|$)/, async route => {
      const response = responseFor(fixture);
      if (response.status === 200) { const query=new URL(route.request().url()).searchParams; response.body.limit=Number(query.get("limit")); response.body.offset=Number(query.get("offset")); }
      await route.fulfill({status: response.status, contentType: "application/json", body: JSON.stringify(response.body)});
    });
    await authenticate(page);
    const documentLanguage = await page.locator("html").getAttribute("lang");
    const expectedLanguage = language === "zh" ? "zh-CN" : "en";
    if (documentLanguage !== expectedLanguage) throw new Error(`Language fixture mismatch: expected ${expectedLanguage}, got ${documentLanguage}`);
    await navigateConsumers(page);
    for (const theme of themes) {
      await page.emulateMedia({colorScheme: theme});
      for (const width of widths) {
        await page.setViewportSize({width, height: 900});
        for (const state of states) {
          fixture = state;
          await page.locator("#consumer-search").fill(`fixture-${state}-${theme}-${width}`);
          await page.locator(".consumer-list-controls button").click();
          if (["normal","stale","long-identifier","maximum-column"].includes(state)) await page.locator(".collection-table-region").waitFor();
          else { await page.locator(".collection-table-region").waitFor({state:"detached"}); await page.waitForTimeout(50); }
          const filename = `chromium-${language}-${theme}-${width}-${state}.png`;
          const metrics = await page.evaluate(() => { const clipped=[...document.querySelectorAll("button,input,select")].filter(element => { const box=element.getBoundingClientRect(); return box.right > innerWidth + 1 || box.left < -1; }).map(element => ({tag:element.tagName,id:element.id,text:element.textContent?.slice(0,80),box:element.getBoundingClientRect().toJSON()})); return {documentOverflow: document.documentElement.scrollWidth > innerWidth, documentWidth:document.documentElement.scrollWidth, viewportWidth:innerWidth, clippedControls:clipped.length, clipped, tableOverflow: (() => { const region=document.querySelector(".collection-table-region"); return region ? region.scrollWidth > region.clientWidth : false; })()}; });
          if (metrics.documentOverflow || metrics.clippedControls) throw new Error(`${filename}: ${JSON.stringify(metrics)}`);
          await page.screenshot({path: path.join(output, filename), fullPage: true});
          evidence.chromium.push({filename, language, theme, width, state, ...metrics});
        }
      }
    }
    await context.close();
  }
} finally { await chromiumBrowser.close(); }

const firefoxBrowser = await firefox.launch({headless: true});
try {
  const context = await firefoxBrowser.newContext({viewport: {width: 375, height: 900}});
  const page = await context.newPage();
  await page.route(/\/api\/v1\/consumers(?:\?|$)/, route => { const body=responseFor("maximum-column").body,query=new URL(route.request().url()).searchParams; body.limit=Number(query.get("limit"));body.offset=Number(query.get("offset"));return route.fulfill({status: 200, contentType: "application/json", body: JSON.stringify(body)}); });
  await authenticate(page); await navigateConsumers(page);
  try { await page.locator(".collection-table-region").waitFor({timeout:5000}); }
  catch { throw new Error(`Firefox collection did not render. Page text: ${(await page.locator("body").innerText()).slice(0,1200)}`); }
  const result = await page.evaluate(() => { const region=document.querySelector(".collection-table-region"); region.focus(); region.scrollLeft=region.scrollWidth; return {focused: document.activeElement===region, overflow: region.scrollWidth>region.clientWidth, scrolled: region.scrollLeft>0, documentOverflow: document.documentElement.scrollWidth>innerWidth}; });
  if (!result.focused || !result.overflow || !result.scrolled || result.documentOverflow) throw new Error(`Firefox overflow/focus failed: ${JSON.stringify(result)}`);
  evidence.firefox.push(result);
  await context.close();
} finally { await firefoxBrowser.close(); }

await writeFile(path.join(output, "manifest.json"), JSON.stringify(evidence, null, 2) + "\n", "utf8");
process.stdout.write(`S-4 visual QA: ${evidence.chromium.length} Chromium screenshots; Firefox overflow/focus passed.\n`);
