# Operations Capability Review (Technical Manager Perspective)

[English](ops-review.md) | [简体中文](ops-review.zh-CN.md)

Status: review complete, pending scheduling · Review date: 2026-10-01
Subject: **completeness and usability of management/operations functions** across the management service (HTTP API), the operator CLI (`rjsctl`), and the admin console (`admin-ui/`)
Method:
1. **Three-layer inventory** — enumerated all 38 management API endpoints (`management/internal/api/routes.go`), every `rjsctl` subcommand, and all 14 console pages;
2. **Scenario walkthroughs** — on-call drills on a real local service (NATS JetStream + management server + 6 seeded queues / ~700 messages / consumers): backlog handling, DLQ, deletion, diagnostics bundle, session expiry, audit troubleshooting;
3. **Documentation cross-check** — the requirements backlog (`docs/webui-rabbitmq-requirements.md`, 31 items), per-capability boundary docs, shipped alert rules (`deploy/observability/alerts.yml`), and measured `/metrics` exposure.

Visual and IA findings are not repeated here — see the [product review](../admin-ui/design-review.md). This document answers the three questions a technical manager asks: **is the feature set complete, is it usable, and what is missing for a production operations loop.**

---

## 1. Executive summary

**This management plane "manages configuration well and cannot touch messages" — an architectural choice, not a defect — but the choice has never been productized for the operator.**

The control plane follows a philosophy of "declarative queue lifecycle + read-only observation + message data-path isolation": queue create/update/delete goes through preview-confirm-conditional-write, audit pairs intent with outcome, and message-level operations are excluded from the management plane. Within that philosophy the delivery quality is high — the declarative CLI toolchain (`plan/diff/reconcile/apply`), the three-step safe deletion, and the diagnostic evidence chain are rare among comparable products.

Judged against "a team runs a messaging platform day to day", four links are broken:

1. **Zero message-level operations** (view one message, purge, replay are all impossible); all data-path troubleshooting must sidestep to SDK/CLI, and that sidestep ships with no tools, samples, or documented convention;
2. **The alerting loop does not close**: six shipped Prometheus rules plus a read-only console projection, but no Alertmanager config, no notification channel — the console explicitly "does not send notifications";
3. **One P0 stability defect**: without Prometheus configured, the alert-watcher path nil-panics and kills the entire management process (actually triggered during this review, see 5.1);
4. **Release engineering is out of joint**: current code has drifted past the rc.2 frozen candidate, and documentation lags implementation (backlog statuses not backfilled; `management-api.md` still calls the UI read-only).

Dimension scores (out of 5):

| Dimension | Score | Summary |
|---|---|---|
| Management completeness (declaration domain) | 4.0 | Queue lifecycle, bulk, templates, import/export, access management all present |
| Management completeness (data/runtime domain) | 1.5 | Message ops, consumer control, connection actions, tenant lifecycle all absent |
| Operations-loop usability | 2.0 | Alerting doesn't close, half-dead session-expiry state, no DLQ-dependent check on delete |
| Automation & API friendliness | 4.0 | Good three-layer parity; declarative workflow fits GitOps |
| Security & compliance baseline | 3.5 | Solid key discipline, audit pairing, sanitized diagnostics; coarse roles, no token rotation |
| Diagnosability | 3.5 | Excellent diagnostics bundle/evidence downloads; no per-queue DLQ attribution |
| Release engineering maturity | 2.5 | Evidence drift, stale docs, missing re-freeze |

**Overall: 3.0 / 5.** Verdict: fit to pilot as a **declarative control plane**, provided a "operations companion checklist" ships alongside (§7) and the P0 is fixed — otherwise on-call hits the wall of "visible problem, no lever" on day one.

---

## 2. Operations capability matrix

Legend: ✔ implemented (usable in at least one of console/API/CLI)｜◐ partial (present with boundaries)｜✖ absent in all three layers

### 2.1 Present capabilities (declaration-domain strengths)

