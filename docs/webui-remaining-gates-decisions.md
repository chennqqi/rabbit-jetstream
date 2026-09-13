# WebUI Remaining-Gates Decisions

[简体中文](webui-remaining-gates-decisions.zh-CN.md)

Status: **accepted for planning** (2026-09-12). This document resolves the tracked decisions and facility gates left by [the WebUI improvement plan](webui-improvement-plan.md). It authorizes sequencing and infrastructure selection; it does not claim that the corresponding implementation is complete.

## Executive decision

| Item | Decision | First-release gate? | Start condition |
|---|---|---:|---|
| F-4 global Consumer ownership | JetStream and Queue declarations remain authoritative. The management service owns a tenant-scoped, bounded, rebuildable read projection, never another source of truth. | No; the current bounded generation implementation is acceptable for the first release. | Record and test multi-replica/staleness semantics before extending the index. |
| F-3 cursor pagination | Split from F-4. Keep the existing generation-bound offset contract for global Consumers. Do not add a generic cursor until each source has a bounded enumeration contract. | No; do not replace a measured working contract only to change pagination syntax. | Per-resource enumeration limits, ordering, snapshot invalidation, and compatibility behavior are specified. |
| O-5 DOM tests | Select Node's existing test runner plus jsdom and Testing Library; add `@testing-library/user-event` for interactions. Do this before broad UI refactors. | Facility gate for the remaining UI refactors, not for the already frozen release candidate. | Pin dependencies and provide one deterministic route/tenant-switch fixture. |
| F-8 OpenAPI codegen | Generate models/types only: pinned `oapi-codegen` for Go and pinned `openapi-typescript` for TypeScript. Keep the handwritten HTTP transport initially. | No. | OpenAPI response schemas are sufficiently concrete and generation is reproducible. |
| O-3 i18n catalogs | Decouple from F-8. Product copy is maintained in feature catalogs; OpenAPI may supply stable error identifiers, not translated prose. | No. | O-5 harness is present and catalog naming/ownership rules are documented. |
| S-4 density/shared tables | Extract only semantically equivalent read-only list primitives, then run a mandatory same-data visual QA matrix. | No, unless the pass exposes a P0/P1 regression in the release candidate. | O-5 harness exists and visual fixtures are frozen. |
| F-7 SSE | Defer until after the first release. Use authenticated `fetch` streaming of the SSE wire format, not native `EventSource`. | No. | Proxy behavior, connection budgets, replay rules, and streaming tests are available. |

The former order in the improvement plan is therefore superseded. In particular, O-3 is not generated from F-8, and O-5 moves ahead of the refactors it is meant to protect.

**Execution update (2026-09-13).** The sole owner subsequently authorized completing the full sequence locally and using Docker Desktop for infrastructure simulation. F-7 was therefore advanced after its proxy, replay, budget, backpressure and test facilities were implemented; this changes scheduling, not the safety contract. All implementation/simulation evidence in this decision is now closed. Exact-candidate bare-metal, Canary and external-signature gates remain outside this scope.

## F-4: ownership of the global Consumer projection

**Decision.** The authoritative facts remain in JetStream and the tenant's Queue declarations. The management service owns only a read-optimized projection. The projection is:

- isolated by tenant;
- replaced atomically only after a complete successful collection;
- bounded by explicit Stream, row, byte, worker, and collection-time limits;
- disposable and rebuildable; and
- allowed to report `collecting`, `stale`, or `unavailable`, but never to turn incomplete evidence into an empty successful result.

This matches the implementation already present in `management/internal/api/global_consumer_index.go`; the decision formalizes it instead of introducing a durable global database. Per-Queue health must not be fetched once per displayed row. Health is either part of a bounded collection/batch read or is obtained from a separate detail/diagnostic request.

Each management replica may hold its own projection. A generation identifier is therefore replica-local evidence, not a cluster-wide revision. Before multi-replica deployment, either sticky routing must cover a pagination session or a shared projection must be designed. A generation mismatch must remain explicit (409/reset), never silently continue on another generation.

## F-3: pagination and bounded enumeration

**Decision.** Do not create one generic cursor abstraction and claim that it fixes full enumeration. A cursor bounds client traversal and preserves ordering; it does not make `ListDeclarations`, `ListStreams`, or per-Stream `ListConsumers` incremental.

For the global Consumer endpoint, retain `generation + offset + limit`. It already provides snapshot identity, bounded page responses, and an explicit generation-change failure. An opaque cursor may later encode those fields, but that is a compatibility/UI convenience rather than a scalability fix.

Queue declarations and other live lists need source-specific contracts before cursor work begins. Each contract must define stable ordering, maximum scanned items/bytes/time, concurrent-change behavior, cursor expiry, tenant and filter binding, invalid-cursor errors, and whether totals are exact, estimated, or omitted. Existing offset parameters remain supported throughout v1; any cursor is additive until a separately approved API-version decision.

The 10k query benchmark measures in-memory filtering/sorting only. It is useful evidence but does not qualify backend enumeration or the 100k-Consumer collection path. F-3 is complete only after end-to-end scale and memory tests cover collection plus pagination.

## O-5: DOM test facility

**Decision.** Keep `node --test` as the runner and add pinned `jsdom`, `@testing-library/react`, and `@testing-library/user-event` development dependencies. Introducing Vitest as a second runner would add migration and configuration work without addressing a missing capability.

