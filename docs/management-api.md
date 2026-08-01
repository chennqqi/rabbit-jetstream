# Management API

The versioned API is read-only during M1. It is intended for `rjsctl`, Admin UI, diagnostics, and monitoring integrations. Responses use JSON. Collection endpoints return `items`, `total`, `offset`, and `limit`; the default limit is 50 and the maximum is 200.

## Endpoints

| Method and path | Purpose |
|---|---|
| `GET /healthz` | Process liveness; does not imply JetStream availability |
| `GET /readyz` | JetStream account readiness |
| `GET /api/v1/info` | Distribution and account usage summary |
| `GET /api/v1/cluster` | Connected server and JetStream account/API usage |
| `GET /api/v1/nodes` | Aggregated process, route, and JetStream node health |
| `GET /api/v1/streams` | Paginated Stream summaries |
| `GET /api/v1/streams/{stream}` | Stream configuration, state, leader, and replicas |
| `GET /api/v1/streams/{stream}/consumers` | Paginated Consumer delivery and backlog state |

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