| Domain | Content | Layers | Assessment |
|---|---|---|---|
| Queue lifecycle | Declarative create/update (conditional write + ETag CAS), change preview, import/export, templates, bulk change with per-item confirm, CLI `plan/diff/reconcile/apply` | UI+API+CLI | ✔✔ Core strength beyond RabbitMQ Management UI |
| Safe deletion | Delete preflight (ownership, message/consumer counts) → force checkbox + exact name + impact acknowledgment → evidence JSON download → unknown outcomes lock automatic retry | UI+API+CLI | ✔✔ Exemplary safety design; missing DLQ-dependent check (see 4.5) |
| Audit | intent/outcome pairing, correlated query window (server-side filtering, verified by test), JSON export, request-ID correlation | UI+API+CLI | ✔ 256-sequence scan window, no match totals |
| Diagnostics | Metadata-only sanitized ZIP (9 sources, manifest+SHA256, bounded jobs, 10-minute expiry), same via CLI | UI+API+CLI | ✔ Create-to-download loop verified |
| Access management | Local account CRUD (argon2id), OIDC PKCE browser SSO, operator/auditor roles, tenant isolation, keys never in URLs/storage/logs | UI+API | ✔ |
| Backup & migration | `rjsctl backup create/verify/restore` (manifest+checksums), `migrate` suite (RabbitMQ definitions conversion, dual-write journal, shadow capture, reconcile, idempotent cutover/rollback) | CLI | ✔ A shippable RabbitMQ migration toolchain is a differentiator |
| DLQ | Controller-driven automatic move (at-least-once, `Nats-Msg-Id` dedup) + read-only diagnostics view (six evidence classes) + evidence download | UI+API | ◐ Counters are process-global aggregates; cannot attribute to a queue (see 4.3) |
| Metrics | `/metrics` exposes 22 metric families including **per-Queue** messages/bytes, controller, DLQ counters, HTTP | API | ✔ No delivery/consume rates, connection counts, or replica lag |
| Alerts | Six shipped Prometheus rules (JetStream unavailable / controller stalled / node unavailable / DLQ failures / backlog 100k / management error rate) + read-only console projection | API+UI | ◐ No notification loop (see 4.2) |
| Live events | SSE (tenant-isolated, authenticated, 256-event replay window, budgeted) driving list invalidation | API+UI | ✔ |

### 2.2 Missing capabilities (data/runtime-domain gaps)

| Gap | Industry baseline (RabbitMQ Management UI) | Impacted scenario | Status |
|---|---|---|---|
| Message-level ops: view/browse messages, purge queue, per-message replay/delete | Get Messages, Purge Queue | Data troubleshooting, test cleanup, poison-message handling | WEB-029/030 P2; architecture explicitly keeps the management plane off the data path — needs a decision |
| Consumer control: pause/resume, reset ack offset, delete consumer | Partial consumer management | Backlog handling, consumer migration | ✖ (consumer creation is SDK-domain, but there is zero lever after observing a problem) |
| Connection actions: fleet-wide connection view, disconnect client | Connections page + Close | Client fault isolation | ◐ Exact-CID lookup and subscription detail exist (WEB-019); no disconnect action |
| Tenant lifecycle: create/modify/disable tenants | vhost management | Multi-tenant operations | ✖ Static `RJS_TENANTS_FILE` only; changes require file edit + restart |
| Exchange management | Exchange CRUD | Binding governance | ✖ Bindings exist only inside Queue declarations (architectural mapping) |
| Alert notifications | Self-built (RabbitMQ ships none either) | Alert-to-human | ◐ Deployer must configure Alertmanager; no webhook/event push |
| Backup orchestration | Plugins | Scheduled backups, restore drills | ◐ CLI only; no scheduling, no restore-drill runbook |

**Manager's conclusion**: the declaration domain is complete; the runtime domain is not. Most gaps have legitimate architectural reasons (the management plane stays off the data path) — the real problem is not "not built", it is **no alternative path was delivered to operations**: no sidecar tools, sample scripts, documentation conventions, or companion-component checklist.

---

## 3. On-call scenario drills (usability)

All scenarios were manually exercised on the real local service (desktop 1440, both languages).

### Scenario 1: Consumer backlog (500 pending on a queue)
- **Detection**: ✔ big numbers on list/detail; shipped rule `RabbitJetStreamQueueBacklogHigh` (>100k messages for 15m) — which does **not** fire at 500, so this backlog raises no alert and is only found by looking.
- **Handling**: ✖ no console action exists (no pause consumer, no requeue, no scaling hint); Consumer detail offers a diagnosis hint with conclusions but no actions.
- **Actual path out**: fix the consumer app; inspect messages via SDK/nats CLI. **No shipped sidestep tooling.**

