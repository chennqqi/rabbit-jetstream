# WebUI requirements research: RabbitMQ Management benchmark

[English](webui-rabbitmq-requirements.md) | [简体中文](webui-rabbitmq-requirements.zh-CN.md)

Research date: **2026-09-09**. Status: **proposed backlog, not approved release scope**.

Next-stage artifact: [Formal WebUI design plan](webui-design-plan.md) — page specifications, interaction contracts and delivery gates; draft, not implementation completion.

## 1. Conclusion and evidence boundary

The next WebUI iteration should become an operational console, not merely add more dashboard cards. Start with guided Queue creation, useful resource details, trustworthy state, pagination, safe changes and audit visibility. Then add historical metrics and connection diagnostics. Message inspection, replay, multi-tenancy and cross-cluster transfer require separate backend/security decisions.

This report combines RabbitMQ official documentation (the documentation navigation displayed **4.3** when retrieved) with repository code at `343ea2930519202f87732309f8007bafdd68d7ec`. It is not a hands-on test of a running RabbitMQ console. “Documented UI”, “plugin extension”, and “API/CLI capability” below must not be treated as interchangeable. Configuration, permissions and queue type affect availability. No particular latest patch version is asserted.

All priorities and acceptance criteria below are project proposals. This document does not change the frozen rc.2 runtime, the 24-hour qualification workload, or the existing first-release qualification boundary. It does not promise RabbitMQ protocol compatibility.

## 2. RabbitMQ capability inventory

