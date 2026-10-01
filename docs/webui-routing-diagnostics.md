# WebUI routing diagnostics

[English](webui-routing-diagnostics.md) | [简体中文](webui-routing-diagnostics.zh-CN.md)

## Status and boundary

WEB-020 is in progress. The pure backend calculation `topology.ProbeRouting`, saved-declaration read API and candidate Queue Routing-tab probe are implemented. This is not a release-readiness claim; embedded release assets have not been promoted.

The calculation validates one Queue declaration through `BuildPlan` and reuses `QueuePublishSubject` for exchange-target translation. It neither publishes messages nor reads a broker, changes resources, resolves other Queues, or proves delivery. It copies the slices sorted by the planner so a diagnostic read cannot reorder its caller's draft.

## Inputs and evidence

- Supply either a literal NATS Subject or an exchange name/type/routing key. Mixed inputs, wildcard probe subjects, malformed declarations and unsupported translations fail; they must not become successful empty matches.
- A valid unmatched subject returns an empty match array, distinct from validation failure.
- Results include Queue, declaration revision, generated Stream name, resolved subject, all generated Stream subjects and their matches, plus each binding's keys/generated subjects and matches. These are declaration-level facts, not live Stream observations.
- `*` matches exactly one token. NATS `>` matches one or more tokens. Supported RabbitMQ trailing `#` is expanded by the existing planner into stem and stem-plus-`>` subjects, preserving zero-token matching. Interior `#` remains unsupported.
- Ordinary Queues include their generated ingress subject. Priority Queues capture priority subjects instead: a binding match can coexist with **no Stream match**. Literal priority probes are supported; exchange-to-priority resolution is explicitly rejected because the probe does not implement SDK priority selection.
- No payload, credentials, subscriptions or client identity metadata is collected. This work does not resolve the separate subscription visibility approval.

## Remaining integration

1. Implemented: `GET /api/v1/queues/{queue}/routing-probe?subject=orders.created`, or `?exchange=events&type=topic&routingKey=orders`. Uses existing resource-read authorization, including its explicit local-demo exception. Rejects repeated/unknown parameters, mixed modes and invalid encodings; raw query ≤4096 bytes, each decoded value ≤1024 bytes, Queue name ≤256 bytes. Reads one saved declaration under a three-second deadline, verifies identity/revision and faithful reconstruction, and performs no mutations. Missing declaration returns 404; unrepresentable/inconsistent evidence returns 409 `routing_declaration_unavailable`; unsupported probe semantics return 400 `invalid_query`. Response is limited to 1 MiB with 503 `routing_probe_limit` instead of truncation. Successful responses carry `no-store` and the exact quoted KV revision ETag. Unit/API tests cover these boundaries, cancellation and GET/HEAD authorization. OpenAPI describes the response projection.
2. Implemented in the candidate: bilingual explicit probe action, literal/exchange modes, separate generated Stream and per-binding evidence tables, declaration revision and local completion time. The original declaration mapping remains visible before a probe. There are no publish/replay actions or automatic probes. Input edits, mode switches, cancellation and unmount clear evidence; late responses cannot restore it. A changed content revision or KV ETag requires a page refresh rather than mixing versions. Errors clear results and distinguish denied/disabled/missing/changed/invalid/limit/unavailable from a valid nonmatch. Frontend tests cover these states and response projection; no JavaScript routing translator is added.
3. Chromium and Firefox now cover real direct/topic/fanout binding results, live declaration changes, cancellation/retry, Chinese mobile results, keyboard access to the last results column and malformed/unknown error handling; both passed 107 full-suite checks on the current candidate. Mobile tables use minimum widths and internal horizontal scrolling, verified by assertions and screenshot inspection. Remaining: review full accessibility across all states and the original declaration map (scoped contrast scans alone do not establish this). Broader scale and release qualification remain independent gates. See the development ledger for exact candidate evidence.
