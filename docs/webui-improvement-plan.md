# WebUI Admin Console Improvement Plan

Status legend: **Done** — implemented in this working tree; **Tracked** — accepted as known work with a rationale.

Companion document: [webui-improvement-plan.zh-CN.md](webui-improvement-plan.zh-CN.md). The analysis covers the console frontend (`admin-ui/src`) and the management backend that serves it (`management/internal/api`).

## Baseline Assessment

The console is a security-first control plane: preview-before-apply, `If-Match` optimistic concurrency (428), capability-drift preconditions (412), fail-closed audit (intent persisted before execution), and mutation evidence in error responses. The frontend mirrors this with explicit write state machines, URL-encoded state, a completion-based refresh scheduler and a real accessibility baseline (skip nav, focus management, `role="status"/"alert"`, axe-gated live suites). The improvement items below therefore concentrate on consistency, scale and interaction polish — none of them change the write-safety semantics.

## 1. Functional Design

| # | Item | Status | Notes |
|---|---|---|---|
| F-1 | Login rate limiting is blind behind reverse proxies: `loginLimiter` keys on `RemoteAddr`, so all clients behind one proxy share a single failure bucket (20/min), letting one actor lock out everyone. | **Done** | `RJS_TRUSTED_PROXY_HOPS` (default 0, lenient parse) resolves the client from `X-Forwarded-For` by hop count; used by the limiter and by audit `SourceIP` attribution. Non-IP or short chains fall back to the peer. See `management/internal/api/handler.go` (`clientIP`), `docs/configuration.md`. |
| F-2 | Expired tokens are indistinguishable from invalid ones: `authorize` swallowed the verifier error, forcing the UI to guess expiry from its local clock. | **Done** | `identity.ErrTokenExpired` is returned by the local verifier and mapped to 401 `token_expired`; the console maps that exact code to its "expired" state and still never labels a generic 401 as expiry. Documented in `docs/webui-api-contracts.md`. |
| F-3 | List endpoints filter by substring over a full backend enumeration with in-memory sort; the 10k Queues / 100k Consumers scale target was never measured. | **Done (simulation scope)** | Query, collection, authenticated HTTP and maximum-page DOM gates cover 10k/100k scale; Docker also passed one million replicated messages. The measured generation-bound offset contract remains; no generic cursor is justified. Real exact-candidate population is release qualification, not unfinished WebUI implementation. |
| F-4 | Global Consumer index ownership/architecture decision is still open; per-row Queue health aggregation risks N+1 reads. | **Done** | JetStream/declarations remain authoritative; management owns only a tenant-scoped, bounded, rebuildable, replica-local projection. Isolation, generation mismatch, stale retention and replica semantics are contract-tested; multi-replica pagination requires sticky routing or a shared projection. |
| F-5 | `handler.go` is a god-file (routes + auth + audit + error mapping + pagination); strict-JSON decode boilerplate is re-implemented per handler; responses mix typed structs and `map[string]any`. | **Done (structure)** | Split into `routes.go` (registration/middleware), `authz.go` (authorization + client IP), `audit.go` (intent/outcome/IDs), `errors.go` (error mapping), and `decode.go`. Generated response typing has begun on the global Consumer path; remaining generic responses are incremental cleanup, not a gate. |
| F-6 | Access-account handlers flatten validation and storage failures into one 409 `account_*_rejected`. | **Done** | Store returns identity sentinels (`ErrAccountExists/NotFound/Policy/Validation/StoreUnavailable`); the API now answers 404 `account_not_found`, 409 `account_already_exists`, 409 `account_policy_rejected`, 400 `invalid_account_request`, 503 `local_account_store_unavailable`, with the historical 409 bucket kept for unrecognized failures. Covered by `access_account_errors_test.go`. |
| F-7 | No push channel: audit/alert pages poll. | **Done** | Authenticated tenant-scoped fetch/SSE invalidations drive bounded Audit/Alert refreshes, with replay, heartbeat, backoff, budgets and slow-client eviction. Proxy, HTTP/2, Linux race and real Docker/Chromium paths passed. Alert detection remains bounded polling, not exact notification. See `docs/webui-sse.md`. |
| F-8 | OpenAPI is embedded but not used to generate Go/TS types. | **Done (generation baseline)** | Pinned reproducible Go/TS generation, offline Queue bundling, checked-in outputs, CI drift and uint64/bigint protection are complete. The global Consumer 200 path now uses generated models with contract-equivalence tests. Further endpoint adoption is incremental typing, not an infrastructure gate. See `docs/openapi-codegen.md`. |

