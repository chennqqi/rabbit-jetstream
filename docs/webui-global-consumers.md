# Global Consumer search design proposal

[English](webui-global-consumers.md) | [简体中文](webui-global-consumers.zh-CN.md)

Status: the owner approved the recommended instance-local generation model. The bounded collector, authorized API, global navigation/list UI and real-service Chromium acceptance are implemented. Target-scale performance measurement remains pending.

## Backend implementation progress (2026-09-11)

`GET /api/v1/consumers` now queries only an immutable completed generation and never initiates broker enumeration. `POST /api/v1/consumers/refresh` is an operator-only explicit refresh; concurrent refreshes coalesce. Collection uses 16 workers with a 30-second deadline and hard limits of 10,000 Streams, 100,000 rows and 128 MiB of encoded list rows. These are safety budgets matching the agreed scale target, not performance evidence.

The collector reads declarations before and after the complete Stream/Consumer traversal. It publishes only if every source succeeds and declaration identities/revisions remain unchanged. A failure cannot replace a complete generation; the previous generation becomes explicitly stale. With no complete generation, reads return unavailable/collecting rather than an empty success. Expected Consumers remain visible when their Stream is missing; observed external Consumers and repeated names on different Streams remain distinct. Exact zero counters are retained, while missing observations use null counters.

API, authorization, generation-conflict, complete-join, partial-failure and declaration-churn tests pass. The UI now provides the list before filtering, stable URL filters, Stream/name identity links, mode/state/order/page-size controls, generation-aware paging and explicit operator collection. Auditors can query completed generations but cannot initiate collection. Lossless decoding rejects partial or misleading rows. Final embedded Chromium evidence is `artifacts/webui-live-PI9GIr/report.json`: all 129 checks and 24 accessibility snapshots passed, including a dedicated global Consumer page audit. This is functional evidence at fixture scale, not the required 100,000-Consumer performance proof.

The repeatable [100,000-Consumer algorithm prototype](webui-global-consumer-scale.md) now exercises the exact collection and query code. It exposed and removed 100,000 temporary query allocations. The optimized 100,000-row scan/filter took 7.15–12.11 ms across eight one-shot samples with 22 allocations on the stated workstation; complete in-memory fixture collection took 149.55–184.85 ms and about 258 MB of allocations. Real broker/UI p95 qualification remains open.

## Requirement and verified gap

WEB-021 requires server-side search by Queue, Stream, durable identity and state, with filtering before pagination. The agreed scale target is 10,000 Queues / 100,000 Consumers; list API p95 ≤ 1 second and first usable list ≤ 2 seconds need measured hardware/load evidence, not just fixture rendering.

Current `management/internal/api/handler.go` exposes only Queue-scoped and Stream-scoped Consumer collections. `stream_consumers.go` filters one full Stream enumeration before paging. `jetstream/client.go` lists Streams and each Stream's Consumers separately. `queue_consumers.go` joins declared/observed identities for one Queue, has a 2,000-observation bound and rechecks its declaration revision. These are useful building blocks, not an existing global index. Calling them once per browser page would either miss matches or repeatedly enumerate the account.

## Alternatives requiring a decision

1. Recommended: instance-local, bounded in-memory snapshot index. Queries use an immutable completed generation, expose collection times and stale/error state, and never imply an atomic broker snapshot. Restart loses the index and requires collection again. No new external service or persisted credentials/data.
2. Per-request live enumeration. Less retained state, but every global filter/page can fan out over all Streams and declarations. Pagination can shift with churn; the latency and backend-cost targets remain unproven.

The index is proposed because repeated full scans are incompatible with a credible low-cost pagination design at the target scale. It is not yet approved or implemented. Persistent indexing, distributed caches and account switching are outside this proposal.

## Proposed correctness contract

- Scope is the management instance's configured NATS account. Reuse current resource-read authorization and local-demo policy; no subscription plaintext, client identity, credentials or arbitrary metadata is added.
- Canonical identity is `(stream, consumer name)`, not durable name alone. Identical names on different Streams remain separate. Queue association must come from a consistent declaration and supported ownership checks, never an `RJS_` naming guess.
- Preserve expected-but-missing declared Consumers and observed external Consumers. Distinguish missing, present, mismatched and unknown observations. A durable Consumer with no connected client is not automatically unhealthy. Pending/ACK-pending values are counts, not root-cause conclusions.
- Collection has cancellation, one in-flight refresh per instance, bounded worker concurrency, row/byte/time budgets and explicit limit errors. Select exact budgets through a measured prototype supporting the agreed scale, not by silently truncating at a smaller convenient dataset.
- Build a new generation off to the side; publish only after every required source succeeds and consistency checks pass. Failed/partial/over-limit refreshes never replace a completed generation with partial rows or zero totals. An older generation may remain visible only as stale, with original timestamps and failure metadata. With no complete generation, return unavailable/collecting, not an empty successful list.
- Record collection start/end and generation ID. A multi-source collection is not a simultaneous snapshot. Declaration/identity changes detected during collection invalidate publication; changes after observation remain possible.
- Filter the whole completed generation, then deterministic sort with an identity tie-breaker, then page. Default 50 and maximum 200 items per response. Return filtered total and generation ID. A request pinned to an unavailable/replaced generation must require an explicit pagination reset, never silently mix generations. Do not retain unbounded historical generations.
- Keep original integer precision through Go JSON and the browser's lossless decoder. Store only fields necessary for the list and joins; detailed configuration remains behind exact existing detail reads.
- Routine UI refresh reads the index cheaply; it must not trigger full account scans every ten seconds. Separate the explicit collection-refresh action, coalesce duplicate requests and report its progress/failure. Final endpoint/method/cooldown choices follow the owner decision and API review.

## Implementation and acceptance sequence

1. Resolve collection model, define typed response/query contracts and explicit observation-state semantics. Record exact budgets after prototype measurement.
2. Implement collection/join/query independently of HTTP; test duplicate identities, external resources, missing declarations/Consumers, ownership disagreement, cancellation, time/row/byte limits, partial failures, stale retention and concurrent refresh requests.
3. Add authorized API handlers and OpenAPI. Reject unknown/repeated/malformed filters, page bounds and generation mismatches. Test direct unauthorized calls and ensure rejected requests do not initiate collection.
4. Add a real global list before filtering, stable URL filters, generation-aware pagination, explicit refresh and exact detail links. Keep stale/unavailable/empty distinct in both languages; do not present the existing Stream page as global search.
5. Verify 201+ cross-Stream matches, repeated names, missing expected members, source failure and generation replacement against real services. Verify keyboard/mobile behavior and safe large integers.
6. Measure 10,000 Queue / 100,000 Consumer collection memory, broker request cost and query latency on stated hardware. Do not claim the performance requirement until measured. Release/native qualification remains separate.
