# WebUI logic review

[English](webui-logic-review.md) | [简体中文](webui-logic-review.zh-CN.md)

Date: 2026-09-09. Verdict: **not ready for functional-design acceptance**. Review only; no application or production changes.

## Scope and evidence

### Follow-up implementation, 2026-09-09

Latest follow-up: [mutation recovery design](webui-mutation-design.md) now demonstrates conflict, forbidden/expired, unknown, partial and missing-audit recovery. Verified totals are 26 browser checks, 15 model tests, 4 packaging tests and 15 accessibility states without automated violations. This advances mock L-03/L-06/L-09/L-10 coverage; real contracts remain open. Earlier iteration counts below are historical.

After this audit the owner requested implementation. The following progress is **isolated-prototype only**, not production closure. Original findings and screenshots below are retained as historical evidence.

| IDs | Implemented and tested | Still open |
| --- | --- | --- |
| L-01/02 | Consumer list, full-fixture name/Subject/mode filtering, five-row paging, selected detail/return, named overview metric link | Real server queries, bounded lookup and ownership |
| L-03 | Separate desired/observed configuration; accepted change waits for successful mock refresh; failure retains old values/time | Real partial/rejected/uncertain writes |
| L-04 | Independent current/catching-up/offline/unknown adapter and unit tests | Live replica and leader evidence |
| L-05 | Standard, priority 0 and priority 0–7 fixtures with actual name/Subject shapes and one Stream; generated routing | Actual plan/metadata association and external resources |
| L-06 | Supported mock routes retain query/page/detail on return, reload and browser history; list breadcrumb fixed | Production history routes and full dirty-draft navigation matrix |
| L-07 | Exact nanosecond comparison, equivalent-value no-op, zero max age, removed arbitrary 1000 limit | Full versioned schema/Go duration grammar and larger backend integers |
| L-08 | Stale labeling, failed/forbidden/missing mock reads, refresh cancellation on fixture/read-state change | Per-source backend times, network response races and real permissions |
| L-09/10 | `queue.apply` session label, accepted-not-observed result, explicitly mock KV revision | Real authorization, audit, concurrency, content revision and delete |

