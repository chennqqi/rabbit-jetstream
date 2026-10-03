# WebUI design plan — operational console

[English](webui-design-plan.md) | [简体中文](webui-design-plan.zh-CN.md)

Date: 2026-09-09. Version: **0.2, Queue detail visual direction selected; contracts remain under review**.

## 1. Mandate and scope

The owner requested formal WebUI planning after the [RabbitMQ capability research](webui-rabbitmq-requirements.md). This plan turns that backlog into page specifications, interaction contracts and delivery gates. Planning is authorized and the Queue detail visual direction is now selected. Implementation completion and release approval have **not** happened. The same owner can review and accept each gate; no additional approvers are required.

Primary user: the project owner acting as operator/developer, with occasional read-only diagnosis and audit work. Primary outcomes: declare a Queue without memorizing JSON, locate backlog evidence, change configuration safely and explain what happened after an operation. This is a desktop-first embedded administration application, not a marketing website or an AMQP compatibility project.

Scope baseline: **WEB-001–016 (P0)**. Retain the existing single-account boundary and native SDK semantics. P1 remains WEB-017–027 and WEB-034; P2 remains WEB-028–033. No scope is silently removed from the research backlog.

Out of this increment: history charts, global connection/Consumer searches, payload access, purge/replay, bulk changes, multi-account management, restore, federation and browser SSO. P0 still includes secure read/write/audit policy and an in-memory token session; SSO is not required to make authorization work.

Do not change frozen release artifacts or the running Docker/bare-metal qualification environment. Future builds remain local. UI/backend changes create a new candidate with their own regression evidence; the existing soak cannot qualify changed binaries.

## 2. Design grounding and working decisions

Source evidence: [existing styles](../admin-ui/dist/styles.css), [editor styles](../admin-ui/dist/management.css), [UI packaging](../admin-ui/README.md), [embedded handler](../admin-ui/embed.go), [Queue contract](../internal/topology/queue.go), [HTTP handlers](../management/internal/api/handler.go). This is code-grounded planning, not a screenshot-based audit. There was no saved Product Design context; the repository and prior research are the working context.

| Decision | Working default | Status / reason |
| --- | --- | --- |
| D-01 product identity | Retain dark-green navigation, light work area and existing semantic colors | Proposed continuation, not a new brand direction. |
| D-02 page model | Full pages for details and editing; dialogs only for short confirmations | Proposed: long JSON/detail dialogs are unsuitable for comparison and recovery. |
| D-03 delivery model | Embedded static assets, same-origin versioned API, no CDN or new production frontend server | Preserve existing deployment/CSP boundary. |
| D-04 frontend tooling | Owner confirmed React/Vite | Build embedded static assets locally; no additional runtime server or CDN. Production integration remains outstanding. |
| D-05 access | Owner confirmed authenticated resource reads by default | Source now enforces authentication; explicit literal-loopback-only `RJS_LOCAL_DEMO` exception. See [access policy](webui-access.md). |
| D-06 deployment intent | Server-declared standalone/cluster profile; unknown intent requires replica selection | Proposed backend contract; reachable-node count is not intent. |
| D-07 language | Browser-language initial selection, Chinese/English switch, remember only nonsensitive preference | Proposed; technical identifiers are not translated. |
| D-08 release boundary | New WebUI work independent of frozen rc.2 acceptance | Planning constraint; no remote process changes. |

## 3. Information architecture and screen inventory

Use `/admin/` history-based routes, building on the existing app-shell fallback. Load assets with stable `/admin/`-rooted paths so deep-link refreshes work. Route patterns below are **proposed UI routes**, not HTTP API endpoints. Encode path segments; store page/filter/tab state in the URL, never tokens or unsaved configuration. Legacy `#overview`, `#queues`, `#nodes` links should map to their new pages.

