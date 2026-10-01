# Exact Consumer detail implementation

[English](webui-consumer-detail.md) | [简体中文](webui-consumer-detail.zh-CN.md)

## Current candidate behavior

The candidate uses real authenticated APIs for Queue/Stream Consumer collections and exact Consumer details. Resource reads require server authorization by default; the explicit local-demo exception is described in [access policy](webui-access.md). Older notes below record implementation milestones, not the current access policy or a release-completion claim. Consult the [execution record](webui-development.md) for current browser evidence. Embedded release assets remain separate from the candidate.

Standalone Consumer details now share the manual/10/30/60-second refresh preference. One exact Stream/name GET is in flight; reads start after the previous read completes, back off to at most 60 seconds on failure and pause when hidden. Route/identity changes stop the old timer, abort its request and fence late results. Network/unavailable failures retain the previous exact observation and original read time with historical/stale labels. Missing Consumer, access denial, explicit read disablement or invalid/mismatched identity clears the observation and time. Recovery replaces it with fresh data, including mode/counter changes. It does not infer that a newly recreated Consumer with the same name is the same lifetime.

The 30-second threshold is freshness policy, not health. Raw observations are historical too while an old value is retained. No message fetch, ACK, mutation, enumeration or preview occurs. Queue Summary now reads the exact declared primary Consumer and Stream in one scheduled batch, retaining independently labeled history and times without advancing the declaration or editor ETag; see [Summary refresh](webui-refresh.md#queue-summary-refresh). Queue/Stream Consumer collections have separate query-scoped refresh with their own observation times; see [refresh contract](webui-refresh.md#consumer-collection-refresh).

## Historical implementation milestones

Date: 2026-09-09. Status: source implemented, unreleased. Partial delivery of [C-06 / API-11](webui-api-contracts.md).

`GET /api/v1/streams/{stream}/consumers/{consumer}` returns the existing Consumer observation object, not a paginated envelope. For example:

```sh
curl --fail http://127.0.0.1:8080/api/v1/streams/RJSQ_orders_events/consumers/RJSQC_orders_events
```

Use the actual management address/port. The running frozen candidate does not include this route. Example response fields: `stream`, `name`, `mode`, `pending`, `ack_pending`, `filter_subjects`, and optional `cluster`; the complete schema is in [OpenAPI](../api/openapi.yaml).

- Exact Stream/name identity, no list enumeration or dependence on list pages. Pagination query parameters have no effect on this detail route.
- Three-second request context. One pull lookup; only the SDK's `ErrNotPullConsumer` permits one push lookup. No message fetch, subscription, acknowledgement, mutation or audit write.
- Pull and push observations use the same mapper as lists. A concurrent mode change during the fallback can produce 503; there is no unbounded retry.
- 200 returns observation; missing Stream/Consumer returns 404 `not_found`; other backend failures return 503 `jetstream_unavailable`. Failure never becomes an empty or zero-valued observation. Responses use `Cache-Control: no-store`.
- Counters remain JSON integers. Clients must decode large integers losslessly before JavaScript rounding. No Queue-wide aggregate, freshness timestamp or health verdict is implied.
- Access matches existing unauthenticated resource reads. This is **not approval for non-loopback exposure**; D-05 policy remains unresolved. No security setting, deployment, credential or frozen artifact was changed.

Verification performed locally:

```sh
go test -count=1 ./...
go vet ./...
```

Both passed. New backend cases cover pull/push, missing resources, cancellation/timeout and disappearance/type changes during fallback; the test SDK makes enumeration/mutations fail immediately. HTTP cases verify scoped identity, paging independence, error envelopes, deadlines, no-store and no mutation/audit calls. Schema field checks detect model/property and required/optional drift; these checks are not a complete JSON Schema validator. Existing route/reference tests also pass.

No live NATS qualification or browser integration was performed. The prototype still uses fixtures. The Queue-related collection was subsequently implemented as recorded below; global list queries and real-API UI integration remain pending. WP-02 is not complete.

## Queue-related collection (v0.3 source update)

`GET /api/v1/queues/{queue}/consumers` now merges the declared primary and additional priority Consumers with actual Consumers on the declared Stream, including external extras. It does not enumerate the account or follow DLQ dependencies. Example (against a locally rebuilt management service):

```sh
curl --fail 'http://127.0.0.1:8080/api/v1/queues/orders_events/consumers?q=orders&mode=pull&order=asc&offset=0&limit=50'
```

The page contains `queue`, `stream`, `declaration_revision`, `stream_status`, `stream_ownership`, `items`, `total`, `offset`, `limit`. Each item has exact `stream`/`name`, nullable `expected` (ConsumerPlan), nullable `observed` (Consumer), `status` (`present|missing`), and `ownership`. Missing expected Consumers keep their declaration configuration but have **null observation, not fabricated zero metrics**. Extra Consumers have null expected configuration. Explicit priority zero remains a priority Consumer; priority 0–7 yields eight expected identities exactly once.

Ownership values `matching|different|unmarked|unknown` report the `rabbit-jetstream.io/queue` metadata evidence. Missing observations are unknown. Check Stream and Consumer evidence separately: neither a name match nor matching Consumer metadata overrides a Stream ownership mismatch. Ownership is not authorization, configuration convergence or health.

Query rules:

- `q`: trimmed, case-insensitive literal substring of name or effective filter Subject; at most 256 UTF-8 bytes after normalization. Use observed multi-subject values, falling back to the observed single Subject; only missing observations use expected Subjects.
- `mode`: omitted/empty for all, or `pull|push`; uses observed mode when present, otherwise expected mode.
- Name ordering: `order=asc|desc`, default ascending. No alternate sort field yet. Filtering and sorting happen before paging; `total` is the complete filtered count, not the page length.
- `offset=0`, `limit=50` defaults; limit 1–200. Offset beyond total clamps to total. Empty items is `[]`. Unsupported, repeated, malformed or invalid query parameters return 400 before backend reads. Existing list routes are unchanged.

Read safety and errors:

- Five-second request context; one Stream enumeration capped at 2,000 observed records (the next record detects excess), plus at most 256 expected identities. Context is canceled on all exits. Cost is bounded enumeration, **not an index or a production load qualification**.
- Missing Queue declaration: 404. Missing Stream: 200 with missing Stream status and missing expected Consumers. Other read failures, incomplete/duplicate/inconsistent enumeration, corrupt declaration or exceeded cap: 503, without a partial-success list.
- Declaration KV revision is re-read after observation; changed/deleted declaration yields 409. The quoted `declaration_revision` is an opaque declaration version, not an HTTP ETag for Consumer metrics. This is not an atomic broker snapshot; resource churn between reads remains possible.
- Responses remain no-store and use the existing unauthenticated resource-read policy. No message delivery, mutation, audit write, security-policy change, deployment or remote operation is introduced.

New tests cover priority membership, missing/external resources, metadata mismatch, zero priority, corrupt declarations, read failures/cancellation, revision changes, cap boundaries, 201-resource search, observed/expected Subject filtering, sorting, pagination and invalid queries. ConsumerPlan fields are checked against OpenAPI required/property declarations. `go test -count=1 ./...` and `go vet ./...` passed locally. Live NATS/load qualification and real WebUI integration remain outstanding; neither the frozen candidate nor the fixture prototype was replaced.
