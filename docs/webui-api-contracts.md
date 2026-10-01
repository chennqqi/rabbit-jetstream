# WebUI API integration contracts

[English](webui-api-contracts.md) | [简体中文](webui-api-contracts.zh-CN.md)

Date: 2026-09-09. Version: 0.6. Status: **session identity, Consumer reads, advisory preview and lossless editable Queue readback implemented in source, unreleased; other extensions remain proposals**.

This refines C-01–07 in the [design plan](webui-design-plan.md) and the [mutation recovery prototype](webui-mutation-design.md). The initial API baseline, proposed changes, and implementation updates are separate. Version 0.1 changed documentation only; v0.2 adds exact Consumer detail source, OpenAPI and tests. Production UI, frozen artifacts and remote environments remain unchanged. One owner may approve the decisions, but approval does not replace tests.

## Deletion preflight implementation update

Operator-only `GET /api/v1/queues/{queue}/delete-preview` and its single-attempt candidate UI are implemented in source. See [the contract and safety limits](webui-delete-preview.md). The historical baseline below predates this addition. Unknown outcomes remain locked with read-only evidence; durable resolution and complete deletion UX qualification remain pending. No release promotion is implied.

## 1. Verified v0.1 API baseline (see C-06 for the v0.2 addition)

Source: [HTTP handlers](../management/internal/api/handler.go), [OpenAPI](../api/openapi.yaml), [backend](../management/internal/jetstream/client.go), [declarations](../internal/topology/declaration.go), [resource models](../management/internal/jetstream/models.go).

| Current route | Actual result/use | Important limit |
| --- | --- | --- |
| `GET /api/v1/info` | Name/version/uptime, redacted NATS URL, account resource totals | Not identity, permissions, intended deployment or qualification; error can be `{error: string}` |
| `GET /api/v1/cluster` | Redacted server URL and account statistics | Not cluster membership or cluster health |
| `GET /api/v1/nodes` | Monitoring snapshot and per-node evidence/errors | Targets are configured monitoring endpoints, not authoritative membership |
| `GET /api/v1/queues` | Declaration page `{items,total,offset,limit}` | No global search/filter/sort contract; no observed Queue health |
| `GET /api/v1/queues/{queue}` | Declaration plus quoted KV revision in `ETag` | Not the original editable Queue document or a fresh resource observation |
| `GET /api/v1/streams` and `GET /api/v1/streams/{stream}` | Stream page or exact observed Stream | No per-response observation timestamp in the resource model |
| `GET /api/v1/streams/{stream}/consumers` | Observed Consumer page | No exact Consumer GET; enumerates all Consumers before sorting/slicing |
| `PUT /api/v1/queues/{queue}` | Conditional apply; `ReconcileResult` on success or blocked result | Synchronous write attempt, not a `202` asynchronous job; not a health guarantee |
| `DELETE /api/v1/queues/{queue}` | Conditional delete; `DeleteResult` | Counts are not atomic empty/unused protection; no preview endpoint |
| `GET /api/v1/audit` | Intent/outcome event page, newest sequence first | No resource/request/actor/time filters or exact event lookup |
| `GET /api/v1/controller` | Controller-level status | Not per-Queue convergence evidence |

Resource reads require operator or auditor authentication by default; the explicit loopback-only local-demo profile is the sole anonymous exception. Writes require operator, while audit permits operator or auditor. If all static role tokens and the OIDC verifier are absent, guarded capabilities return their disabled 404 response. Otherwise missing/invalid bearer returns 401 `unauthorized`; a well-formed but expired local access token returns 401 `token_expired` so the console can label expiry precisely instead of assuming it; a disallowed role returns 403 `forbidden`. The public `/api/v1/oidc/config` and `/api/v1/oidc/token` endpoints implement the stateless browser Authorization Code + PKCE entry/callback contract when configured; they create no cookie or server session and return no refresh token.

Default pagination: offset 0, limit 50, limit range 1–200. Resource pages clamp offset beyond total to total; audit paging retains its requested offset. Unknown query parameters currently are not validated as supported filters. Never send `q` to a legacy server and claim it searched globally.

## 2. Field mapping and numeric fidelity