### Scenario 2: Dead letters (7 messages in dead-letters)
- **Detection**: ✔ the DLQ diagnostics view provides six evidence classes (target declaration, Stream, controller state...).
- **Handling**: ✖ explicitly no replay/move/acknowledge (documented boundary). Replay requires hand-written SDK code.
- **Extra obstacle**: DLQ counters (`rjs_dlq_*`) are process-global aggregates — **"which queue's dead-letter transfers are failing" is unanswerable**; the `RabbitJetStreamDeadLetterFailures` alert is likewise global-only.

### Scenario 3: Queue deletion (full walkthrough executed)
- **Experience**: ✔ preflight (ownership matching / 0 messages / 1 consumer) → three-step confirmation → evidence download; five steps, solidly misuse-proof.
- **Gap**: ✖ the preflight **does not check DLQ dependents** — if queue A declares deadLetter → B, deleting B raises no warning and A's dead-letter moves start failing (discoverable only later via the global DLQ failure counter); the "unknown outcome" path (audit 503) has no shipped runbook for manual investigation.

### Scenario 4: Diagnostics bundle (created and downloaded for real)
- **Experience**: ✔ create → manifest table (5 sources) → two-step download, done in 30 seconds; sanitized, excludes messages and credentials, boundaries clearly stated.
- **Friction**: two-step download plus the line "Server completion does not prove the browser saved the file"; jobs die on restart and expire in 10 minutes — unfriendly to cross-shift handover (recommend "forward the file to a person before it expires").

### Scenario 5: Session expiry (happened for real during the review)
- **Observed**: ✖ on token expiry the page degrades to a half-dead state: primary navigation gone, no re-login entry, a long paragraph instructing the user to "record necessary evidence before clearing the session", and the editor draft's internal JSON (`{"name":"","subjects":"",...}`) rendered on the page. The user must happen to know to click "Clear local session" in the top bar.
- **Impact**: with a 15-minute default token TTL this is a high-frequency event, and today it plays as a usability incident.

### Scenario 6: Audit troubleshooting (verified)
- **Experience**: ✔ the correlated query window genuinely filters server-side (tested `resource=orders-primary` returns exactly 4 matching events with a filter echo); the client adds a second validation and errors out rather than displaying wrong data — good engineering.
- **Friction**: ◐ one window scans at most 256 sequences with no match totals (anti-fishing cost: no confirmation that "nothing exists earlier"); JSON export is a manual two-step.

---

## 4. Decision items for the technical manager

### D1｜P0: the management process crashes without Prometheus
- **Fact**: in a deployment without `RJS_PROMETHEUS_URL`, after a few minutes of logged-in browsing the management process panics and exits — top frames `management/internal/prometheus/alerts.go:49` ← `management/internal/api/events.go:227` (alert watcher nil dereference). Happened during this review; pointing the URL at an unreachable address avoids the crash (error path instead).
- **Decision**: fix the nil guard + regression test now (hours of work). Until merged, every deployment must set `RJS_PROMETHEUS_URL` explicitly.

### D2｜The message-level operations decision can't wait any longer
- WEB-029 (message diagnostics) / WEB-030 (purge/replay/retry) have sat at P2 "needs separate architecture approval" for a while. This is not a missing page — it is an **operating-model gap**: without it, "poison-message cleanup, test-data reset, dead-letter replay" all depend on per-team hand-written SDK scripts of uncontrolled quality.
- **Recommendation**: approve a "controlled data-path tool" track outside the management plane (SDK/CLI form, sanitized, audit-hooked), or explicitly declare "this product will never provide this; operations must self-build" in the runbook. Both beat today's ambiguity.

### D3｜The alerting loop does not close
- Six shipped rules are only definitions: no Alertmanager config, routes, or notification templates; the console alerts page explicitly sends nothing. The DLQ failure alert is a global signal with no queue attribution.
- **Recommendation**: ship a baseline Alertmanager config (webhook/email examples) + per-queue DLQ metrics (label `rjs_dlq_failed_total` by queue or add per-queue counters).

### D4｜Half-dead session-expiry state
- On expiry: no navigation, no re-login entry, draft JSON rendered (Scenario 5). **Recommendation**: keep navigation, show a primary "sign in again" action, hide draft details.

### D5｜Complete the deletion safety model
- Add DLQ-dependent checks to the delete preflight (which queues declare deadLetter → target); provide a one-page runbook for the unknown-outcome path (what to check, whom to ask, how to capture evidence).