Verification: **19 browser checks, 8 model tests, 4 packaging tests passed; zero automated accessibility violations in 11 scanned states**; build and formatting passed. [Current results](../design-prototypes/webui/evidence/results.json). Open [Consumer list](http://127.0.0.1:18224/#consumers), expand Mock scenarios, select priority 0–7, then page/search/open/return. Browser reload resets mock edits and read scenarios; the URL retains only safe navigation/fixture state. **The ten findings are not all production-closed.**

Reviewed the isolated Queue prototype at `http://127.0.0.1:18224/`, the [design plan](webui-design-plan.md), and current local Queue/API contracts. The Product Design audit workflow required fresh screenshots and interaction evidence; these were captured with the owner's authorized local headless Chromium. This is not a live broker, production security, or whole-console acceptance test. No remote host or Docker service was changed.

### Captured journey

1. Queue overview — readable evidence, but Consumer metric ownership is ambiguous.
2. Consumer tab — incomplete: filtering a hardcoded detail instead of navigating a collection.
3. Edit and review ACK wait — before/after is clear; impact and failure branches are absent.
4. Apply mock edit — declaration and observed Consumer configuration are incorrectly conflated.
5. Return to Queue list — list renders, but URL/breadcrumb remain on the detail and reload restores it.

Screenshots below were captured and opened in this review. They are local ignored evidence, not committed release artifacts.

![1. Overview](../design-prototypes/webui/evidence/logic-audit/01-overview.png)
![2. Consumers](../design-prototypes/webui/evidence/logic-audit/02-consumers.png)
![3. Review](../design-prototypes/webui/evidence/logic-audit/03-review.png)
![4. Applied mock edit](../design-prototypes/webui/evidence/logic-audit/04-applied.png)
![5. Queue list](../design-prototypes/webui/evidence/logic-audit/05-list.png)

## Findings and required acceptance

Severity: P1 = incorrect operational conclusions or blocked core workflow; P2 = misleading navigation/review. “Contract gap” means not yet demonstrated, not a reproduced production bug. All findings remain open.

### L-01 · P1 · Consumer collection is missing — confirmed, step 2

[App.jsx](../design-prototypes/webui/src/App.jsx) renders `orders_worker` directly and filters that single name. There is no collection, selection, paging, or distinction between an empty collection and unmatched results. The plan lists S-08 detail but does not sufficiently specify the Queue Consumer sub-list.

Acceptance: Queue → associated Consumer list → whole-dataset filtering → paging → selected detail → return with query/page retained. Show total, stable identity `(stream, name)`, filter subjects and delivery mode; distinguish empty, no matches, forbidden, missing and failed reads. A record on page 2 must be discoverable without downloading all records. Current [handler](../management/internal/api/handler.go) only offers offset/limit; [ListConsumers](../management/internal/jetstream/client.go) enumerates then sorts before slicing. Server filters and bounded direct lookup remain C-03/C-06 work, not existing capabilities.

### L-02 · P1 · Overview backlog has no named Consumer scope — confirmed, step 1

The 8,420 pending and 240 ack-pending values say “Consumer” without identifying which Consumer; 12,480 is Stream storage. “Do not add” is correct but does not establish ownership. More Consumers would make the default selection arbitrary.

Acceptance: display the selected Consumer identity beside its metrics and link to that exact record, or show per-Consumer rows. A Queue aggregate needs explicit membership, completeness and overlap semantics; no sum of only the loaded page. No Consumer, missing Consumer and unknown evidence must not become zero. Do not interpret delivered sequence as completed business messages.

### L-03 · P1 · Desired configuration overwrites observed state — confirmed, steps 3–4

After changing ACK wait from 30s to 45s, `apply()` changes `config`; the Consumer detail reads `config.ackWait` and immediately shows 45s with the unchanged observation timestamp. This depicts convergence without observing it. The mock is clearly labeled, so no real resource was changed, but the interaction model is unsuitable for real integration.

Acceptance: separate original declaration, draft, accepted declaration and observed resources. Show affected managed Consumers in the review. Accepted/reconciling/partially applied/failed/unknown outcomes must remain distinct; only fresh evidence can change observed values. Actual [apply code](../management/internal/jetstream/client.go) writes a Stream, then Consumers, then persists the declaration; partial progress is possible. Preserve drafts on failure; never automatically retry an uncertain mutation.

### L-04 · P1 · Replica states collapse “not offline” into “current” — confirmed code-path limitation, step 1

The prototype branches exclusively on `offline`. The real [Replica model](../management/internal/jetstream/models.go) has independent `Current` and `Offline` fields. An online-but-catching-up replica would be presented as current if this adapter were reused. This state is not in the present fixture; it was not reproduced against a broker.

Acceptance: cover current, catching up, offline and unknown, without inferring health solely from reachability. Keep leader/replica evidence separate from cluster-wide node health; do not fabricate leader Lag. Unknown/offline Lag remains unknown, not zero.

### L-05 · P1 · Priority and managed-resource identity need a concrete design — contract/fixture gap, steps 1–2

[Plan construction](../internal/topology/plan.go) creates **one Stream**, `plan.Consumer` for priority 0 and `plan.PriorityConsumers` for remaining priorities. With maxPriority 7 this means eight Consumers, not eight Streams and not just the additional seven. A normal managed Consumer is named `RJSQC_<queue>`; the mock `orders_worker` does not establish whether it is external or managed.

Acceptance: consume the actual plan and ownership metadata; do not infer ownership solely from names. Cover omitted priority, explicit zero, 0–7, missing expected Consumers and unexpected observed resources. Clearly distinguish expected managed resources from other observed resources. Resolve membership before filtering/paging. Do not invent overlapping same-subject workqueue Consumers just to fill a table. Routing must distinguish declaration subjects from generated priority subjects; DLQ dependencies are not the Queue's own Stream.

### L-06 · P2 · List/detail navigation is not restorable — reproduced, step 5

From `#consumers`, click sidebar Queues: the list appears but the URL stays `#consumers` and breadcrumb still includes `orders_events`. Reload returns to Consumer detail. `choose()` uses `replaceState`; page state is memory-only.

Acceptance: route expresses list/detail identity; URL holds safe tab/query/page state. Back/forward, reload, copied deep links and return-from-detail restore the intended resource and context. Unsaved drafts must be guarded. Do not put tokens/drafts in URLs. The plan already specifies this; the prototype has not implemented it.

### L-07 · P2 · Review compares strings, not configuration meaning — reproduced model behavior, step 3

[configDiff](../design-prototypes/webui/src/model.js) reports `30s → 0.5m` as a change although both are accepted and equivalent. Applying it can increment the mock revision and append an event. Max-age validation also rejects zero although the Queue validator allows nonnegative retention limits; maximum delivery 1–1000 is explicitly prototype-only, not a backend limit.

Acceptance: canonical, precision-safe comparison; equivalent values produce no mutation. Define omitted/default/zero/unlimited separately per field. Bind constraints to the versioned Queue schema. Keep Stream retention policy (`workqueue`) distinct from max age (`24h`). Show unsupported form conversions without losing input.

### L-08 · P1 · Freshness and asynchronous failure behavior are unproven — coverage gap, steps 1–4

Refresh only updates a mock clock after 600ms. There is no stale overlay, source-specific failure, out-of-order response, forbidden or not-found simulation. The design plan already requires these states; happy-path screenshots do not satisfy that requirement.

Acceptance: stale last success remains visible and labeled; unsuccessful reads do not refresh its timestamp. Cover partial Stream/Consumer/node failures, old responses arriving after resource/filter changes, hidden-tab resume, no successful evidence and resource disappearance. Loading, zero, missing and unavailable are distinct. Accessibility tests must include announcing dynamic errors and restoring focus, not only static contrast.

### L-09 · P1 · Mutation/audit/access recovery lacks an executable design — coverage gap, steps 3–4

The mock has one successful update path and session events labeled `queue.update`; the real handler records `queue.apply` intent/outcome. Access is a simulated operator, not a permission check. The plan correctly includes conflict/unknown outcome and authorization; these are still unresolved implementation gates, not discovered production failures.

Acceptance: original ETag precondition; explicit conflict comparison; expired session with draft retention; forbidden writes; request accepted but response lost; audit outcome write failure after resource mutation; success followed by incomplete observation. Events must use actual audit action/correlation/outcome semantics. “No permission” and “audit unavailable” cannot appear as an empty history. Delete stays blocked until its separate impact/confirmation/race contract is exercised; simplified single-owner approval does not remove these safeguards.

### L-10 · P1 · Revision labels must distinguish two backend concepts — contract gap, steps 1, 3–4

The mock has one integer “declaration revision” incremented locally. Backend [Plan.Revision](../internal/topology/plan.go) is a content-derived string, whereas the [HTTP declaration ETag](../management/internal/api/handler.go) is the KV revision used for conditional writes. Treating these as interchangeable risks false drift or incorrect preconditions.

Acceptance: name and retain both separately where needed: KV revision/ETag for concurrency, plan revision for desired-resource comparison. Pass the original server ETag unchanged; never fabricate it by incrementing a displayed number. Demonstrate different KV revisions and equal/different content revisions in contract fixtures.

## What is already sound, and limits

The plan already prohibits first-page-only global filters, blind metric addition, automatic uncertain-write retries, token persistence and frontend-only authorization. It explicitly separates health/freshness and warns that delete counts are not atomic guarantees. The prototype labels its mock scope, displays unknown offline Lag as a dash, and provides before/after review. Preserve these decisions.

Previous visual QA or happy-path interaction passes are not functional-design approval. This review did not exercise production APIs, concurrent real writers, permissions, delete, large datasets, or screen readers; screenshots cannot prove those guarantees. Unimplemented Streams/Nodes/Create/Audit screens received contract review only, not a screen-flow pass. No blanket claim that every design defect has been eliminated is justified.

## Closure gate

Before integration, attach to every screen: resource scope and identity, complete state matrix, navigation contract, data-source/units/defaults, permissions, mutation outcomes and executable acceptance cases. Use L-01–L-10 as closure IDs. First fix resource identity, list/detail flow and desired/observed separation; then add failure fixtures and real API contract tests. No P1 finding may be waived merely because the owner is the only approver.