| UI concept | Current source | Required interpretation |
| --- | --- | --- |
| Conditional-write version | Quoted `ETag` from Queue GET, based on `kvRevision` | Keep as an opaque string; original value remains fixed while editing |
| Desired content version | Declaration `revision`, `plan.revision` | Content-derived string; not KV version and not an incrementing counter |
| Declaration time | `appliedAt` | Not a fresh Stream/Consumer observation time |
| Queue Stream | `plan.stream.name` | Resolve from plan, not from an account-wide search |
| Managed Consumers | `plan.consumer` plus `plan.priorityConsumers[]` | Priority 0 is in the primary Consumer; maxPriority 7 means eight Consumers on one Stream |
| Expected ACK / observed ACK | `plan.consumer.ackWaitNanos` / observed `ack_wait_nanos` | CamelCase plan versus snake_case observation; never overwrite one with the other |
| Expected/observed limits | Plan `maxAgeNanos/maxBytes/maxMessages`; Stream `max_age_nanos/max_bytes/max_messages` | Age limit is not retention policy; normalize only documented unlimited/default semantics |
| Consumer filters | Observed `filter_subjects`, with `filter_subject` fallback | Use plural when nonempty, otherwise singular; missing filter is not a fabricated Queue Subject |
| Replica health | Independent `current`, `offline`, `lag`; leader separate in `cluster.leader` | Online is not necessarily current; absent cluster data is not an empty healthy R3 set |
| Backlog | Stream `messages`; Consumer `pending`, `ack_pending` | Name the source Consumer; do not sum unrelated scopes or only loaded rows |
| Delivery evidence | Consumer `delivered`, `redelivered`, `waiting` | Delivered is a sequence, not completed business work; no invented rates or online-client counts |

Plan/observed int64 and uint64 JSON numbers can exceed JavaScript safe integers. The production transport must retain exact number tokens **before** ordinary `JSON.parse` loses precision; converting an already rounded number to BigInt does not repair it. Keep ETag as text. Any proposed string-valued exact fields must be additive or versioned; do not silently change existing v1 types. Schema-driven editors must preserve labels, routing, optional priority (including explicit zero), DLQ and large limits. Existing `queueFromPlan` in [management.js](../admin-ui/dist/management.js) is not proof of lossless round-trip fidelity.

## 3. Response-to-UI contract (current wire behavior)

| Response/evidence | UI state and action |
| --- | --- |
| 200 apply with `blocked:false`, `status:ready` or `noop` | Apply returned successfully. Separately read declaration and resources. `ready` is reconciliation-plan status, not healthy/fully converged. `noop` can leave KV revision unchanged. |
| 200 apply without ETag | Follow-up declaration read did not supply one. Do not invent `old+1` or immediately enable another conditional write; GET Queue again. |
| 409 with `ReconcileResult.blocked:true` and `operations` | Safety/transition block. Display affected operations/reasons. This is not necessarily a stale edit; no force-update bypass is defined. |
| 409 with `error.code=conflict` | May be existing create target, changed KV revision, **or resource lock contention**. Preserve draft, reload current declaration and compare; do not label every conflict as a changed field or auto-retry. |
| 409 `name_mismatch` | URL/document inconsistency; fix request, not rebase. |
| 400 validation/confirmation/pagination error | Correct fields/request; preserve user input. Current messages are not structured field-error maps. |
| 401 / 403 / guarded 404 disabled | Reauthenticate or explain denied/disabled capability; do not show an empty resource or automatically retry the write. Never diagnose expiry solely from 401. |
| 404 `not_found` during reads | Missing requested resource, not zero counts. For writes inspect context: a missing dependency and missing Queue are not necessarily the same situation. |
| 428 `precondition_required` | Client must obtain a valid original ETag or use create-only `If-None-Match: *`; not permission to retry unconditionally. |
| 503 `audit_unavailable` during mutation | Shared code covers missing audit backend, failed intent **before mutation**, and failed outcome **after a mutation attempt**. Current machine-readable code alone cannot prove no write. Treat as uncertain unless reliable additional evidence resolves it. Do not branch on English message text. |
| Other write 5xx, disconnect, timeout, malformed/unrecognized result | Potential partial/unknown outcome; preserve operation context, inspect, never automatically replay the request. An audit outcome `failed` does not prove rollback. |

The error decoder must preserve status, headers and raw-safe body distinctions, accepting the normal `{error:{code,message}}`, legacy string errors and non-JSON transport failures. Never reduce everything to a thrown message and lose `operations` or correlation. `X-Request-ID` is echoed after audit-intent handling starts, not guaranteed on all responses. Caller-supplied IDs must be 1–128 ASCII letters/digits or `-_.:`; generate a fresh ID per explicit attempt and keep it in memory before sending. It is **correlation, not idempotency**. Same ID does not make repeating PUT/DELETE safe.

## 4. Extension contracts — implementation updates noted per contract

Names below are concrete proposals for implementation review. Do not call or advertise them until implemented and described in the shipped OpenAPI/capabilities.

### C-01 · Identity and exposure

