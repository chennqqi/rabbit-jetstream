# Troubleshooting Guide

[English](troubleshooting.md) | [简体中文](troubleshooting.zh-CN.md)

Symptom → cause → action for the most common errors seen in the console, API, CLI, and SDK. Error codes in the API body are stable; match on those, not on message text.

## HTTP / API errors

| Symptom | Cause | Action |
|---|---|---|
| `401 unauthorized` (API) / back at login (console) | Session expired (token TTL 15 min) or token wrong | Sign in again; the console keeps drafts. Automation tokens: check `RJS_ADMIN_TOKEN(S)` / `RJS_AUDIT_TOKENS`. |
| `403 role-denied` | Your role lacks the permission (auditor cannot write) | Ask a platform admin for the operator role via Tenant access, or use an operator credential. |
| `404 read_api_disabled` | Server has no authentication configured at all | Set `RJS_ADMIN_TOKEN(S)` / `RJS_AUDIT_TOKEN(S)` / local accounts or OIDC, then restart. |
| `404 alerts_api_disabled` / console alerts page: "Prometheus alert backend is not configured" | `RJS_PROMETHEUS_URL` unset | Configure Prometheus (see [Operations Companion](operations-companion.md)); alert notifications additionally need Alertmanager. |
| `404 history_api_disabled` / "历史后端不可用" | Same as above | Same fix; history metrics share the Prometheus backend. |
| `428 precondition_required` | Missing `If-None-Match: *` (create) or `If-Match: <kvRevision>` (update) | Re-read the resource for the current KV revision, then retry with the header. The console does this automatically. |
| `409 conflict` / ETag mismatch | Declaration or KV revision changed under you | Re-read, re-apply your edit on the fresh revision (console: open the editor again). Never blindly re-submit. |
| `409 name_mismatch` | URL name and document name differ | Align them. |
| Delete returns `409` with `requires_force` | Stream contains messages | Confirm the impact, tick force, type the exact Queue name. Prefer draining consumers first. |
| Delete outcome **unknown** (error after dispatch, console locks retry) | Audit write failed after the backend call | Do not retry blindly. Check audit by request ID, check `nats stream info RJSQ_<name>`; follow [Operations Companion](operations-companion.md) scenario C. |
| `503 queue_unavailable` / backend errors | NATS or metadata quorum down | See [Incident Triage](operations.md): check nodes, controllers, do not restart multiple members at once. |

## Console symptoms

| Symptom | Cause | Action |
|---|---|---|
| Badge shows 缺失/Missing for a Queue you just created | Stream not yet reconciled by the controller | Wait one reconcile cycle (~5 s); refresh. If persistent, check controller metrics `rjs_controller_leader` / blocked queues. |
| Refresh indicator shows stale banner | Tab hidden, refresh failed, or data older than 30 s | Bring tab to front; if failure persists, check management healthz and NATS. |
| "SSO 登录不可用" only after clicking SSO | OIDC misconfigured or IdP rejected | Verify `RJS_OIDC_*` set, callback origin exact, IdP reachable. Local sign-in remains available. |
| Alerts page shows incomplete coverage | Prometheus missing some shipped rules | Check `deploy/observability/alerts.yml` loaded; promtool validate. |
| Session clears on F5 | By design: tokens live only in page memory | Sign in again; drafts are lost, evidence requires the session — record first. |

## SDK (Native Go)

| Symptom | Cause | Action |
|---|---|---|
| Publish fails with priority validation | Priority > Queue `maxPriority` | Publish ≤ max, or update the Queue declaration (preview first). |
| Publish returns but consumer sees nothing | Consuming the wrong subject/durable | Use the SDK `Consumer` (binds `rjs.q.<queue>.p.*` durables); don't hand-roll subjects. |
| Duplicate deliveries | At-least-once by design; consumer NAK/ack-wait expiry | Make handlers idempotent; keep stable publish IDs. |
| `Close` then messages redeliver | Buffered messages were NAKed on close | Expected; drain before Close or design for redelivery. |
| Starvation of low priorities | Prefetch budget too small | Tune `ConsumerConfig.Prefetch` (default splits 256 across levels). |

## CLI (rjsctl)

| Symptom | Cause | Action |
|---|---|---|
| `audit list` requires `--token` | Audit reads need an explicit credential | Pass an operator token. |
| `queue apply` returns 409 blocked | Plan is recreate-class or ownership mismatch | Inspect with `queue plan`/`diff`; use the backup path for destructive changes. |
| `backup restore` refuses | Destination Streams exist / no `--confirm RESTORE` | Restore into an empty account (stop writers + controller) and pass the confirm word. |
| `diagnostics collect` limited to 200 queues/streams | Bounded by design (4 MiB per response) | Fine for incidents; use the API for full inventories. |

## Deployment / local dev

| Symptom | Cause | Action |
|---|---|---|
| kind rootless: "Delegate=yes" error | user@.service lacks delegation | `systemctl set-property user@<uid>.service Delegate=yes`, restart user service. |
| kind: images ErrImageNeverPull | Images not loaded into nodes | `kind load image-archive` per image (multi-image archives collapse shared-base images). |
| Management: `connect to NATS: no servers available` | NATS down or wrong `RJS_NATS_URL` | Start NATS first; check routes/cluster name (`--cluster_name` required for JetStream clusters). |
| `subjects overlap with an existing stream` | Declared subjects collide with another Stream | Fix Queue subjects or remove the colliding stream; the control plane refuses overlaps. |
| npm test: 3 Windows skips | vite SSR drive-letter defect on Windows | Expected: skips are covered by Playwright e2e. |
| LF/CRLF warnings | Git autocrlf | Repo now pins LF via `.gitattributes`; set `core.autocrlf=false` locally. |

## Still stuck

`rjsctl diagnostics collect --output bundle.zip` and attach per [Escalation Evidence](operations.md) (no message payloads).