| Area | Officially documented capability and boundary | Implication for our WebUI |
| --- | --- | --- |
| Overview | Management exposes broker/resource statistics and recent activity. Its history is not a long-term monitoring store. [Management](https://www.rabbitmq.com/docs/management) | Add useful operational summaries and a separate history source. |
| Nodes and resource pressure | Memory/disk alarms can block publishing; connection states help diagnose this. These are RabbitMQ-specific semantics. [Alarms](https://www.rabbitmq.com/docs/alarms) | Display real NATS health evidence; do not invent equivalent blocking alarms. |
| Connections | Client identity, connection state and connection churn support diagnosis. [Connections](https://www.rabbitmq.com/docs/connections) | Add connection search, client metadata and node association. |
| Channels | AMQP channels multiplex operations over a connection. [Channels](https://www.rabbitmq.com/docs/channels) | Do not create a fictitious NATS “channel” page. |
| Queues | Queue type, lifecycle, limits and delivery characteristics matter operationally. [Queues](https://www.rabbitmq.com/docs/queues) | Make configuration understandable through forms and explicit units. |
| Quorum queues | Current documentation includes UI visibility for priority counts and delayed-retry state. These are type/version-dependent features. [Quorum queues](https://www.rabbitmq.com/docs/quorum-queues) | Show per-priority/retry diagnostics only where our contracts support them. |
| Streams | The UI can declare a stream via queue type selection; stream retention and offsets differ from destructive queue consumption. [Streams](https://www.rabbitmq.com/docs/streams) | Expose JetStream Streams separately from business Queues. |
| Consumers | Acknowledgement, prefetch, activity and capacity support backlog diagnosis. [Consumers](https://www.rabbitmq.com/docs/consumers) | Show pending, acknowledgement backlog, redelivery and delivery configuration with correct meanings. |
| Exchanges and bindings | Routing supports exchange types and bindings; not every type has a direct NATS equivalent. [Exchanges](https://www.rabbitmq.com/docs/exchanges) | Visualize supported Queue routing plans, not imaginary standalone exchange resources. |
| Priorities | Current quorum queues support strict priorities with 32 levels; older guidance that quorum queues lack priorities is obsolete. [Priority](https://www.rabbitmq.com/docs/priority) | Keep our qualified priority range separate; no inferred compatibility. |
| Dead lettering | Dead-letter exchanges route messages after qualifying events. [Dead lettering](https://www.rabbitmq.com/docs/dlx) | Explain our destination Queue/controller model and failure evidence. |
| Policies | Matching rules, priorities and operator constraints centralize configuration; operator-policy modification can be disabled. [Policies](https://www.rabbitmq.com/docs/policies) | Consider declarative templates and constraints later, with precedence rules. |
| Virtual hosts | Virtual hosts scope resources and permissions and can impose limits. [Virtual hosts](https://www.rabbitmq.com/docs/vhosts) | Multi-account isolation is an architecture feature, not a dropdown-only task. |
| Users and authorization | User authorization includes resource configure/write/read permissions. Management tags and message permissions are distinct. [Access control](https://www.rabbitmq.com/docs/access-control), [Management](https://www.rabbitmq.com/docs/management) | Design API enforcement separately from visible buttons. |
| SSO | OAuth integration includes management login configuration. [OAuth](https://www.rabbitmq.com/docs/oauth2) | Backend JWT validation alone does not supply a browser login flow. |
| Definitions | Topology/configuration export and import are supported. Definitions are not a message backup. [Definitions](https://www.rabbitmq.com/docs/definitions) | Separate declaration portability from full snapshot recovery. |
| Publish/get/purge | HTTP operations include publishing, delivery retrieval and purging Ready messages. Retrieving messages changes queue state even when requeued; these are troubleshooting tools, not passive browsing. [HTTP API](https://www.rabbitmq.com/docs/http-api-reference) | Do not add a supposedly read-only “peek” using a consuming endpoint. |
| Destructive operations | APIs include queue deletion conditions and connection closure. [HTTP API](https://www.rabbitmq.com/docs/http-api-reference) | Require scoped privileges, current impact preview and auditable confirmation. |
| Federation | Management UI functionality requires the federation management extension. [Federation](https://www.rabbitmq.com/docs/federation) | Treat cross-cluster federation as a separate product capability. |
| Shovel | A management extension supplies transfer status; configuration is related to dynamic runtime parameters. [Shovel](https://www.rabbitmq.com/docs/shovel) | Transfer jobs need workers, credentials and retry semantics, not just a page. |
| Tracing | The tracing plugin adds a GUI for trace capture; payload capture has performance and confidentiality costs. [Firehose/tracing](https://www.rabbitmq.com/docs/firehose) | Diagnostics must default to metadata and redaction. |
| Events versus audit | The event-exchange plugin publishes broker events. This is not by itself a durable, tamper-proof audit console. [Event exchange](https://www.rabbitmq.com/docs/event-exchange) | Build on our existing audit stream and describe its limits honestly. |
| Upgrade administration | Feature-flag operations are documented through administrative interfaces; a deprecated-features panel is explicitly documented. This report does not verify every flag has a GUI control. [Feature flags](https://www.rabbitmq.com/docs/feature-flags), [Deprecated features](https://www.rabbitmq.com/docs/deprecated-features) | Start with read-only version/capability visibility. Do not expose host upgrades as generic buttons. |
| Long-term monitoring | Prometheus/Grafana are recommended for production monitoring. [Monitoring](https://www.rabbitmq.com/docs/monitoring) | Link to or integrate an external metrics backend; do not imply built-in alert delivery. |

Do not copy legacy classic-queue mirroring settings into a new design. Likewise, CLI coverage is not proof of equivalent GUI coverage: [rabbitmqadmin](https://www.rabbitmq.com/docs/management-cli) is an HTTP API client, not a replacement for every node administration tool.

## 3. Actual implementation and gaps

Evidence: [HTML](../admin-ui/dist/index.html), [dashboard code](../admin-ui/dist/app.js), [Queue editor](../admin-ui/dist/management.js), [HTTP handlers](../management/internal/api/handler.go), [OpenAPI](../api/openapi.yaml), [existing browser tests](../tests/admin-ui/admin-ui.spec.js).

| Current implementation | Gap / next requirement |
| --- | --- |
| Embedded static `/admin/`; Overview, Queues and Nodes navigation; four summary cards | No dedicated Stream, Consumer, connection or audit workspace. |
| Queue JSON create/update, detail dialog, name-confirmed deletion | Add guided forms, validation, impact preview and field explanations; preserve expert JSON editing. |
| Queue and Stream lists request `limit=200`; local filtering and array-length counts | Results beyond the first page are absent, not nonexistent. Implement complete pagination and authoritative totals. |
| “Ready” is inferred from a matching Stream existing | Existence alone does not prove reconciliation, consumer availability or replica health. |
| New Queue template fixes `replicas: 3` | Single-instance demos need an explicit deployment profile. Never infer single-node mode from one surviving node of a degraded cluster. |
| Edit reads a declaration; save fetches a fresh ETag just before PUT | Static inspection identifies a lost-update risk: the edited document may be older than the newly fetched revision. Bind the precondition to the revision originally edited. |
| Deletion also fetches a fresh revision | Bind deletion to the resource revision/impact the user actually confirmed. |
| Dashboard refreshes every 10 seconds; partial errors handled with `Promise.allSettled` | Add cancellation, backoff, staleness labels, hidden-tab pause and protection against overlapping refreshes. No history exists today. |
| Operator token input; backend static-token/OIDC validation | No browser SSO flow or complete read-access policy. Resource read APIs are generally unauthenticated; operator/auditor checks cover writes/audit. Hiding controls cannot secure reads. |
| Backend audit endpoint and structured resource data exist | Add audit UI and richer details before inventing equivalent new services. |
| Existing tests cover CRUD/auth errors, accessibility and responsive scenarios | Extend tests to real revision conflicts, pagination, mixed failures and the proposed workflows; existing tests do not establish their readiness. |

Documentation drift: [management API documentation](management-api.md) still calls the embedded UI “read-only”, while current code implements writes. Align documentation as part of implementation acceptance.

## 4. Semantic mapping: borrow workflows, not incompatible objects

| RabbitMQ concept | Rabbit-JetStream interpretation / restriction |
| --- | --- |
| Queue | Business Queue declaration plus generated Stream/Consumer plan; show both desired and observed state. |
| Stream / consumer / connection | Distinct resource types. A durable consumer is not a continuously connected client and is not an AMQP channel. [NATS consumers](https://docs.nats.io/learn/jetstream/pull-consumers) |
| Exchange and binding | Existing `bindings` support direct/topic/fanout inside a Queue plan. Generated subjects are Queue-scoped; there is no independent exchange CRUD API. Wildcard translation requires contract validation. |
| Vhost | NATS accounts provide namespace isolation, but require real account/credential/controller design. A label is not isolation. [NATS accounts](https://docs.nats.io/learn/security/accounts-and-multitenancy) |
| Priority | Use the native SDK contract. First-release qualification is priorities **0–7**, three-node **R3**, at-least-once, **linux/amd64**; broader parser acceptance is not production qualification. |
| DLX | Current `deadLetter.queue` and controller transfers, not an AMQP exchange. Do not promise identical dead-letter headers, TTL or ordering semantics. |
| Definitions / backup | Declarative configuration portability versus message/consumer-state snapshots. Existing restore requires an empty account and stopped writers/controllers; no cross-stream atomicity guarantee. |
| Message testing | The management service must remain outside the message data path. Any future payload tool needs an approved separate SDK execution boundary. |

Contract evidence: [Queue schema](../internal/topology/queue.go), [plan generation](../internal/topology/plan.go), [migration boundaries](rabbitmq-migration.md), [production readiness](production-readiness.md), [backup/restore](backup-restore.md).

## 5. Proposed navigation

```text
Overview — health, resource totals, active issues, recent changes
Queues — search/list → Summary | Configuration | Routing | Consumers | Events
Streams — read-only list/detail; distinguish managed and external resources
Nodes — node/detail → Connections (P1)
Operations — Audit (P0) | Diagnostics and declaration export (P1)
Settings — identity/capabilities (P0) | SSO and access policy (P1)
```

Consumer details are initially reached through Queues/Streams; global Consumer search is P1. Historical charts are P1. Do not display nonfunctional navigation entries as completed capabilities.

## 6. Prioritized backlog and acceptance criteria

> **Status backfill (2026-10-01):** this backlog predates several implemented candidates and was not backfilled when they landed. Implemented in the current development candidate: WEB-017 (metric history), WEB-022 (DLQ diagnostics), WEB-024 (diagnostic jobs), WEB-026 (operational alerts projection), plus parts of WEB-002/003/005/007/010/012/018/019/021/023/025/027/034 tracked in `webui-development.md` and `webui-api-contracts.md`. On 2026-10-01 the review-hardening work (alert nil-backend crash fix, session-expiry state, pre-login OIDC config, per-Queue DLQ metrics, delete-preflight DLQ-dependency checks) landed on top; see `review-improvement-plan.md` for the full plan. Items WEB-028–033 remain unimplemented and require separate approval.

**P0:** next usable console increment. **P1:** operational depth after P0. **P2:** separately approved capabilities. Dependencies: **F** existing API/frontend work; **B** new or changed backend contract; **A** architecture/security decision; **O** external integration. Combined labels mean all apply. Even with one owner, operator and auditor remain permission concepts, not extra approval staff.

| ID | Priority / dependency | Requirement and minimum acceptance |
| --- | --- | --- |
| WEB-001 | P0 / F | Navigable console: stable detail URLs, browser back/forward, selected resource survives refresh; missing resources show a proper not-found state. |
| WEB-002 | P0 / F+B | Paginated Queue/Stream lists: use server totals; a 201-resource fixture is fully reachable; global filtering/sorting happens before pagination; changing a filter resets the page. |
| WEB-003 | P0 / F | Guided Queue form: subjects XOR bindings; name, storage, retention, acknowledgement, delivery limit, priority and DLQ validation; JSON/form round-trip preserves every supported field and label. |
| WEB-004 | P0 / B | Deployment-aware capabilities: explicit standalone/cluster profile and qualified limits; missing profile requires replica selection; a degraded three-node cluster never silently defaults to R1. |
| WEB-005 | P0 / F+B | Change preview: show current/proposed configuration and affected generated resources; distinguish safe updates from rejected changes; preview never mutates state. |
| WEB-006 | P0 / F | Correct concurrency: preserve ETag from editor load; conflict displays server/local differences and offers explicit reload/reapply; never silently replace the precondition with a fresh revision. |
| WEB-007 | P0 / F+B | Safe deletion: exact name, confirmed revision, managed-resource ownership and available backlog/consumer impact; force is off by default; changed impact requires reconfirmation; no automatic retry. |
| WEB-008 | P0 / F+B | Truthful health: distinguish unknown, stale, missing, reconciling, degraded and healthy; Stream existence is insufficient; unavailable evidence is not a green zero. |
| WEB-009 | P0 / F | Queue details: declaration revision, retention/limits, subjects, generated Stream, priority consumers, pending/ack backlog and replica leader/current/lag; every metric has units and a definition. |
| WEB-010 | P0 / F | Stream/Consumer inspection: dedicated read-only details using existing endpoints; identify externally managed resources and do not expose unsupported raw mutation controls. |
| WEB-011 | P0 / F | Node details: version, uptime, CPU, memory, connections, JetStream capacity fields and fetch errors; distinguish bytes used from an unknown host disk limit. |
| WEB-012 | P0 / F | Robust refresh: default 10 seconds, manual refresh, last-success time, one refresh in flight, cancel obsolete requests, pause hidden tabs and exponential error backoff capped at 60 seconds. |
| WEB-013 | P0 / F+B | Audit workspace: correlate intent/outcome and resource/actor/time; paginate; full-dataset filters need server support; ambiguous mutation outcomes display “verify before retry”. |
| WEB-014 | P0 / F+B+A | Access baseline: document and enforce the chosen read/write/audit exposure policy; unauthorized direct API access is tested; secrets never enter URLs, persistent browser storage or logs. |
| WEB-015 | P0 / F | Error recovery: distinguish validation, authorization, conflict, unavailable and uncertain outcome; provide actionable errors and correlation identifiers where supported; keep unaffected panels usable. |
| WEB-016 | P0 / F | Chinese/English and accessibility: translated navigation/forms/errors/units, keyboard-complete operations, dialog focus restoration and text alternatives to color-only states. |
| WEB-017 | P1 / B+O | Historical metrics: selectable 15-minute/1-hour/24-hour windows using a declared history backend; consistent timestamps; show gaps/resets instead of connecting fabricated values. |
| WEB-018 | P1 / B | Backlog diagnosis: combine pending, acknowledgement backlog, redelivery, consumer configuration and node health; recommendations cite evidence and avoid claiming a proven root cause from one gauge. |
| WEB-019 | P1 / B | Client connections: server-paginated search by client/node/identity and bounded subscription detail; redact sensitive metadata; no public browser access to NATS monitoring ports. |
| WEB-020 | P1 / F+B | Routing workspace: graph or table of Queue, binding, generated subject and Stream; validate supported matching without publishing test messages; flag unsupported translation. |
| WEB-021 | P1 / B | Global Consumer workspace: paginated filters for Queue/Stream/durable/status; inactive client connections do not by themselves mark a durable consumer broken. |
| WEB-022 | P1 / F+B | DLQ diagnosis: show configured target, target existence, controller health and transfer evidence; global counters are explicitly global; per-Queue history requires new instrumentation. |
| WEB-023 | P1 / B | Declaration import/export: schema/version validation, dependency preview, redaction, dry run and itemized outcomes; never advertise atomic rollback; clearly exclude message backups. |
| WEB-024 | P1 / B | Diagnostics download: reuse existing CLI redaction rules through a bounded job/API; metadata only by default, size/expiry limits and audited access; no arbitrary host paths or commands. |
| WEB-025 | P1 / B+A | Browser SSO/session: choose a reviewed OIDC login flow, token expiry/logout and server authorization; backend token validation is a dependency, not proof this UI feature exists. |
| WEB-026 | P1 / B+O | Operational alerts: source, threshold, duration and recovery state; link to external alert delivery unless a delivery service is separately built; no fake “notification sent”. |
| WEB-027 | P1 / F+B | Templates and bulk changes: inspectable templates, preview every target, bounded batch size and individual results; one failure cannot be presented as total success. |
| WEB-028 | P2 / A+B | Multi-account isolation: approved account selection/credential lifecycle, per-account cache and server authorization; cross-account negative tests must pass before exposure. |
| WEB-029 | P2 / A+B | Message diagnostics: separately approved SDK path, explicit state-change warning, payload limit/redaction, scoped permission and audit; no misleading read-only peek or management data-plane forwarding. |
| WEB-030 | P2 / A+B | Purge/replay/retry jobs: define ready/in-flight scope, ordering, duplication, rate limits, cancellation and idempotency; dry run and typed confirmation; never reuse Queue delete semantics. |
| WEB-031 | P2 / A+B | Policies/quotas: define precedence among template, Queue declaration and operator constraint; preview affected resources and reject unsupported live transitions. |
| WEB-032 | P2 / A+B | Federation/transfer: separately qualified workers, credential handling, failure/retry semantics and ownership; do not label an arbitrary NATS connection as federation. |
| WEB-033 | P2 / A+B | Backup/restore orchestration: preserve empty-account, stopped-writer and snapshot-consistency constraints; isolated recovery rehearsal before a UI restore operation is enabled. |
| WEB-034 | P1 / F+B | Compatibility page: runtime/build/SDK contract and supported versus qualified capabilities; no host upgrade, restart or feature-toggle button without a separately authorized API. |

P0 dependencies are deliberate: basic forms/details can start immediately, but broad network exposure must wait for WEB-014, trustworthy state for WEB-008, and safe writes for WEB-006/007. P0 does not require a metrics database or message playground.

## 7. API and metric design inputs

Existing routes are authoritative in [handlers](../management/internal/api/handler.go) and [OpenAPI](../api/openapi.yaml). The following extension descriptions are proposals, **not implemented endpoints**.

| Existing capability | UI use | Needed extension |
| --- | --- | --- |
| `GET /api/v1/info`, `/cluster`, `/nodes`, `/controller` | Overview, nodes, controller health | Explicit deployment profile, capability boundaries and reconciled health reasons. |
| `GET /api/v1/queues`, `/streams` | Lists | Global filters/sorts; assess backend enumeration cost, not just response size. |
| `GET /api/v1/queues/{queue}`; `PUT`/`DELETE` same path | Detail/edit/delete | Read-revision-bound preview and optional non-mutating validation contract. |
| `GET /api/v1/streams/{stream}` and `/consumers` beneath it | Stream/Consumer detail | Cross-Stream consumer index, bounded query and richer diagnosis if required. |
| `GET /api/v1/audit` | Intent/outcome inspection | Full-dataset filters and controlled export. |
| `GET /metrics` | Monitoring integration | Historical query/authentication boundary; browser must not scrape arbitrary URLs. |
| No current connection or import/diagnostic job API | New P1 pages | Design server-owned monitoring access and bounded jobs; CLI existence does not imply HTTP availability. |

Current collections use offset/limit (default 50, maximum 200), with `items`, `total`, `offset`, `limit`. Some server paths enumerate resources before slicing, so UI pagination alone does not prove scalability. Existing updates/deletes require preconditions; creation uses `If-None-Match: *`; update/delete use `If-Match`; deletion also requires `X-RJS-Confirm-Queue`. Preserve backend rejection of unsafe changes rather than auto-delete/recreate.

Metric rules:

- Stream stored-message count is not automatically “ready business messages”. Do not sum overlapping consumer views into a Queue total.
- Consumer `Delivered` is a consumer sequence, not successful business processing count. `Redelivered` must not be treated as a monotonic event counter without an explicit metric contract.
- Distinguish gauges, cumulative counters, rate windows, units, sampling time and process-reset behavior. Missing samples remain unknown.
- Node traffic rates are not automatically native SDK acknowledgement throughput. Priority-consumer aggregation needs a documented formula.
- Display replica current/offline/lag with observation time. Host filesystem capacity and file-descriptor usage need actual new evidence if not exposed today.
- NATS `/connz` can supply connection diagnostics, while `/routez` describes routes; route pooling means route connections are not a unique-peer count. Keep monitoring endpoints private and use scoped server-side queries. [NATS monitoring endpoints](https://docs.nats.io/learn/monitoring/monitoring-endpoints)

Implementation evidence: [JetStream client](../management/internal/jetstream/client.go), [monitoring client](../management/internal/monitoring/client.go), [audit guarantees](audit.md), [OIDC](oidc.md), [diagnostics](diagnostics.md).

## 8. Acceptance journeys and quality gates

1. **Create a Queue:** choose an explicit deployment profile; enter bindings or subjects, retention and delivery settings; preview; submit once; navigate to actual observed state. Unsupported settings fail before mutation; errors never erase the form.
2. **Investigate backlog:** open Queue → Consumer → Node; see pending versus acknowledgement backlog, observation times and incomplete evidence. The UI does not equate stored messages with failed processing.
3. **Concurrent edit:** two editors load the same revision; the first saves; the second receives a conflict and a comparison. The second cannot overwrite the first merely because the UI refreshed its ETag.
4. **Delete with changed impact:** preview one revision, modify it elsewhere, then confirm. The stale operation fails and requires renewed review; force remains explicit and off by default.
5. **Partial outage and uncertain mutation:** one endpoint fails while others remain useful; freshness changes visibly. If mutation outcome/audit recording is uncertain, instruct inspection rather than automatic resubmission.

Proposed nonfunctional test targets, to be frozen against a named hardware/load profile before implementation sign-off:

- Dataset: 10,000 Queues and 100,000 Consumers in fixtures; default UI page size 50 and server response cap 200. Verify backend bounded cost separately from mock UI tests.
- Performance: list/filter API p95 ≤ 1 second and first usable list ≤ 2 seconds on the agreed local test profile. Record dataset, concurrency, latency and cold/warm state with results; these are targets, not measured current results.
- Reliability: one dashboard refresh in flight; 10-second default refresh, error backoff capped at 60 seconds; no blanking healthy panels when another source fails.
- Accessibility: target WCAG 2.2 AA, no serious/critical automated findings, plus manual keyboard/focus tests. Test 375-pixel and 1440-pixel layouts and Chinese text expansion. [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- Browser coverage: keep Chromium and Firefox regression gates; add other browsers only with actual execution evidence. Never infer support from engine similarity.
- Security: direct unauthorized API requests, token leakage, HTML injection from resource names/metadata, cross-account isolation where applicable, and destructive-operation conflict paths have negative tests.
- Documentation: English/Chinese guides, API schema, screenshots and error examples match the shipped workflow. No placeholder controls or misleading empty-state success.

## 9. Delivery sequence and owner decisions

**Increment A — usable and safe (P0):** implement WEB-001–016; explicitly resolve access policy and deployment profile first; reuse existing details/audit APIs. Exit only when the five journeys above, API authorization tests and bilingual documentation pass.

**Increment B — operational depth (P1):** implement WEB-017–027 and WEB-034 after selecting metrics storage and bounded diagnostic/query contracts. Historical charts must use real time-series data; connection diagnostics must use protected monitoring access.

**Increment C — separately approved extensions (P2):** WEB-028–033. Each requires an architecture decision record and specific safety/qualification evidence. None is implied by “match RabbitMQ UI”.

The sole project owner can record scope selection and acceptance in the delivery issue/document; no multi-person approval chain is proposed. Logical roles still enforce least privilege. Avoid expanding the ongoing frozen release qualification simply because this backlog exists.

Decisions to record before their dependent work begins:

1. **Read exposure:** recommend authenticated console reads for any non-loopback deployment; retain a clearly documented local-demo exception only if explicitly selected.
2. **Deployment profile:** recommend server-declared standalone/cluster intent; never derive intent solely from currently reachable nodes.
3. **Metrics:** recommend external Prometheus/Grafana integration before building an internal time-series store; establish query authorization and retention.
4. **Tenant scope:** recommend keeping the first UI increment single-account; design actual isolation before adding account selection.
5. **Payload operations:** recommend excluding message bodies, purge and replay from P0/P1; approve a separate native-SDK execution design if later needed.

This research produced requirements only. It did not implement the features, execute competitor UI tests, or alter running Docker/bare-metal services.