**v0.6 update:** authenticated `GET /api/v1/session` reports verified actor/role, role permissions, known OIDC expiry and the current resource-read policy. See [identity semantics](webui-session.md). Browser Authorization Code + PKCE now uses this existing memory-only session model; static bearer entry remains available for recovery and local administration.

Propose `GET /api/v1/session` returning authenticated `actor`, `role`, `permissions`, and expiry only when known. Invalid credentials return 401, forbidden capability 403; no frontend token decoding as authority. Static tokens need not have an expiry. Reuse operator/auditor, do not invent a viewer role.

Protected non-loopback reads and an explicit local-demo read exception remain the D-05 security decision, **not enabled by this document**. Define policy separately for API resource reads, `/info`, bootstrap/capabilities, static assets, health/readiness and metrics. A bootstrap response may expose non-sensitive capability/schema identifiers, never account resources or credential material. Sign-in refresh retains the memory draft; explicit session clearing warns then clears sensitive state. Test actual handlers directly, not just disabled UI controls.

### C-02 · Capabilities and canonical editable document

Schema publication and consumption update: the authenticated all-field typed authoring schema, content revision binding, Settings display and preview/pre-dispatch schema reads are implemented. See [schema contract](webui-queue-schema.md). Release-bundle manifest loading and exact runtime version/revision/WebUI binding are also implemented; signature verification and final release-approval evidence remain separate. Earlier references below to missing schemas describe the original gap.

Receiving-instance update: capabilities ETag and optional `X-RJS-If-Capabilities-Match` are now enforced on both previews and Queue mutations. Candidate UI requires support; legacy clients remain compatible without the header. 400 denotes malformed conditions and 412 a receiving-instance mismatch before audit/backend work. This is not binary/cluster attestation. See [current details](webui-capabilities.md).

**Implementation update:** the additive `document` field on Queue GET is implemented with strict complete-Plan round-trip validation; unrepresentable declarations return null plus `document_error`. See [editable document semantics](webui-queue-document.md). Authenticated console capabilities, explicit configured deployment intent, capability-bound preview/pre-dispatch checks and shared-read notifications invalidating retained unsubmitted reviews are implemented in the local candidate; see [capability semantics](webui-capabilities.md). Notifications do not unlock pending/unknown outcomes. Complete schemas and release-qualification manifest integration remain pending.

Propose `GET /api/v1/console/capabilities`: `schemaVersion`, `features`, server-declared deployment profile (`standalone|cluster|unknown`), supported values and independently qualified profiles. Current parser accepts replicas 1/3/5 and priority 0–255; the current production qualification boundary remains linux/amd64, three-node R3, priority 0–7, at-least-once. Neither a reachable-node count nor parser support establishes qualification. Missing capability data means unknown, not a guessed production default.

Define a versioned full Queue schema and exact editable-document read, preferably an additive normalized `document` on declaration GET. It must be generated by a proven inverse mapping or preserved canonical declaration, not lossy JS reconstruction. Legacy records that cannot round-trip must be explicitly non-editable until resolved. Normalized defaults are shown separately from original omission semantics; no requirement to recover original whitespace. Current storage/ACK/max-delivery defaults are file/30s/5; replicas are required by Queue validation rather than independently guessed by the browser.

### C-03 · Global list query

Extend advertised list capabilities with `q`, `sort`, `order` and route-specific filters. Define `q` as trimmed case-insensitive literal substring (no regex): Queue/Stream names; Consumer name or any effective filter Subject. Consumer mode accepts `pull|push`; omitted means all. Initial sort is name ascending with stable identity tie-breaker. Filters precede ordering and offset/limit. Validate supported parameters/values rather than silently ignoring them; retain existing basic pagination compatibility.

Complete responses retain `{items,total,offset,limit}` and may add query/observation metadata. `total` is filtered total, never loaded-row count. For a future explicitly partial envelope use `complete:false` and unknown total, not an apparently authoritative number. Existing v1 list failures remain errors until partial semantics are implemented. Offset paging is not a snapshot under resource churn: explain changes, deduplicate identities and recover an empty last page without infinite requests. Small response size alone does not bound backend enumeration; indexed/cached or targeted implementation and cancellation must be load-tested before claiming scale readiness.

### C-04 · Observation evidence

Implementation update: node responses now include per-source read evidence. Reported varz numeric scalars and jsz memory/storage/streams/consumers/messages/metadata cluster size/pending preserve explicit zero; omitted or null source fields remain absent, including on failed jsz reads. Source availability means HTTP/JSON decoding succeeded, not that every field exists. The candidate renders absent/invalid numeric fields as unknown. JetStream enabled inference, route collection completeness, source identity correlation and the complete observation/health/freshness contract still need separate qualification. The paragraph below records the original gap, not the current numeric serialization behavior.

