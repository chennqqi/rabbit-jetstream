# Review Improvement Plan

[English](review-improvement-plan.md) | [简体中文](review-improvement-plan.zh-CN.md)

Status: pending owner approval · Created: 2026-10-01
Inputs: [Operations Capability Review (technical manager)](ops-review.md) + [Admin Console Product Review (PM)](../admin-ui/design-review.md)
Scope: consolidate all action items from both reviews — deduplicated, sequenced, with acceptance criteria. Does not change existing architecture decisions (D-04 embedded assets, D-05 access policy, management plane stays off the message data path).

Effort units: S ≤ 1 day · M = 2–5 days · L = 1–2 weeks (single developer, full-time, excluding review wait time).
Item IDs A1–A26; the Source column references finding IDs from the two reviews (P0-1/P1-x/P2-x = product review; D1–D6 / Scenario x = operations review).

---

## 1. Goals and principles

**Goal**: raise the "3.0/5 declarative control plane" to an operations workbench the on-call team trusts and prefers — without sacrificing the strengths both reviews confirmed (declarative workflow, semantic honesty, bilingual & accessibility, token discipline).

Principles:
1. **Trust before polish**: the crash (P0) and the half-dead session state come before any visual work;
2. **Architectural boundaries hold**: the management plane never enters the message data path; message-level tooling goes through a separate controlled CLI/SDK track (A23);
3. **All visual changes through the token layer**: new colors/components must enter `shell.css` tokens via visual QA and pass the S-4 matrix regression (1440/1280/1024/375 × both languages × dark/light);
4. **No NATS subtree changes**;
5. **Every item has acceptance criteria**, and completion is written back (learning from the un-backfilled backlog, see D6).

---

## 2. Milestone overview

| Milestone | Theme | Items | Exit criteria |
|---|---|---|---|
| M0 (this week) | Stop the bleeding | A1–A4 | Management process survives 1h without Prometheus; session expiry recovers in one action; on-call runbook usable |
| M1 (2–4 weeks) | Trust and efficiency | A5–A15 | Status badges shipped; disclaimers out of page bodies; rc.3 re-frozen |
| M2 (1–2 months) | Cockpit and closed loop | A16–A22 | Overview answers core questions in 30s; alert-to-human < 5 min |
| M3 (this quarter) | Architecture decisions | A23–A26 | Message-level tooling track decided; first controlled tool delivered |

Dependency notes: A15 (rc.3 freeze) must follow all M1 merges; A19 (alert loop) depends on A12 (per-queue DLQ metrics); A23 requires the decision doc first.

---

## 3. M0: emergency fixes (this week)

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A1 | Receiver nil-guard in `AlertRules`: return a "not configured" error instead of panicking; add a failure-path test (no Prometheus config + alert watcher + SSE subscription, assert process survival) | P0-1 / D1 | Backend | S | New `go test` case passes; local run without `RJS_PROMETHEUS_URL` + logged-in browsing survives 1 hour |
| A2 | Session-expiry state fix: on token expiry keep primary navigation, show a primary "sign in again" button, hide the editor draft JSON; shorten the expiry notice to one line with details collapsed | D4 / Scenario 5 | Frontend | S | Verified: after expiry the page stays navigable and can reach login in one click; no `{"name":...}` draft string rendered |
| A3 | Publish the "operations companion checklist": Alertmanager baseline config (webhook/email examples), sidestep script samples for message-level troubleshooting (nats CLI/SDK: inspect messages, replay dead letters), a `rjsctl backup` cron example, on-call runbooks for backlog/DLQ/deletion (including manual investigation of the delete unknown-outcome path) | D2/D3/Scenarios 1–3 | Docs+SRE | M | Each of the three runbook scenarios is rehearsed end-to-end; the Alertmanager config is verified routing-capable on the local Compose stack |
| A4 | Deployment safety note: until A1 merges, deployment docs and compose comments state "`RJS_PROMETHEUS_URL` must be configured explicitly" | D1 | Docs | S | standalone.yml / cluster.yml / deployment docs all consistent |

