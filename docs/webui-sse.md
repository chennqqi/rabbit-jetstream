# Authenticated WebUI Event Stream

[English](webui-sse.md) | [简体中文](webui-sse.zh-CN.md)

`GET /api/v1/events` is a read-only, tenant-scoped SSE invalidation stream. The WebUI uses `fetch`, so its page-memory Bearer credential and `X-RJS-Tenant` header follow normal API policy. Credentials never enter the URL, persistent browser storage, cookies, or native `EventSource` state.

Events request a bounded authoritative refresh; they are not mutation outcomes or exact notifications. Persisted audit intent/outcome emits `audit`. A single process-wide Prometheus watcher emits `alerts` only after its fixed rule projection changes; its first successful observation establishes a baseline. Alert collection remains a bounded 15-second poll and is not exact delivery.

Each tenant has an independent monotonic ID and a 256-event instance-local replay window. A fresh connection starts at the current head. Reconnect supplies the last fully processed ID; unavailable replay returns 409, after which the client refreshes both resources and reconnects without an ID. Backoff grows from one to 30 seconds, and the server sends a comment heartbeat every 15 seconds.

Budgets are two connections per actor, 32 per tenant, and 256 per instance. Admission overflow returns 429 with `Retry-After`; a subscriber that fills its 16-event buffer is closed. Management remains outside the message data path.

## Qualification evidence

On 2026-09-13, Go tests passed authorization, invalid inputs, tenant isolation, fresh-head/replay/expiry semantics, connection budgets, slow-client eviction, shutdown, reverse-proxy flushing, and HTTP/2. The same focused suite passed the Linux race detector in a disposable Docker Desktop Go container. Admin UI tests cover fragmented parsing, the 64 KiB bound, invalid data, memory-only headers, replay, reconnect, and backoff.

All Go tests and `go vet ./...` passed. All 404 Admin UI tests completed with 403 passes, one Windows symlink-capability skip, and zero failures. Production build, promotion, and embedded-dist verification passed. A real isolated Docker Desktop mutation produced a tenant event and matching retained audit evidence. Headless Chromium loaded the embedded Audit page without URL credentials; its audit reads advanced from one to three for the real intent/outcome events.

Reverse proxies must disable response buffering and permit idle time beyond the heartbeat. Real deployment proxy configuration and capacity remain exact-candidate environment qualification; the implementation and reproducible local facility are complete.