## 2. Interaction Design

| # | Item | Status | Notes |
|---|---|---|---|
| I-1 | Native `window.confirm/alert/prompt` for destructive flows: they cannot carry evidence-safety guidance, block the renderer and are unstyleable — the console's most important copy lived in the weakest medium. | **Done** | `dialog.jsx` provides a promise-based in-app `alertdialog` (confirm / alert / prompt modes, danger-tone confirm buttons, Escape-cancel, focus restore, one-dialog-per-host semantics). All 8 native-dialog sites were replaced (session clear, tenant switch, SSO logout, creation resume/archive/import, editor handoff, routing mode change, label add/rename/remove, account delete). Live suites updated from `page.once("dialog")` to in-page dialog interactions. |
| I-2 | A global write lock (`submitting`/`uncertain`/`inspecting`) is invisible once the user navigates away from the Queue page that owns it. | **Done** | `evidence-indicator.jsx`: a topbar badge lists retained drafts, deletions and evidence with phase labels and deep links; the badge turns danger-colored while writes are locked. |
| I-3 | URL-driven tenant switching confirmed through a blocking native dialog mid-navigation. | **Done** | The route effect now reverts the URL immediately and asks in-page; a clean console still switches synchronously without a dialog. |
| I-4 | Queue detail lacks breadcrumbs and resource headers are inconsistent (design-qa P1). | **Done** | Shared `resource-breadcrumb.jsx` on the Queue detail/edit/delete trio, Stream detail, Consumer detail and Connection detail; queue detail additionally gained shared `RouteLink` components for its edit/delete/tab navigation. Full resource-header composition (refresh placement, density) still belongs to the S-4 visual pass. |
| I-5 | Tables lack `<caption>`; refreshed totals already announce via `role="status"`. | **Done** | Captions on Queue/Stream list, Node list, Stream consumer collection, Queue consumer collection, Global Consumers, Audit window, Connections and tenant accounts tables. |
| I-6 | Copy affordances: request IDs and evidence JSON require manual text selection. | **Done (request IDs)** | `copy-value.jsx` attached to the editor's preview/apply request IDs and the deletion request ID. Evidence-JSON export remains tracked with the evidence-indicator follow-up. |
| I-7 | Refresh wiring, stale-evidence predicate and router-link click handling are copy-pasted across pages. | **Done** | `use-refresh-loop.mjs` and `stale-evidence.jsx` serve every polling page. Shared `RouteLink` covers router-owned breadcrumbs, tabs, collection rows and summary links; remaining anchors are intentional native/external links. |

## 3. Visual Style

| # | Item | Status | Notes |
|---|---|---|---|
| S-1 | Only 4 CSS custom properties; 30+ hardcoded hex values, near-duplicate danger/warning shades, old-palette remnants. | **Done (tokenized)** | `shell.css` now defines a semantic token layer (surfaces, ink, nav, lines, status, focus) and all rules reference tokens. Colors are byte-identical to the audited build on purpose — consolidating near-duplicate shades is deferred to the visual-QA pass to keep the acceptance baseline stable. |
| S-2 | Font stack put `Arial` before `system-ui` and lacked CJK fallbacks for macOS/Linux (Microsoft YaHei only). | **Done** | `system-ui` first with PingFang SC / Noto Sans CJK SC fallbacks; `color-scheme: light` declared. |
| S-3 | Dead rules: `#172b4d` button color overridden later; blue focus outline color always overridden by green. | **Done** | Removed; a single `:focus-visible` token rule remains. |
| S-4 | Spacing scale (4/8/12/16/24/32) and density targets from `docs/webui-selected-design.md` not enforced; no shared Table/Pagination components. | **Done** | Queue/Stream and global Consumer collections share native-table and pagination primitives with dense cells, scoped headers, captions, live ranges, and focusable overflow. The 96-case Chromium matrix and Firefox overflow/focus check passed after fixing the 1280px filter breakpoint. See `docs/webui-s4-visual-qa.md`. |
| S-5 | Icon usage is inconsistent (Tabler in nav only). | **Done (decision recorded)** | Decision written into `admin-ui/README.md`: icons stay navigation-only for bilingual clarity; extending them to action buttons is an explicit design decision. |
| S-6 | No favicon (persistent 404). | **Done** | Inline SVG data-URI favicon; no extra asset file, so the embedded-asset allowlist is unchanged. |
| S-7 | Dark mode absent. | **Done** | `prefers-color-scheme: dark` re-points the token layer (plus a dedicated `--brand` token); contrast verified with an axe color-contrast probe in dark preference across login, overview, queues, nodes and settings — zero violations. New shades must stay inside the token layer. |