| Screen | Proposed route | Main content and actions | Requirement mapping |
| --- | --- | --- | --- |
| S-01 Overview | `/admin/overview` | Deployment identity, health evidence, totals, issues, last successful observation; navigate to affected resources | WEB-001, WEB-008, WEB-012, WEB-015 |
| S-02 Queues | `/admin/queues` | Server search/filter/sort, authoritative total, paginated table, create; columns: name, observed health, stored messages, consumer count, storage, replicas, revision | WEB-002, WEB-009 |
| S-03 Queue detail | `/admin/queues/by-name/:queue` | Summary / Configuration / Routing / Consumers / Events tabs; edit and isolated danger section | WEB-005, WEB-007, WEB-009, WEB-013 |
| S-04 Create Queue | `/admin/queues/new` | Guided form, expert JSON, explicit deployment context, validation and plan preview | WEB-003, WEB-004, WEB-005 |
| S-05 Edit Queue | `/admin/queues/by-name/:queue/edit` | Immutable name, original revision, dirty form, change preview, conflict comparison | WEB-003, WEB-005, WEB-006 |
| S-06 Streams | `/admin/streams` | Paginated read-only resource list, managed/external classification | WEB-002, WEB-010 |
| S-07 Stream detail | `/admin/streams/:stream` | Retention, limits, subjects, stored state, replica evidence, paginated Consumers | WEB-010 |
| S-08 Consumer detail | `/admin/streams/:stream/consumers/:consumer` | Durable/filter/ACK/delivery settings, pending/ack backlog/redelivery with metric explanations | WEB-009, WEB-010 |
| S-09 Nodes | `/admin/nodes` | Node list and fetch errors; select a node by stable ID | WEB-011 |
| S-10 Node detail | `/admin/nodes/:node` | Version, uptime, CPU/memory, connections, JetStream state and observation timestamp | WEB-011 |
| S-11 Audit | `/admin/operations/audit` | Paginated intent/outcome events, actor/resource/time filters when server-supported, correlated event detail | WEB-013, WEB-015 |
| S-12 Access and settings | `/admin/settings` | Token session/clear action, effective permissions when server-provided, exposure mode, deployment profile, language, refresh preference | WEB-004, WEB-014, WEB-016 |

Cross-cutting WEB-001/012/014/015/016 apply to every screen, not only the rows naming them. Queue Events means correlated **management audit events**, not message-delivery history. Queue Routing in P0 is a read-only plan table; the interactive routing workspace is P1.

Route decision: `new` is a legal Queue name today, so resource details use the explicit `by-name` prefix. `/admin/queues/new` opens creation; `/admin/queues/by-name/new` opens the Queue named `new`. This preserves legal names without reserving them silently.

## 4. Page composition and field behavior

### Overview and list/detail hierarchy

Persistent header: product, deployment name/profile, observation time, refresh, language and access entry. Single-account context is a label, not a nonfunctional tenant switcher. Navigation groups: Overview; Resources (Queues, Streams, Nodes); Operations (Audit); Settings.

Overview prioritizes problems, then resource totals and node summaries. Do not add a rate chart or sparkline without historical data. A total whose source is incomplete displays an incomplete-data label, not the number of locally loaded rows. Current issues are observations, not delivered alerts. Recent audit information is conditional on audit access; lack of access is not “no changes”.

Queue list uses a compact toolbar and stable header, a 50-row default page and a maximum API page size of 200. Search/sort/filter operate on the full dataset before pagination; unsupplied server features cannot masquerade as global filters. Keep name/state visible at narrow widths and allow explicit horizontal scrolling for secondary columns. A text link opens details; row click is optional, not the only keyboard-accessible action.

Queue detail header shows name, declaration revision, observed health and freshness separately. Summary links the declaration to generated resources; Configuration shows units/default sources; Consumers explains pending versus ack-pending; Routing shows supported binding-to-subject mappings; Events links to the same filtered audit dataset. Editing belongs on a full page, not in a detail popup.

### Create/edit form contract

Mandatory related-resource logic (review follow-up): Queue Consumers is collection-first. Resolve the plan's one Stream and managed Consumers (primary plus additional priority Consumers), distinguish expected/missing/external resources, then filter/sort before paging. Identity is `(stream, name)`. Detail/return/deep links retain query/page and require bounded exact lookup independent of loaded pages. Define no resources, no matches, forbidden, missing, partial and stale states separately. Overview metrics identify their Consumer; never pick an arbitrary first record or sum the displayed page. Desired configuration, accepted declaration and observed resources are separate objects. Failed reads cannot advance observation time. KV revision/ETag is distinct from content-derived plan revision. Canonical exact comparison prevents equivalent-value writes. These rules block production closure of C-03–07; [review progress](webui-logic-review.md) tracks the independent mock implementation.