Propose `GET /api/v1/queues/{queue}/observation` with desired revision reference, required resource set, per-source timestamp/status/errors and explicit health reasons. Distinguish response fetch time from source observation time and from declaration `appliedAt`. Health (`unknown|missing|degraded|reconciling|healthy`) and freshness are independent; keep confirmed degradation when other evidence is absent. Unknown/unsupported fields must not default to zero.

Current node fields may use `omitempty`; some successful zero metrics are omitted, while failed `/jsz` can leave a zero-valued struct. Therefore absence/default values alone cannot distinguish measured zero from unavailable data. Snapshot `available` with total zero is not proof of a healthy cluster; `total` counts configured monitor endpoints. Add per-varz/routez/jsz provenance before interpreting those metrics generally. Console stale threshold 30 seconds and normal refresh 10 seconds are display policy, not Broker health thresholds. Preserve last success on failure; ignore obsolete responses after resource/query changes; pause/resume hidden tabs without falsely fresh labels.

### C-05 · Preview, write phases and reconciliation

**Candidate update:** `POST /api/v1/queues/{queue}/preview` and real editor integration are implemented. See [preview semantics and verification](webui-preview.md). Preview is read-only and operator-authorized; blocked plans return an advisory 200 response. Optional typed [receiving-attempt phase/effects](webui-mutation-evidence.md) are now implemented, displayed and exported. They do not resolve durable original-request outcomes or authorize replay; C-05 is not closed by this addition.

Propose operator-only `POST /api/v1/queues/{queue}/preview` with the full Queue document and original create/edit precondition. Return canonical document, plan, impact operations, base ETag/content revision, warnings and observation time. **No Stream/Consumer/metadata/audit-intent creation or update during preview.** Any draft, base revision or capability/schema change invalidates the preview. Apply revalidates independently; preview is not a reservation and never freezes live counts.

Keep existing conditional PUT. Extend errors additively with trustworthy phase/effect metadata: e.g. `phase=validation|audit_intent|precondition|apply|audit_outcome`, `effects=none|possible|confirmed`, and a conflict kind where known. Only emit `none` when proven; timeout ambiguity remains possible. Do not change 200 into a job protocol without a separately versioned decision. Typed fields should remove ambiguity rather than asking the browser to parse error prose.

Delete remains separate: typed exact-name confirmation, original ETag, force off by default, ownership and advisory impact evidence. Existing backend blocks non-owned Streams and nonempty Streams without force, but does not provide an atomic “still empty/no Consumers” guarantee. UI must not offer that guarantee. New data between preview and delete and failures after Stream deletion require explicit tests. This document does not authorize destructive API calls.

### C-06 · Related resources and exact Consumer lookup

Propose `GET /api/v1/queues/{queue}/consumers` and `GET /api/v1/streams/{stream}/consumers/{consumer}`. The first resolves the declaration plan and ownership metadata before filtering/paging, returns expected identity and observed status separately, and includes primary plus additional priority Consumers exactly once. Missing expected resources have known identity but unknown metrics; additional resources are not silently treated as managed. The second is a bounded exact lookup, independent of loaded pages or full-account enumeration; a mismatched route returns missing rather than another Consumer with the same name in a different Stream.

**v0.2 implementation update:** the second route is now registered in source; the Queue-scoped collection remains proposed. See the [implementation and verification record](webui-consumer-detail.md). This does not close all of C-06 or change D-05 access policy.

**v0.3 update:** the Queue-scoped collection is also registered, with expected/observed separation, explicit metadata ownership, q/mode filtering before name-sorted pagination, bounded enumeration and declaration revision checks. The linked record specifies limits and error behavior. Global list queries, live qualification and real UI integration remain pending; neither all of C-03 nor WP-02 is complete.

Keep DLQ dependencies distinct from Queue-owned Stream membership. Priority omitted versus explicit zero must survive all adapters. Do not substitute normal declaration Subjects for generated priority Subjects. An external Stream's Consumers may legitimately lack Queue ownership; that does not by itself mean the Consumer is invalid.

### C-07 · Correlated audit query

Extend advertised audit queries with exact resource kind/name, actor, request ID and UTC time interval (inclusive start, exclusive end); propose exact lookup by event ID. Pair outcome with intent via **`intentId`**, not solely request ID: callers can reuse correlation IDs. Newest-first browsing needs an opaque cursor/high-water boundary and stable sequence ordering with defined behavior for retention gaps, new events and invalid cursors. Filter/lookup must be bounded/indexed; do not scan all retained audit data in the browser. If an exact filtered total cannot be computed cheaply, explicitly return unknown total rather than fabricate it.