---

## 4. M1: quick wins (2–4 weeks)

### 4.1 Frontend batch (depends on token-layer extensions + visual QA, see §8.1)

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A5 | Status badge system: three tiers for observed state / read state / alerts (new token shades via visual QA); pending deliveries beyond threshold get warning color + bold | P1-2 | Frontend | M | Queue list/detail, nodes, alerts all use badges; 500 vs 0 backlog distinguishable in screenshots; S-4 matrix regression passes |
| A6 | Data presentation: truncate 32-char hashes to 8 + copy button; humanize byte counts (64.7 KB); remove 1440px horizontal table overflow (verified on English Queue list, Chinese Stream list) | P2-1 | Frontend | S | No horizontal scrollbar at 1440px in English; full hash copyable in one click |
| A7 | Copy governance: page-top disclaimers demoted to collapsible tooltips / an "about these numbers" entry; "auto refresh 10s" becomes a status indicator (refreshed hh:mm:ss · auto) | P1-5 | Frontend | M | No negating sentences in page bodies; the refresh indicator shows correct state across network loss/recovery |
| A8 | Login page fixes: no red "SSO sign-in unavailable" error when SSO is unconfigured (collapsed note at most); login button promoted to primary style | P2-2/P2-3 | Frontend | S | No red error on SSO-less deployments; axe regression passes |
| A9 | Navigation regroup: groups (Resources / Operations / Governance); "Create Queue" moves into the Queue list page header as primary button; "Compatibility" demoted to settings or footer | P1-3 | Frontend | M | Nav ≤ 3 groups; create reachable from the list header; S-4 mobile matrix regression passes |
| A10 | Top-bar weight fix: identity (username + tenant) shown flat, "Clear local session" folded into an identity menu; dedupe branding with the sidebar | P2-5/P2-10 | Frontend | S | Top bar height reduced; identity visible without expanding |
| A11 | Form and control governance: create-form control width cap + two-column layout; consumer/audit filter areas collapsed to a single toolbar row; unified skins for native file inputs/selects | P2-4/P2-7/P2-8 | Frontend | M | Create Queue page shows its primary CTA within one screen; filter area height ≤ 120px |

### 4.2 Backend batch

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A12 | Per-queue DLQ metrics: add a queue label to `rjs_dlq_failed_total` et al. or add per-queue counters; rewrite the alert rule accordingly | D3/Scenario 2 | Backend | M | DLQ failures queryable per queue in Prometheus; `RabbitJetStreamDeadLetterFailures` can attribute a queue |
| A13 | DLQ-dependent check in delete preflight: preflight response and UI list "which queues declare deadLetter → this one" | D5/Scenario 3 | Backend+Frontend | M | Deleting a referenced queue shows dependents and requires explicit confirmation |
| A14 | Documentation hygiene: backfill backlog statuses (WEB-017/022/024/026 implemented); remove the stale "read-only" wording in `management-api.md`; update the stale title date in `remaining-release-work.md` | D6 | Docs | S | Three docs consistent with implementation; any doc-lint CI passes |

### 4.3 Release

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A15 | rc.3 re-freeze: after M1 merges, rebuild the exact candidate, run full regression (S-4 matrix, axe, dual-browser playwright gates), bind new 24h soak evidence to the new revision | D6 | Release | M | New frozen asset fingerprint committed; all gates green; evidence revisions consistent |

---