| Group | Fields and interactions | Validation / preservation |
| --- | --- | --- |
| Identity | Name, labels, deployment profile | Name uses existing alphanumeric/underscore/hyphen contract; name locked for edits; labels survive round trips. |
| Routing | Explicit choice: subjects OR bindings; repeating entries; binding exchange/type/keys | At least one mode has content; switching modes with content requires confirmation; fanout has no keys; backend validates wildcard semantics. |
| Storage and retention | Replicas, file/memory, age/byte/message limits with unit inputs | Separate configured value from default/unlimited meaning; do not silently clamp or infer new production qualification. |
| Delivery | ACK wait, maximum deliveries, optional priority and optional DLQ target | Distinguish omitted priority from explicit zero; show unresolved DLQ target as a validation issue according to backend rules, not invented success. |
| Review | Canonical declaration, generated-resource summary, old/new diff, warnings | Compare semantic values, not whitespace; show every default applied; preview must not mutate resources. |

Store a single canonical draft in memory. Form and expert JSON are two views, not competing sources. Invalid JSON stays recoverable; do not silently discard unsupported fields. If a field cannot be represented, block form conversion and explain it. Preserve large integers without unsafe JavaScript rounding; validate exact serialization against Go parsing. Defaults come from a versioned server contract or an explicitly version-pinned shared schema, not independently guessed frontend constants.

Create uses `If-None-Match: *`; a name collision never silently becomes update. Edit stores the original declaration and its ETag. Background refresh must not overwrite a dirty draft or advance its precondition. Route changes with unsaved changes require confirmation. Authentication expiry keeps the draft in page memory, but explicit session clear discards sensitive drafts after warning; do not persist drafts or tokens.

### Mutation state model

| State | Visible behavior | Allowed next step |
| --- | --- | --- |
| Editing | Draft and original revision | Validate/preview, or discard. |
| Reviewing | Canonical diff, impact evidence and observation time | Submit once or return to edit. |
| Submitting | Disabled submit, progress text | Await result; closing the page is not a cancellation guarantee. |
| Accepted, observing | Accepted request with separate convergence status | Poll observed state without resubmitting the write. |
| Conflict | Base/local/current comparison | Explicit rebase, revalidate and review; never silently merge incompatible changes. |
| Rejected | Field/global error and preserved draft | Correct input or permissions. |
| Outcome unknown | Explain that the write may have occurred; show correlation ID when available | Inspect resource and audit; no automatic mutation retry. |

Delete is a distinct flow: load ownership/configuration revision and current impact → show exact name and observed data → typed confirmation (force off) → conditional delete → inspect result/audit. A configuration ETag does **not** freeze live message/consumer counts. Changed declarations must conflict; a refreshed impact change requires reconfirmation. Strong “delete only if still empty/unused” guarantees require enforceable backend/broker semantics, not a frontend timestamp. Until defined, label counts advisory and describe the race; do not claim atomic no-loss deletion.

## 5. Health, permissions and failure states

Health and freshness are separate axes. Freshness proposal: observation older than 30 seconds is stale; use observation time plus elapsed time, handle client/server clock skew, and label this as a console freshness threshold, not a broker health threshold. The normal refresh remains 10 seconds, with one refresh batch in flight and error backoff capped at 60 seconds. Hidden-tab pause must not leave old observations looking current.

| Evidence | Primary state / presentation |
| --- | --- |
| No successful evidence or insufficient required sources | Unknown; identify missing sources. |
| Confirmed missing managed resource | Missing; name the missing resource. |
| Confirmed failed required replica/resource | Degraded; retain the evidence even if another source is unavailable. |
| Desired versus observed plan differs without confirmed failure | Reconciling; show the difference and observation time. |
| All required checks passed | Healthy, qualified by freshness; never infer solely from Stream existence. |
| Last success is old or latest refresh failed | Stale/unavailable overlay with last-known state; not a replacement green result. |

