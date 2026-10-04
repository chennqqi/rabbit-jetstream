---
name: rjsctl-operations
description: Operate the Rabbit JetStream management plane with the rjsctl CLI and management API - queue lifecycle (plan/diff/reconcile/apply/delete), audit queries, backups, diagnostics, and status checks. Use when an agent needs to inspect, create, change, or delete Queues, query audit trails, take backups, or collect diagnostics on a rabbit-jetstream deployment.
---

# Rabbit JetStream management operations (rjsctl)

## Environment

- CLI: `bin/rjsctl` (build: `go build -o bin/rjsctl ./tools/rjsctl`).
- Server URL and token: `--url http://127.0.0.1:8223 --token <operator-token>`, or `RJS_ADMIN_TOKEN` env for the server side.
- Terms: a **Queue** is a declarative document (`spec.replicas`, `subjects`, `storage`, `retention`, `maxPriority`, `deadLetter`); the controller reconciles it into a JetStream Stream `RJSQ_<name>` + consumer(s) `RJSQC_<name>`. See docs/glossary.md.

## Read operations (safe, no token needed for demo-mode loopback)

```bash
rjsctl status --url $URL                          # server identity
rjsctl queue list --url $URL                      # declared queues
rjsctl queue validate FILE.yaml                   # local schema check
rjsctl queue plan FILE.yaml                       # offline JetStream plan
rjsctl queue diff CURRENT.yaml DESIRED.yaml       # two documents side by side
rjsctl queue reconcile --url $URL FILE.yaml       # read-only: create/update/noop/recreate/reject per queue
curl -s -H "Authorization: Bearer $T" $URL/api/v1/queues            # declarations
curl -s -H "Authorization: Bearer $T" $URL/api/v1/queues/<name>     # one queue + observation
```

## Write operations — follow the safety sequence

Never apply without running the read-only sequence first. The control plane is audit-logged; agents are audited exactly like humans.

1. `rjsctl queue plan FILE.yaml` — inspect the generated plan.
2. `rjsctl queue diff CURRENT.yaml DESIRED.yaml` — confirm the delta is what you intend.
3. `rjsctl queue reconcile --url $URL FILE.yaml` — server-side read-only check (create/update/noop/recreate/reject).
4. Only then: `rjsctl queue apply --url $URL --token $T FILE.yaml` (conditional write; `blocked` results return 409 — do not force).
5. Verify: `rjsctl queue list` and `curl .../api/v1/queues/<name>` observation.

## Delete — extra care

```bash
rjsctl queue delete --url $URL --token $T --confirm <exact-name>            # refuses non-empty
rjsctl queue delete --url $URL --token $T --confirm <exact-name> --force    # required if messages exist
```

- `--force` is destructive and still cannot bypass ownership protection.
- If the outcome is **unknown** (error after dispatch, 503), DO NOT retry: check `rjsctl audit list` for the request outcome and `nats stream info RJSQ_<name>` for existence; record evidence. See docs/operations-companion.md scenario C.
- Check delete preflight first via API: `GET /api/v1/queues/<name>/delete-preview` (shows `dead_letter_dependents` — queues whose DLQ targets this one).

## Audit, diagnostics, backup

```bash
rjsctl audit list --url $URL --token $T --offset 0 --limit 50     # intent/outcome events
rjsctl diagnostics collect --url $URL --output bundle.zip         # sanitized metadata ZIP (no payloads)
rjsctl backup create --output <dir> && rjsctl backup verify --input <dir>
rjsctl backup restore --input <dir> --confirm RESTORE --replicas 3   # empty account, writers+controller stopped
```

## Hard rules

- The management plane never touches message payloads; peek/purge/replay belong to `nats` CLI + ticket audit (docs/operations-companion.md).
- Queue replica changes and R-level migrations: docs/scaling-topology.md (recreate is destructive; R changes need backup/restore).
- Update declarations only via preview → conditional apply (`If-None-Match: *` create, `If-Match: <kvRevision>` update); never hand-edit JetStream.
