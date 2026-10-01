# Management API

The embedded Admin UI is served at `/admin/` and consumes only the endpoints below. The console is no longer read-only: reviewed Queue create/update/delete, local account management, diagnostics jobs and audit reads are implemented behind authentication (see `webui-access.md` and the C-contract status in `webui-api-contracts.md`). `/` redirects to the console; health and API paths remain unchanged.

`GET /metrics` exposes Prometheus text metrics without authentication for in-cluster scraping. Do not expose it directly to untrusted networks.

The versioned API is intended for `rjsctl`, Admin UI, diagnostics, and monitoring integrations. Responses use JSON. Collection endpoints return `items`, `total`, `offset`, and `limit`; the default limit is 50 and the maximum is 200. Queue writes are disabled unless an operator token is configured.

The compatibility rules and machine-readable contract are defined in [Management API Versioning](api-versioning.md).

Reported NATS and monitoring URLs never include URL user information. `rjsctl diagnostics collect` composes the read-only endpoints into a redacted, checksummed support bundle; see [Diagnostic Bundles](diagnostics.md).

## Endpoints

| Method and path | Purpose |
|---|---|
| `GET /healthz` | Process liveness; does not imply JetStream availability |
| `GET /readyz` | JetStream account readiness |
| `GET /api/v1/openapi.yaml` | Embedded OpenAPI 3.1 contract for this v1 server build |
| `GET /api/v1/native-sdk-contract.json` | Cacheable native SDK resource, message, priority and delivery contract |
| `GET /metrics` | Prometheus service, JetStream, Queue, node, controller, DLQ, and HTTP metrics |
| `GET /api/v1/info` | Distribution and account usage summary |
| `GET /api/v1/cluster` | Connected server and JetStream account/API usage |
| `GET /api/v1/nodes` | Aggregated process, route, and JetStream node health |
| `GET /api/v1/streams` | Paginated Stream summaries |
| `GET /api/v1/streams/{stream}` | Stream configuration, state, leader, and replicas |
| `GET /api/v1/streams/{stream}/consumers` | Paginated Consumer delivery and backlog state |
| `GET /api/v1/queues` | Queue declarations persisted in JetStream KV |
| `GET /api/v1/queues/{queue}` | Queue declaration; returns its KV revision as `ETag` |
| `PUT /api/v1/queues/{queue}` | Authenticated, reconciled Queue apply |
| `DELETE /api/v1/queues/{queue}` | Authenticated Queue deletion with explicit confirmation |
| `GET /api/v1/controller` | Local controller instance, leadership, reconcile counts, cumulative DLQ processed/moved/failed counts, and last error |
| `GET /api/v1/audit` | Authenticated, newest-first Queue mutation audit events |

Example:

```text
GET /api/v1/streams?offset=0&limit=50
```

Errors have a stable envelope:

```json
{"error":{"code":"not_found","message":"resource not found: stream ORDERS"}}
```

`400` indicates invalid pagination, `404` a missing resource, and `503` unavailable JetStream management data. Node/process metrics come from explicitly configured NATS monitoring endpoints and are not inferred from account data.

`/api/v1/nodes` collects each configured monitoring endpoint concurrently. Its top-level status is `available`, `degraded`, or `unavailable`. Individual nodes preserve endpoint-specific errors so one failed member does not hide healthy members. Credentials embedded in monitoring URLs are never returned.

Write requests require `Authorization: Bearer <token>` and an optimistic concurrency precondition. Use `If-None-Match: *` for creation or `If-Match: "<KV revision>"` for updates and deletion. Missing conditions return `428`; stale revisions and active Queue locks return `409`. Delete additionally requires `X-RJS-Confirm-Queue` to exactly match the path name. Unsafe reconcile plans and non-empty deletion without `force=true` return `409` without performing the destructive operation. `rjsctl` reads the current ETag and supplies these headers automatically.

Operator tokens protect Queue writes and may read `GET /api/v1/audit`; auditor tokens may only read that endpoint. Authenticated, validated mutations are durably recorded before execution and fail closed if the intent cannot be persisted. See [Management Audit Trail](audit.md) for outcome semantics and [Management Credential Rotation](credential-rotation.md) for role and rotation procedures.