The first fixture must exercise `main.jsx` through real DOM events and cover route dispatch, browser history, dirty-state tenant switching (cancel and confirm), permission/session replacement, focus restoration, and unmount cleanup. Network and time are injected; no real NATS service is used. Playwright remains the authority for browser integration, accessibility, and screenshots; DOM tests are the fast regression layer, not a replacement.

O-5 is the first enabling task because F-8/O-3 and S-4 will touch the same high-risk composition layer.

## F-8: OpenAPI generation boundary

**Decision.** Use pinned `oapi-codegen` for Go models and pinned `openapi-typescript` for TypeScript types. Initially generate schemas only, check generated outputs into the repository, and add a CI drift check that regenerates and requires a clean diff. Generator versions and commands must be single-source and reproducible without network access after dependencies are installed.

Do not generate server routing or replace the WebUI HTTP transport in the first pass. The current transport owns authentication, tenant headers, error normalization, cancellation, downloads, and lossless handling of counters larger than JavaScript's safe integer range. A generated client may replace it only after equivalent tests exist.

Before generation is a gate, replace generic response objects in `api/openapi.yaml` with concrete schemas for the endpoints being migrated. Go handlers may adopt generated response models incrementally; internal JetStream/domain models must not be made dependent on transport-generated types.

## O-3: complete i18n cataloging

**Decision.** O-3 and F-8 are independent workstreams. OpenAPI describes protocol fields and stable error codes; it cannot own navigation labels, instructions, warnings, empty states, or accessibility text. Generating translated copy keys from the API contract would create false coupling and leave most UI copy uncovered.

Use feature-scoped catalogs with identical `en`/`zh` key structure. Keep the existing parity test and extend it with missing-key and unused-key checks. Inline bilingual ternaries for user-visible copy are removed incrementally by feature; dynamic values and non-translated protocol identifiers remain parameters. Completion requires both languages across normal, empty, loading, stale, error, confirmation, and accessibility states.

F-8 may supply typed error identifiers consumed by catalogs, but neither task waits for or marks the other complete.

## S-4: density, shared tables, and visual QA

**Decision.** Create shared table and pagination primitives only for semantically equivalent read-only collections. Complex evidence, comparison, editable, or nested tables remain specialized. Shared primitives must preserve native table semantics, captions, focusable horizontal overflow, stale/error evidence, exact/unknown totals, and link behavior.

Component extraction and density changes land together behind DOM coverage. Visual acceptance uses the same deterministic data at 1440, 1280, 1024, and 375 CSS pixels, in English and Chinese, light and dark themes. At minimum it covers normal, empty, error/stale, long-identifier, and maximum-column cases. Chromium is the screenshot baseline; Firefox receives a functional overflow/focus pass. Any screenshot difference must be reviewed rather than automatically accepted.

The objective is information density without clipped controls, hidden evidence, ambiguous pagination, or reduced keyboard access. Reuse percentage is not an acceptance metric.

## F-7: authenticated server-sent events

**Decision.** Implementation remains post-first-release. Retain polling until the facility gate is met. The browser's native `EventSource` cannot attach the required `Authorization` and `X-RJS-Tenant` headers, and moving the access token into a URL or persistent storage is prohibited. The client will therefore use an abortable `fetch` request and parse the standard SSE wire format.

The first stream is read-only and tenant scoped. It may carry audit-tail events and invalidation hints that trigger a bounded refresh; it must not claim exact alert delivery if the alert source is still polled. The contract must define event IDs, `Last-Event-ID` replay window, heartbeat interval, reconnect/backoff, slow-client buffer policy, maximum connections per actor/tenant/instance, server shutdown behavior, and 401/403/429/503 responses.

The facility gate requires verified reverse-proxy buffering/timeouts, HTTP/2 behavior, leak/backpressure tests, reconnect/replay tests, and capacity evidence. Long-lived connections remain outside the message data path.

## Approved sequence for one maintainer

1. Record F-4/F-3 API invariants and add missing generation/multi-replica contract tests; no pagination rewrite yet.
2. Deliver O-5's DOM harness and high-risk routing/tenant tests.
3. Harden the selected OpenAPI schemas, then introduce F-8 type generation and CI drift checks.
4. Migrate O-3 feature catalogs independently, using the DOM harness for behavioral protection.
5. Implement S-4 shared primitives and density changes, followed immediately by the full visual QA matrix.
6. Run end-to-end scale qualification; implement source-specific F-3 cursors only where the evidence shows they are necessary.
7. After the first release, qualify the streaming facility and then implement F-7.

With a single responsible maintainer, each step uses one decision record, one implementation change, and one acceptance evidence set. Separate committee-style approval stages are not required; a failed objective gate still blocks promotion of that step.

## Completion evidence

These tracked items may be closed only with the following evidence:

- F-3/F-4: API contract tests, generation invalidation tests, bounded collection tests, and end-to-end 10k Queue/100k Consumer time and memory results.
- O-5: deterministic DOM tests covering the listed route/session/tenant paths and passing existing Node and Playwright suites.
- F-8: pinned offline-reproducible generation, clean-diff CI, concrete schemas, and transport parity tests.
- O-3: catalog parity plus missing/unused key checks and bilingual state coverage.
- S-4: component tests, accessibility checks, and reviewed same-data screenshot evidence for the matrix.
- F-7: proxy qualification, connection-budget/load results, replay/reconnect/backpressure tests, and authorization/tenant-isolation tests.
