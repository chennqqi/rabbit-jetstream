---
name: rjs-console-automation
description: Automate and verify the Rabbit JetStream admin console (embedded at /admin/) - login flows (local accounts, SSO, recovery tokens), SPA navigation, page-level assertions, accessibility checks, and API/console parity verification. Use when testing the admin UI with Playwright or an agent browser, verifying console pages render correctly, or checking UI-vs-API behavior consistency.
---

# Rabbit JetStream admin console automation

## Environment

- Console served by the management binary at `http://127.0.0.1:8223/admin/` (embedded static assets; no dev server needed).
- Credentials: local accounts (`RJS_LOCAL_ACCOUNTS_FILE`, argon2id JSON) sign in with username/password; token automation uses the "Recovery or automation token" disclosure; API-only automation uses `Authorization: Bearer <token>` with `RJS_ADMIN_TOKEN`.
- Playwright suite: `tests/admin-ui/` (`npx playwright test` with `RJS_ADMIN_UI_URL` and `RJS_EXPECTED_DEPLOYMENT_PROFILE`); visual matrix: `admin-ui/scripts/s4-visual-qa.mjs` (96 screenshots; needs auth fixtures and a server at the target URL).

## Login automation (Playwright)

```js
await page.goto("/admin/");
await page.locator(".recovery-login summary").click();          // language-agnostic entry
await page.locator(".recovery-login input").fill(token);
await page.locator(".recovery-login button").click();
await page.waitForTimeout(2000);                                 // SPA route settles
```

Use `locale: "en-US"` in the context for stable English role names, or match on CSS classes (`.session-controls`, `.identity-panel`, `.recovery-login`) which are language-independent.

## Page map (agents read the SPA, not source)

- Primary navigation is grouped: `nav.primary-nav .nav-group h3` = RESOURCES / OPERATIONS / GOVERNANCE. Create Queue lives on the Queue list header (`a.list-create`), NOT the nav. Compatibility lives under Access and settings.
- Top bar: `.identity-flat` (actor + role + tenant, hidden < 850px), `.session-controls summary` opens `.identity-panel` containing expiry, policy, tenant select, and the destructive "Clear local session" button (confirmation dialog only appears when drafts/evidence are retained — tolerate absence with `.catch(()=>{})`).
- Session expiry: navigation stays, an `.expired-session` card offers re-login; draft JSON is never rendered on the expired page.
- Status badges: `.status-badge.status-ok|warn|bad|neutral` (three-tier semantics; pending deliveries > 0 get `.metric-attention`).
- Disclaimers are folded into `details.page-notes`; the refresh indicator is `.refresh-status`.

## API ↔ console parity

Everything the console shows comes from `/api/v1/*` with `Authorization: Bearer`:

- Queue list: `GET /api/v1/queues` · detail: `GET /api/v1/queues/{name}` · consumers: `.../consumers`
- Write path: `POST /api/v1/queues/{name}/preview` → `PUT /api/v1/queues/{name}` with `If-None-Match: *` (create) or `If-Match: "<kvRevision>"` (update)
- Delete: `GET .../delete-preview` → `DELETE` with `If-Match` + `X-RJS-Confirm-Queue: <name>` (+`?force=true`)
- Audit filtering uses `GET /api/v1/audit/windows?resource=...` (server-side filtered; the legacy `/api/v1/audit` endpoint is pagination-only)

## Conventions and traps

- Tokens are page-memory only: every full reload returns to login. Automate re-login per navigation.
- Clicks are SPA-routed: prefer `router.navigate` semantics via link clicks with plain left-button checks; full `page.goto` resets app state.
- `beforeunload` guard fires when drafts/evidence are retained — close tabs via explicit session clear or expect the dialog.
- Accessibility gate: zero axe serious/critical violations (`@axe-core/playwright`, tags wcag2a/wcag2aa) on authenticated pages; contrast tokens are in `admin-ui/src/shell.css`.
- The chart's helm test hook and the S-4 matrix exercise the same pages; keep spec locators aligned with `tests/admin-ui/admin-ui.spec.js`.