## 5. M2: cockpit and operations loop (1–2 months)

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A16 | Overview cockpit: health strip (endpoints/Queues/Consumers + status colors), top-5 backlog list linking details, storage trend chart (Prometheus); all overview disclaimers folded | P1-1 | Frontend+Backend | L | Usability test (3 users × 3 personas): "is it healthy / what's most backlogged / do I need to act" answerable in 30 seconds |
| A17 | Declared-vs-observed side-by-side: two columns on the Queue summary with mismatch highlighting; raw JSON into the collapsed area | P1-6 | Frontend | M | A manufactured ETag conflict renders visibly and links to detail |
| A18 | Session refresh recovery (after security review): one-time in-page re-verification after refresh (password re-entry silently restores), token still never persisted; or opt-off "remember username (this device)" | P1-4 | Frontend+Backend | M/L | F5 recovers the session within one interaction; security review on record; credentials still never in localStorage |
| A19 | Alert-loop verification: shipped Alertmanager config + a local Compose drill "rule fires → human notified" | D3 | SRE | S | Drill on record: trigger-to-notification < 5 minutes |
| A20 | Backlog threshold guidance: `QueueBacklogHigh` default 100k annotated "tune per workload", with a per-queue tuning example in the companion checklist | Scenario 1 | Docs | S | Runbook includes the tuning step |
| A21 | Mobile governance: 375px top bar compressed to one line, dismissible banner, card layouts replacing horizontally scrolled tables | P2-9 | Frontend | M | First screen shows data content at 375px; S-4 mobile matrix regression passes |
| A22 | Audit experience: time presets (last 1h/24h/7d), filter area single-row | Scenario 6/P2-8 | Frontend | S | Presets generate correct from/until; S-4 regression passes |

---

## 6. M3: architecture decision items (this quarter)

| # | Item | Source | Layer | Effort | Acceptance criteria |
|---|---|---|---|---|---|
| A23 | Decide the message-level tooling track: decision doc (outside the management plane, controlled CLI/SDK form: sanitized, audit-hooked, read-only peek first); if approved, deliver the first tool (read-only message browsing) | D2 | Architecture+Backend | Decision M / Delivery L | Decision doc committed; first tool supports "browse messages by subject (sanitized)" with audit events |
| A24 | Tenant lifecycle API/UI: create/modify/disable tenants without file edits + restart | §2.2 matrix | Backend+Frontend | L | UI-created tenant can log in; empty tenant deletable; zero restarts |
| A25 | Finer-grained roles: at minimum split `access:manage` out of operator (new role or permission set) | Security observation 2 | Backend | M/L | The new role cannot touch account management but can complete queue changes; authorization-matrix tests updated |
| A26 | Integration long tail (ordered on demand): event webhook push, Terraform provider, backup/restore orchestration (WEB-033) | §6 | Backend | L | Each is its own initiative; no dates promised here |

---

## 7. Metrics and acceptance summary

| Metric | Baseline (this review) | Target | Items |
|---|---|---|---|
| Management process stability | Panic within minutes without Prometheus | 7×24 survival (incl. regression for that scenario) | A1 |
| Session-expiry recovery | Half-dead state; blind spot "Clear local session" | Re-login in ≤ 2 clicks | A2/A18 |
| Alert to human | No loop (page projection only) | Drill < 5 minutes | A3/A19 |
| Overview decision support | Three screens of text, zero graphics | 3 personas' core questions answered in 30s | A16 |
| Status readability | 500 backlog looks like 0 | Three-tier badges, distinguishable in screenshots | A5 |
| Disclaimers | 1–3 per page body | 0 in bodies, folded into tooltips | A7 |
| Table overflow | Horizontal scroll at 1440px in English | None | A6 |
| Deletion safety | No DLQ-dependent check | Preflight lists dependents | A13 |
| DLQ attribution | Global counters only | Queryable per queue | A12 |
| Release consistency | Code drifted past rc.2 | rc.3 frozen + evidence revisions consistent | A15 |

---

## 8. Risks and boundaries

1. **Visual regression cost**: A5–A11 touch many pages — merge in two batches (token/component layer first, page application second), each batch passing the S-4 matrix + axe + dual-browser gates; avoid one big-bang diff;
2. **No honesty regression**: folding disclaimers is not deleting them — "observed ≠ declared" and "unknown stays unknown" must survive in tooltips; verify line-by-line at acceptance;
3. **A18's security boundary**: no refresh-recovery scheme may write credentials to localStorage/cookies (C-01 red line); produce the security review record before implementation;
4. **A23's architecture red line**: the management plane never enters the message data path (no peek disguised behind management APIs); the controlled tool is a separate process/authorization surface;
5. **Capacity reality**: A16/A23/A24 are all L-sized; with a single developer, pick at most one of M3's tracks — recommend A23 first (largest operational pain).