## 4. Other Issues

| # | Item | Status | Notes |
|---|---|---|---|
| O-1 | `/metrics` is unauthenticated by design. | **Documented (existing)** | Already documented; deployment checklists must keep the network-isolation reminder visible. No code change. |
| O-2 | `build-candidate/` vs `dist/` promotion confusion. | **Documented** | `admin-ui/README.md` explains the promote flow; `verify:dist` fails loudly. No change needed. |
| O-3 | i18n: four competing string idioms; en/zh drift risk. | **Done** | User-visible feature copy now comes from feature-scoped catalogs or parameterized catalog formatters. `i18n-inline-debt.test.mjs` requires zero inline bilingual selections; `labels-parity.test.mjs` checks en/zh structure, non-empty leaves, feature ownership, and unused top-level keys. Normal, empty, loading, stale, error, confirmation, and accessibility copy is present in both catalogs and protected by the existing model/DOM/browser suites. |
| O-4 | Accessibility gates cover static axe snapshots; dynamic announcement and focus restoration are asserted only implicitly. | **Done (dialogs)** | The live suite now asserts that a confirmation dialog moves focus inside itself on open and restores it to the triggering control on close (L-08 dialog behavior); dynamic error announcement continues to be exercised through the existing `role="alert"` assertions. |
| O-5 | `main.jsx` routing/tenant glue is the highest-regression-risk layer with no direct unit coverage. | **Done (facility + first critical paths)** | Added pinned jsdom + Testing Library/user-event on the existing Node runner. `main-dom.test.mjs` loads the real `main.jsx` through Vite and covers password login, clean tenant switching with permission replacement, dirty-route rollback, cancel/focus restoration, confirmation, evidence discard and canonical tenant URLs. Playwright remains the browser/visual authority. |

## Verification Performed

- `go test ./...` on all touched packages (`api`, `auth`, `config`, `identity`, `app`), plus new tests: `trusted_proxy_test.go` (hop resolution, per-client budgets behind a proxy, expired-token 401 code), `list_query_benchmark_test.go`, config parsing, and the updated local-auth expiry test.
- `go vet ./...` and `gofmt` clean on the repository.
- `node --test tests/*.test.mjs`: 390 passing (was 389; +labels parity, +token_expired mapping; one pre-existing skip).
- `vite build` + `dist-sync` promote and `--check` so the embedded assets match the source.
- Live suite `tests/admin-ui/candidate-live.mjs` (Chromium, real `nats-server` + management binaries): full pass, evidence in `artifacts/webui-live-*/report.json`. The in-app dialog interaction was additionally pinned by a throwaway 60-round open/cancel loop (0 missed clicks, 0 page errors) after root-causing a `settle` wiring defect the first live run exposed — exactly the class of regression the suite exists to catch.

## Completion Order

The decision order in `webui-remaining-gates-decisions.md` was executed: F-4/F-3 invariants, O-5, F-8, O-3, S-4, scale evidence, then F-7. All code and Docker Desktop simulation gates in this plan are closed. Bare-metal 24-hour qualification, exact-candidate Canary and external release signatures remain release gates and are intentionally not represented as WebUI implementation work.