### D6｜Release engineering hygiene
- Current code is newer than the rc.2 frozen candidate; the 1M-message load test, 24h soak, and 100k-consumer gates bind to **different revisions and hardware**; `docs/management-api.md` still calls the UI read-only; the backlog (WEB-017/022/024/026 implemented) is not backfilled.
- **Recommendation**: re-freeze an exact candidate + regression before rc.3; one alignment pass over doc statuses (half a day).

---

## 5. Stability and security observations (measured this session)

1. **P0 panic**: see D1.
2. **Role granularity**: only operator/auditor; the operator simultaneously holds queue writes, diagnostics, audit reads, and `access:manage` — no "queue admin without account management" or "read-only audit on-call" split. This becomes a friction point past ~5 team members.
3. **Token management**: `RJS_ADMIN_TOKEN` is a static env var, never expires (`expires_at: null`), no rotation path; local-account tokens use a sane 15-minute TTL. Provide expiry and rotation.
4. **Done well**: keys never in URLs/storage/logs (token verified memory-only); argon2id local accounts; audit pairs intent/outcome including failed attempts; diagnostics bundles are sanitized and exclude messages/credentials; tenant-level connection pools and isolation.

---

## 6. Automation and IaC readiness

- ✔ OpenAPI spec published (`/api/v1/openapi.yaml`) with Go/TS codegen wired (oapi-codegen / openapi-typescript);
- ✔ The declarative CLI (`queue plan/diff/reconcile/apply/delete`) directly supports GitOps: declarations in repo, `diff` in CI, `apply` after approval — rare maturity for this product class;
- ✔ Good three-layer parity (console/API/CLI): no console-only operations;
- ✖ No Terraform provider / Kubernetes Operator integration (queue declarations can't enter IaC);
- ✖ No event webhook push (alerts and audit changes are pull-only);
- ⚠️ Backup/migration is a single-host CLI toolchain (file journal, local locks); scheduling/service-ization is self-build.

---

## 7. Action checklist for the technical manager

**Immediate (this week)**
1. Fix the D1 panic (nil guard + regression test); until merged, deployments must set `RJS_PROMETHEUS_URL` explicitly.
2. Fix the D4 session-expiry half-dead state (keep navigation + re-login button + hide drafts).
3. Publish an "operations companion checklist": Prometheus + Alertmanager baseline config, sample SDK/nats-CLI sidestep scripts for message-level troubleshooting, a `rjsctl backup` cron example, and on-call runbooks for the backlog/DLQ/deletion scenarios.

**Short term (1 month)**
4. Per-queue DLQ failure metrics + shipped Alertmanager config (D3).
5. DLQ-dependent check in the delete preflight (D5).
6. One documentation hygiene pass: backlog backfill, `management-api.md` correction, rc.3 re-freeze (D6).

**Mid term (1 quarter, architecture decisions required)**
7. Decide the WEB-029/030 message-level tool track (D2) — a controlled CLI/SDK beats a WebUI here.
8. Tenant lifecycle API/UI (today: file edit + restart).
9. Finer-grained roles (at minimum split `access:manage` out of operator).

**Long term**
10. Terraform provider / Operator; event webhooks; backup/restore orchestration (WEB-033).

---

## Appendix A: environment

Same as the [product review appendix](../admin-ui/design-review.md#appendix-environment-and-reproduction): local NATS (4222/8222) + locally built `rjs-management` (8223), 6 queues seeded, ~700 messages, one pull consumer; the diagnostics bundle, delete preflight, audit filtering, and session expiry were all really exercised this session.

## Appendix B: audit filtering verification (avoiding a misjudgment)

- `GET /api/v1/audit` (legacy endpoint): pagination only (offset/limit); filter parameters are ignored — consistent with C-07 "correlated audit queries still proposed";
- `GET /api/v1/audit/windows` (what the console actually calls): server-side filtering **works** (tested precise resource matching with a filter echo), and the client adds a second validation against non-matching echoes (`admin-ui/src/audit-list.mjs`).
- Conclusion: console audit filtering functions correctly; the legacy endpoint's lack of filters is a known API boundary, not a UI defect.

## Appendix C: review boundaries

- Not covered: multi-node failover drills, OIDC/SSO end-to-end, `rjsctl migrate` cutover against a real RabbitMQ, operations paths under helm/k8s deployment.
- Capability boundaries cited here follow the docs (`docs/webui-dlq-diagnostics.md`, `docs/webui-diagnostic-jobs.md`, `docs/webui-operational-alerts.md`, `docs/webui-rabbitmq-requirements.md`, etc.); where implementation and docs conflict, measurement wins and is flagged in the text.
