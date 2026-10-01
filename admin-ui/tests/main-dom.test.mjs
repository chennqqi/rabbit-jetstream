import test from "node:test";
import assert from "node:assert/strict";
import {JSDOM} from "jsdom";
import {createServer} from "vite";
import react from "@vitejs/plugin-react";
import {fileURLToPath} from "node:url";

const globals = ["window", "document", "navigator", "location", "history", "HTMLElement", "Node", "Event", "FormData", "localStorage", "sessionStorage"];

function installDOM(url = "https://console.example/admin/settings") {
  const dom = new JSDOM('<!doctype html><html lang="en"><body><div id="root"></div></body></html>', {url});
  dom.window.matchMedia = () => ({matches: false, addEventListener() {}, removeEventListener() {}});
  const previous = new Map(globals.map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  for (const key of globals) Object.defineProperty(globalThis, key, {configurable: true, writable: true, value: dom.window[key]});
  return () => {
    dom.window.close();
    for (const [key, descriptor] of previous) descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key];
  };
}

test("main App logs in and switches a clean tenant through the DOM", async t => {
  const restoreDOM = installDOM();
  const React = (await import("react")).default;
  const {render, waitFor, cleanup} = await import("@testing-library/react");
  const userEvent = (await import("@testing-library/user-event")).default;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(url, window.location.origin).pathname;
    if (path === "/api/v1/oidc/config") return Response.json({error: {code: "not_found", message: "not found"}}, {status: 404});
    if (path === "/api/v1/auth/login") return Response.json({access_token: "short-lived", token_type: "Bearer", expires_at: "2099-09-12T00:00:00Z", tenants: ["team-a", "team-b"]});
    if (path === "/api/v1/session") {
      assert.equal(new Headers(options.headers).get("Authorization"), "Bearer short-lived");
      return Response.json({actor: "local:alice", role: "operator", permissions: ["resources:read", "queue:apply"], tenants: ["team-a", "team-b"], tenant_roles: {"team-a": "operator", "team-b": "auditor"}, tenant_permissions: {"team-a": ["resources:read", "queue:apply"], "team-b": ["audit:read"]}, expires_at: "2099-09-12T00:00:00Z", resource_read_policy: "authenticated"});
    }
    if (path === "/api/v1/capabilities") return Response.json({});
    throw new Error(`unexpected request ${path}`);
  };
  const server = await createServer({configFile: false, root: fileURLToPath(new URL("..", import.meta.url)), mode: "test", plugins: [react()], server: {middlewareMode: true}, appType: "custom", logLevel: "silent"});
  t.after(async () => {
    cleanup();
    await server.close();
    globalThis.fetch = originalFetch;
    restoreDOM();
  });

  const {App} = await server.ssrLoadModule("/src/main.jsx");
  const user = userEvent.setup({document});
  const view = render(React.createElement(App), {container: document.getElementById("root")});

  await user.type(view.getByLabelText("Username"), "alice");
  await user.type(view.getByLabelText("Password"), "correct horse battery staple");
  await user.click(view.getByRole("button", {name: "Sign in"}));

  const tenant = await view.findByLabelText("Active tenant");
  assert.equal(tenant.value, "team-a");
  await user.selectOptions(tenant, "team-b");
  await waitFor(() => assert.equal(tenant.value, "team-b"));
  assert.equal(window.location.pathname, "/admin/tenants/team-b/settings");
  assert.match(document.body.textContent, /auditor/);
  assert.doesNotMatch(document.body.textContent, /resources:read/);
  assert.match(document.body.textContent, /audit:read/);
});

test("main App reverts a dirty tenant route until the in-page confirmation succeeds", async t => {
  const restoreDOM = installDOM();
  const React = (await import("react")).default;
  const {render, waitFor, cleanup} = await import("@testing-library/react");
  const userEvent = (await import("@testing-library/user-event")).default;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(url, window.location.origin).pathname;
    if (path === "/api/v1/oidc/config" || path === "/api/v1/console/capabilities") return Response.json({error: {code: "not_found", message: "not found"}}, {status: 404});
    if (path === "/api/v1/auth/login") return Response.json({access_token: "short-lived", token_type: "Bearer", expires_at: "2099-09-12T00:00:00Z", tenants: ["team-a", "team-b"]});
    if (path === "/api/v1/session") return Response.json({actor: "local:alice", role: "operator", permissions: ["resources:read", "queue:apply"], tenants: ["team-a", "team-b"], tenant_roles: {"team-a": "operator", "team-b": "auditor"}, tenant_permissions: {"team-a": ["resources:read", "queue:apply"], "team-b": ["audit:read"]}, expires_at: "2099-09-12T00:00:00Z", resource_read_policy: "authenticated"});
    throw new Error(`unexpected request ${path} ${options.method ?? "GET"}`);
  };
  const server = await createServer({configFile: false, root: fileURLToPath(new URL("..", import.meta.url)), mode: "test", plugins: [react()], server: {middlewareMode: true}, appType: "custom", logLevel: "silent"});
  t.after(async () => {
    cleanup();
    await server.close();
    globalThis.fetch = originalFetch;
    restoreDOM();
  });

  const {App} = await server.ssrLoadModule("/src/main.jsx");
  const user = userEvent.setup({document});
  const view = render(React.createElement(App), {container: document.getElementById("root")});
  await user.type(view.getByLabelText("Username"), "alice");
  await user.type(view.getByLabelText("Password"), "secret");
  await user.click(view.getByRole("button", {name: "Sign in"}));
  const tenant = await view.findByLabelText("Active tenant");

  window.history.pushState(null, "", "/admin/tenants/team-a/queues/new");
  window.dispatchEvent(new window.PopStateEvent("popstate"));
  const name = await view.findByLabelText("New Queue name");
  await user.type(name, "orders");
  await user.selectOptions(tenant, "team-b");
  assert.equal(window.location.pathname, "/admin/tenants/team-a/queues/new");
  await user.click(await view.findByRole("button", {name: "Cancel"}));
  await waitFor(() => assert.equal(tenant.value, "team-a"));
  assert.equal(name.value, "orders");
  assert.equal(document.activeElement, tenant);

  await user.selectOptions(tenant, "team-b");
  await user.click(await view.findByRole("button", {name: "Switch tenant"}));
  await waitFor(() => assert.equal(window.location.pathname, "/admin/tenants/team-b/queues/new"));
  assert.equal(tenant.value, "team-b");
  assert.equal(document.querySelector("#create-name"), null);
});