States on each page: initial loading; truly empty; no search matches; forbidden; not found; partial failure; stale success; complete failure. Each has distinct copy and recovery. Do not use skeleton loading after a successful page refresh; retain useful data with freshness indicators.

Proposed authorization matrix (requires backend implementation; current read APIs are not generally protected):

| Action | Unauthenticated protected deployment | Auditor | Operator |
| --- | --- | --- | --- |
| Resource reads | Deny | Allow | Allow |
| Audit reads | Deny | Allow | Allow |
| Queue create/update/delete | Deny | Deny | Allow |
| User/account administration | Not offered | Not offered | Not offered |

No new viewer role is implied. A local-demo exception may allow resource reads only, never bypass writes/audit. Probe/metrics exposure needs an explicit route policy so adding console auth does not accidentally break readiness or scraping. `/info` and identity bootstrap must define which non-sensitive fields may be unauthenticated. Backend authorization is authoritative; never decode a token in the browser and treat its claims as verified permissions.

## 6. Backend contracts to settle before implementation

These are **contract work items, not new endpoints already available**. Reuse existing routes where compatible; publish actual schemas and error examples before implementation.

The [API integration specification](webui-api-contracts.md) now records code-verified current behavior, C-01–07 extension proposals and 22 acceptance cases. WP-02 remains open pending policy decisions, executable schemas and implementation-aligned fixtures; this specification does not establish that the proposed endpoints exist.

| Contract | Minimum decision / evidence | Blocks |
| --- | --- | --- |
| C-01 Identity and exposure | Effective identity/permissions, read policy, local-demo exception, probe/metrics separation, expiry errors | WEB-014; safe non-loopback exposure |
| C-02 Deployment/capabilities | Intended mode, allowed versus qualified replicas/priorities, canonical defaults, schema version | WEB-003/004 |
| C-03 List queries | Search/filter/sort before paging, stable tie-breaker, total semantics, query limits, cancellation; acknowledge resource churn rather than promise snapshots | WEB-002/013 |
| C-04 Observation | Per-resource health reasons, required evidence set, observation timestamp, source errors; distinguish unsupported from zero | WEB-008/009/011 |
| C-05 Preview and concurrency | Non-mutating normalized plan/diff, original revision, unsafe transitions, preview expiry/invalidation, final mutation preconditions | WEB-005/006/007 |
| C-06 Related-resource reads | Direct Queue-to-Stream mapping and bounded Consumer lookup; targeted pagination; no full-account list download for a detail page | WEB-009/010 |
| C-07 Audit query | Resource/actor/time filters, correlation, outcome uncertainty, redaction, pagination under new events | WEB-013/015 |

Existing audit detail and Consumer detail may initially be resolved through bounded collection queries; if the UI cannot reach a specific record reliably, add an explicit lookup contract. Do not download every event/Consumer to simulate a detail endpoint. C-03 must assess actual backend enumeration cost, not just enforce a small JSON response.

Suggested frontend responsibilities, independent of framework choice: API/auth transport; route and URL state; query/freshness cache; canonical Queue schema adapter; mutation state machine; locale/formatting; reusable table/form/status/error primitives; page composition. Same-origin API only, no browser NATS access, no service worker or persistent cache containing credentials/resource data. Future asset changes must also address the existing one-hour asset cache through versioned filenames or equivalent cache invalidation.

## 7. Visual system brief for the next stage

Preserve existing tokens as the starting point: ink `#15211e`, muted `#66726e`, paper `#f6f8f6`, surface `#ffffff`, line `#dce4e0`, navigation `#102a25`, action green `#13795b`, warning `#a15c00`, danger `#b42318`. These are extracted values, not a claim of verified contrast in every combination. Keep system-font fallbacks; use tabular numbers and monospace identifiers. Do not require remote fonts or decorative image assets.

Existing component vocabulary: top bar, sidebar, panel, table, badge, field, confirmation dialog. Reuse it with consistent sizing and focus states rather than inventing unrelated page styles. Suggested density tokens for visual review: 4/8/12/16/24/32 spacing, 14-pixel table text, 16-pixel body text, 44-pixel default table rows, 8-pixel controls and 12-pixel panel corners. These dimensions are proposals, not approved screenshots.