Current intent `revision` is `create` or original KV revision; apply outcomes can replace it with desired content revision, including failed attempts; delete outcomes can retain the original precondition. Display phase-qualified legacy revision. Propose separate `expectedKvRevision` and `desiredContentRevision` fields; do not assert either is a resulting committed version. Missing intent/outcome, denied reads and unavailable storage are distinct from no events. A truly absent audit Stream currently returns an empty page; empty history does not prove no historical mutations beyond retention or absence of storage.

## 5. Acceptance matrix and implementation order

The IDs below are **required cases**, not a claim they have all been implemented. Existing tests are supporting evidence only for behavior they actually exercise.

| Test ID | Required assertion | Contract |
| --- | --- | --- |
| API-01 | Protected anonymous reads denied; local-demo exception explicit; probes/metrics follow their separate policy | C-01 |
| API-02 | Operator/auditor/invalid token/disabled capability differ; 401 not automatically “expired” | C-01 |
| API-03 | Unknown deployment does not choose R1/R3 from reachable count; supported versus qualified values separate | C-02 |
| API-04 | Queue document round-trip preserves labels, bindings, DLQ, omitted/zero priority and exact large values | C-02/05 |
| API-05 | 201 resources: item outside page 1 can be found; total reflects full query; stable ordering | C-03 |
| API-06 | Invalid filters rejected; unsupported legacy server never masquerades as filtered | C-03 |
| API-07 | Deleted last-page item, churn, cancellation and late response do not show the wrong resource | C-03/04 |
| API-08 | Zero monitor targets, unavailable source and measured zero never collapse to healthy/zero | C-04 |
| API-09 | Online-but-not-current replica, missing leader evidence, stale success and mixed failures remain distinct | C-04 |
| API-10 | Priority 0–7 includes all eight Consumers on one Stream; missing expected identity carries no invented count | C-06 |
| API-11 | Exact Consumer lookup reaches page-independent identity; same name in another Stream cannot match | C-06 |
| API-12 | Preview leaves broker resources, metadata and audit-intent count unchanged | C-05 |
| API-13 | Original quoted ETag is retained; noop may keep revision; 200 without ETag triggers readback | C-05 |
| API-14 | Create collision, changed revision, lock conflict, name mismatch and blocked operations receive distinct recovery | C-05 |
| API-15 | Repeated submit dispatch causes one client request; repeated request ID is not claimed idempotent | C-05/07 |
| API-16 | Audit-intent failure has zero backend write calls; audit-outcome failure may follow a completed write | C-05/07 |
| API-17 | Partial broker write or lost response preserves uncertainty; no automatic replay or claimed rollback | C-05 |
| API-18 | Conflict/access recovery preserves draft and requires independent review; session clear warns and erases it | C-01/05 |
| API-19 | Fresh messages between delete impact and submission are not protected by declaration ETag | C-05 |
| API-20 | Audit pairing uses intent ID; phases distinguish revision meaning; missing outcome remains unresolved | C-07 |
| API-21 | Audit paging remains defined under inserts/retention gaps; filters precede paging, incomplete total labeled | C-07 |
| API-22 | All documented response codes, envelopes, headers and schemas match handlers, not only registered paths | All |

Implementation order: (1) settle D-05 exposure and deployment-source policy; (2) versioned schemas/capabilities and exact related reads; (3) query/observation contracts; (4) non-mutating preview and typed mutation errors; (5) indexed/correlated audit queries; (6) real-API UI integration and a new candidate's regression. This is not authorization to deploy or replace the frozen candidate.

Current [OpenAPI tests](../api/contract_test.go) verify embedding, route inventory, operation IDs and references, not complete response semantics. OpenAPI uses generic response objects, omits some actual 403/disabled 404 responses, and does not fully describe ETag/correlation or the legacy info error. Update shipped schemas and add handler/schema equivalence tests **with implementation**, not by documenting proposed endpoints as already available.

## 6. Initial v0.1 verification (historical)

Read local handlers, models, reconciliation, conditional-write/locking, audit, monitoring, existing UI adapter and OpenAPI tests. Ran locally with no cached test results:

```powershell
go test -count=1 ./management/internal/api ./management/internal/jetstream ./management/internal/monitoring ./internal/topology ./api
```

All five packages passed. This verifies existing tests, not future extensions, deployment safety or production scale. Existing tests specifically cover KV ETag, correlated audit intent/outcome, intent failure before write, outcome failure after write, role authorization and API path documentation. No SSH, container changes, credentials, live mutations or new public endpoints were used.