## 9. Suggested sequence (single-developer view)

```
Week 1     A1 → A2 → A4 → (A3 in parallel, owned by SRE)
Week 2–3   A6 → A8 → A5 (token layer first) → A7 → A10
Week 4     A9 → A11 → A12 → A13 → A14 → A15 (freeze)
Week 5–8   A16 (backend metrics first) ‖ A19 → A17 → A22 → A20 → A21
Week 9–12  A18 (security review) → A23 decision → start A24 or A25
```

## 10. Write-back obligation

On completion of each item: mark Done + date + evidence link (screenshots/tests/drill records) in this file, linked with the `docs/webui-development.md` ledger; the two review documents keep their original text (reviews are snapshots).

### Backfill record

| Item | Status | Date | Evidence |
|---|---|---|---|
| A1 AlertRules nil guard + regression tests | Done | 2026-10-01 | `management/internal/prometheus/nil_client_test.go`, `management/internal/api/alerts_nil_backend_test.go`; 3 packages green |
| A2 Session-expiry state fix | Done | 2026-10-01 | `admin-ui/src/main.jsx` expired branch rewrite; browser-verified one-click re-login, no draft JSON rendered |
| A3 Operations companion checklist | Done | 2026-10-01 | `docs/operations-companion.{,zh-CN.}md`, `deploy/observability/alertmanager.yml`, prometheus alertmanager wiring, compose alertmanager service |
| A4 Deployment safety note | Done | 2026-10-01 | `docs/configuration.md` Prometheus URL row; compose annotations; `docker compose config` valid |
| A5 Status badge system | Done | 2026-10-01 | `admin-ui/src/status-badge.jsx` + token-derived badges; applied in Queue list/detail, Nodes, Alerts |
| A6 Hash truncation + humanized bytes | Done | 2026-10-01 | `admin-ui/src/format.mjs` + tests; CopyValue short in lists; bytes humanized in Overview/QueuePanels/StreamDetail |
| A7 Disclaimer folding + refresh indicator | Done | 2026-10-01 | PageNotes/RefreshStatus applied on QueueList/Overview/Nodes/StreamDetail |
| A8 Login SSO fix + primary button | Done | 2026-10-01 | `read_auth.go` public oidc/config + test; silent SSO probe; `.primary-action` login button |
| A9 Nav grouping + create in list header | Done | 2026-10-01 | ConsoleNavigation 3 groups; Queue-list header Create Queue; Compatibility moved to Settings |
| A10 Top-bar weight fix | Done | 2026-10-01 | Flat identity, clear-session folded into panel, desktop brand dedupe |
| A11 Form control governance | Done | 2026-10-01 | `form.filter-toolbar` single-row; creation-form width cap |
| A12 Per-queue DLQ metrics | Done | 2026-10-01 | PerQueue attribution end-to-end; `rjs_dlq_queue_{moved,failed}_total`; DeadLetterFailures alert rewritten to `sum by (queue)` |
| A13 Delete-preflight DLQ dependents | Done | 2026-10-01 | `DeadLetterDependents` in preflight + UI row; API-verified |
| A14 Documentation hygiene | Done | 2026-10-01 | management-api read-only fix; release-work dates; backlog status backfill |
| A15 rc.3 re-freeze | Done (soak gate) | 2026-10-03 | Local gates green (S-4 96 shots, Playwright 8/8 incl. axe zero violations, Go contract CRLF fix); frozen linux/amd64 artifacts checksum-verified on host (revision 033010ee); 24h bare-metal soak observed **42 h**: 5053 cycles all-200 health/queues API, zero crashes post-fix, zero errors in management log, 453,505 messages sustained — evidence in `docs/releases/v0.1.0-rc.3-soak-evidence.md`. An actual rc.3 release still needs the owner freeze decision + helm/Kind cluster qualification (see remaining-release-work) |