First visual review set: **Queue list, Queue detail, Create/Review**, with normal, empty, loading, forbidden, degraded/stale and conflict examples. Use clearly labeled synthetic identifiers/data, never live payloads or tokens. Review at 1440 and 375 pixels; do not hide access controls on mobile as the current token field does. At narrow widths, collapse navigation while keeping identity/access available.

The owner allowed code-grounded visual generation without capturing the running UI. The initial comparison mixed list/detail/create workflows and was unsuitable for choosing layout. A second comparison held Queue, data, tabs and actions constant. The owner explicitly selected `exec-f508c114-b351-4751-aac4-b6f9f2d6627c`, the primary/supporting two-column Queue detail layout. See the [selected design and implementation handoff](webui-selected-design.md). This is a generated visual target, not browser-rendered implementation evidence. Other pages and backend contracts remain to be specified; production assets are unchanged.

## 8. Work packages, gates and acceptance

| Package | Deliverable / dependencies | Covered requirements | Exit evidence |
| --- | --- | --- | --- |
| WP-01 Design baseline | This plan, scope decisions and unambiguous routes; no coding prerequisite | WEB-001–016 | Owner records reviewed scope and unresolved decisions. |
| WP-02 Contract baseline | C-01–07 schemas, authorization matrix, errors, test fixtures | WEB-002/003/004/005/007/008/013/014/015 | Backend/frontend examples agree; unknown capabilities cannot produce success. |
| WP-03 Visual/interaction specification | Selected direction, three core screens and failure states; uses WP-01/02 | WEB-001/003/005/006/007/009/016 | Owner selects direction; contrast, focus, narrow layouts reviewed. |
| WP-04 Read console | Shared shell, lists/details, observation and access; WP-02/03 | WEB-001/002/008/009/010/011/012/014/015/016 | Deep links, 201-resource coverage, partial failures, negative authorization tests. |
| WP-05 Safe mutations and audit | Form/JSON, preview, conflict/delete flows, audit; WP-02/03/04 | WEB-003/004/005/006/007/013/014/015/016 | Concurrent edits and uncertain outcomes verified with real API behavior. |
| WP-06 Qualification and handoff | Local builds, regression, bilingual docs, screenshots; WP-04/05 | All P0 | New candidate evidence; no claim inherited from unchanged old soak. |

Sequence: WP-01 → WP-02 and WP-03 (iterate together) → WP-04 → WP-05 → WP-06. These are work packages for one owner, not a staffing or calendar estimate. Current status: **WP-01 draft produced; WP-03 Queue detail direction selected, other screens/states pending; WP-02 and WP-04–06 not completed**. No implementation percentage is assigned.

Required fixtures: standalone demo intent/R1; healthy three-node/R3; degraded three-node with only one reachable node; 201 Queues; malformed configuration; name `new`; priority omitted versus zero; large exact integers; external Stream; two simultaneous editors; delayed/failed APIs; expired/forbidden credentials; missing audit outcome; new messages between delete preview and submit.

Release gates inherit the [research acceptance targets](webui-rabbitmq-requirements.md): all five operational journeys, Chromium/Firefox, keyboard/accessibility, Chinese/English, 375/1440 layouts, unauthorized direct requests, and no token persistence. Proposed scale fixtures remain 10,000 Queues / 100,000 Consumers; list API p95 ≤ 1 second and first usable list ≤ 2 seconds require a named load/hardware profile. Those are unmeasured targets, not current achievements. Audit failures and partial broker errors must be exercised against actual service behavior, not only mock screenshots.

Plan validation performed for this document: repository-source review, local Markdown link checks and bilingual screen/contract/work-package identifier consistency. No browser tests, service changes or production code modifications are part of this planning deliverable.

## 9. Immediate handoff

Use this plan as the implementation issue outline, with WEB IDs as traceability keys. Next, resolve D-05/C-01 (access) and D-06/C-02 (deployment intent), then produce the visual review set in section 7. A single recorded owner decision is sufficient; unresolved security/contract guarantees remain explicit blockers for their dependent implementation, not blockers to drafting the screens.
