---
name: rjs-incident-response
description: Diagnose and respond to Rabbit JetStream incidents - management health checks, JetStream availability triage, backlog and DLQ investigation, audit forensics, diagnostics bundles. Use when the management console or API reports errors, queues show backlog or missing streams, alerts fire, or an operation has an unknown outcome.
---

# Rabbit JetStream incident response

## First five checks (in order)

1. Management health: `curl -s http://127.0.0.1:8223/healthz` → `{"status":"ok"}`. `/readyz` for readiness with dependency state.
2. JetStream availability per node: `curl -s http://127.0.0.1:<8222|8223|8224>/jsz?consumers=false` — all nodes must report `"config"`.
3. Controller: `curl -s http://127.0.0.1:8223/metrics | grep rjs_controller_leader` (exactly one 1) and `rjs_controller_blocked_queues` should be 0.
4. NATS node states: `rjs_nats_nodes{status=...}` — any `unavailable` is a finding.
5. Recent audit: `rjsctl audit list --url ... --token ...` — correlate intent/outcome pairs with the incident window.

## Classify the fault

- **Control-plane only** (management 5xx, console errors, but publish/consume fine): check management log category (no raw errors in logs by design), Prometheus `rjs_management_5xx` rate, then the specific dependency (Prometheus configured? local accounts file valid? tenants file valid?).
- **Data path** (publish/consume ack errors): check `nats stream info RJSQ_<name>` per stream, consumer pending/ack-pending/redelivered, node liveness. Follow docs/operations.md Incident Triage: freeze topology changes, one member at a time, never delete volumes to force start.
- **Unknown outcome after a write**: do NOT retry. Correlate audit intent/outcome by request ID (GET /api/v1/audit/requests/{id}), check resource existence, record evidence. See docs/operations-companion.md scenario C.

## Backlog triage (queue pending > threshold)

1. Locate: console Queue list sorted by stored messages, or `rjs_queue_messages{queue=...}` metric.
2. Diagnose: consumer offline (pending high, ack_pending 0) vs stuck consumer (both high) — check consumer info `nats consumer info RJSQ_<n> RJSQC_<n>`.
3. Act on the consumer side; the management plane has no backlog actions by design.
4. Verify: pending falls; metric-history trend returns to baseline.

## DLQ triage

1. Per-queue failures: `rjs_dlq_queue_failed_total{queue=...}` (attribution shipped in rc.3).
2. Inspect dead letters: `nats stream view RJSQ_dead-letters --last 10 --raw` (no payloads in tickets).
3. Replay only with ticket audit: `nats pub <original-subject> <payload>` — workqueue deletes on ack; replay publishes new messages, originals age out.
4. Failed moves that persist: check the target Queue declaration exists and the controller leader is active.

## Evidence to collect before closing

`rjsctl diagnostics collect --url ... --output bundle.zip` (metadata only, sanitized) + audit export + incident window in UTC + image digests. Never attach message payloads. Full procedure: docs/operations.md, runbooks in docs/operations-companion.md.
