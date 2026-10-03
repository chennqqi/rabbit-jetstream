# Operations Companion Checklist

[English](operations-companion.md) | [简体中文](operations-companion.zh-CN.md)

Status: delivered with improvement-plan item A3 · Date: 2026-10-01
Applies to: the four broken links identified in the [operations review](ops-review.md) — alert-to-human delivery, message-level troubleshooting sidestep, backup scheduling, and on-call workflows.

The console's boundary is deliberate: the management plane governs queue declarations and read-only observation, and never enters the message data path. The following components and workflows are therefore **deploy-time dependencies**; this document gives a directly usable baseline that teams scale to their needs.

---

## 1. Alert-to-human (mandatory, otherwise alerts are just page colors)

Chain: management `/metrics` → Prometheus (scrapes and evaluates the six shipped rules in `deploy/observability/alerts.yml`) → **Alertmanager (notification)** → on-call endpoint.

Shipped:
- `deploy/observability/alertmanager.yml`: baseline routing (critical groups after 10s, repeats every 4h); the default webhook receiver **must be replaced** with the team's real endpoint (Slack/PagerDuty/email, etc.);
- The `observability` profile in `deploy/compose/standalone.yml` now includes an `alertmanager` service (9093), and `prometheus.yml` wires the alertmanager target.

Enable:

```bash
docker compose -f deploy/compose/standalone.yml --profile observability up -d
# Verify: rules visible at http://localhost:9090/alerts; alerts received at http://localhost:9093
```

Verify delivery: temporarily lower the `RabbitJetStreamQueueBacklogHigh` threshold or push a `_testOnly` alert through the Alertmanager UI and confirm the on-call endpoint receives it. **The config has not been deeply validated with amtool yet** (the review environment cannot pull images); run before deploying:

```bash
docker run --rm -v $PWD/deploy/observability/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro prom/alertmanager:v0.28.1 amtool check-config /etc/alertmanager/alertmanager.yml
```

Tuning note: `RabbitJetStreamQueueBacklogHigh` defaults to 100k messages for 15m as a safety net; adjust per queue throughput (add per-queue rules for high-throughput queues).

## 2. Message-level troubleshooting sidestep (the management plane deliberately does not do this — an alternative path is mandatory)

The console **cannot see or touch** individual messages. All such operations go through the `nats` CLI ([nats-box container](https://github.com/nats-io/nats-box) or a local install) against JetStream:

```bash
# Enter a container with the nats CLI (adjust the image for your registry mirror)
docker run --rm -it --network <compose-network> natsio/nats-box:latest
nats context save rjs --server nats://nats:4222

# Scenario: browse the latest messages of a queue's Stream (read-only)
nats stream info RJSQ_orders-primary
nats stream view RJSQ_orders-primary --last 5

# Scenario: confirm a specific message exists
nats stream get RJSQ_orders-primary --last --raw --subject "orders.created"
```

Dead-letter replay sample (**confirm semantics with the owning team first**; replaying publishes a new message and does not delete the dead-letter original):

```bash
# 1) Inspect pending messages in the dead-letter Stream
nats stream info RJSQ_dead-letters
nats stream view RJSQ_dead-letters --last 10 --json
# 2) Republish the selected message to its original subject (raw payload from the previous step)
nats pub orders.created '<original-payload>'
# 3) Verify: the source queue's pending grows; the dead-letter Stream ages out per retention
```

Convention: every replay is recorded in a ticket (subject, stream sequence, operator, time). The `nats` CLI operates the data plane directly and bypasses management-plane audit — the ticket is the audit record.

## 3. Backup scheduling (CLI capability, scheduling is on you)

`rjsctl backup` is a single-host toolchain; nothing schedules it for you. Baseline cron (daily 02:30 backup + verify, 7-day retention):

```cron
30 2 * * *  /opt/rjs/bin/rjsctl backup create --output /var/backups/rjs/$(date +\%F) --server http://127.0.0.1:8223 && /opt/rjs/bin/rjsctl backup verify --input /var/backups/rjs/$(date +\%F) >> /var/log/rjs-backup.log 2>&1
0 4 * * *   find /var/backups/rjs -maxdepth 1 -mtime +7 -exec rm -rf {} \;
```

Constraints (from the tool boundary): `backup restore` requires an **empty account with writers and the controller stopped**, and has no cross-Stream atomicity — restore is a disaster-recovery procedure, not a routine; rehearse quarterly and keep records.

## 4. On-call runbook (three scenarios)

### Scenario A: queue backlog (alert `RabbitJetStreamQueueBacklogHigh`, or rising pending in the console)
1. **Locate**: console → Queue list sorted by messages; Queue detail summary shows "pending (primary consumer)".
2. **Diagnose**: Consumer page — high pending with pending-ack 0 usually means the consumer is offline; both high usually means the consumer is stuck processing.
3. **Act**: fix on the consumer side (restart/scale consumers); **the console has no handling action — that is the design boundary**.
4. **Verify**: console pending falls; the metric-history page shows the `queue messages` 15m/1h/24h trend.

### Scenario B: dead letters (alert `RabbitJetStreamDeadLetterFailures`, or DLQ diagnostics anomaly)
1. **Locate the queue**: console DLQ diagnostics (Queue detail → Configuration → DLQ diagnostics) shows the six evidence classes. Note: transfer counters are process-global — until item A12 lands **they cannot attribute a queue**; check suspects one by one with `nats stream info RJSQ_<name>`.
2. **Assess**: business impact (lose vs replay); inspect content with `nats stream view RJSQ_dead-letters` (mind data privacy).
3. **Act**: replay per §2 or let the owning team abandon; the controller keeps auto-moving new dead letters (at-least-once).
4. **Verify**: `rjs_dlq_failed_total` stops growing (do not rely on Alertmanager recovery notices — recovered state lives only in process memory).

### Scenario C: uncertain outcome after deleting a Queue (delete request returned 503/unknown)
1. **Do not blindly retry**: the delete flow deliberately locks automatic retry on unknown outcomes (to avoid misdeleting an already-deleted resource).
2. **Evidence**: console → Queue detail → delete page downloads the deletion-evidence JSON (contains the request ID); console → Audit, query intent/outcome pairs by request ID and time window.
3. **Determine**: `nats stream info RJSQ_<name>` — Stream gone = deletion took effect; still present with 0 messages = not executed, re-run the delete preflight after confirming.
4. **Record**: archive the evidence JSON and the conclusion in a ticket; deletion evidence contains configuration and audit data — store it as sensitive.

---

## 5. Console vs sidestep quick reference

| Task | Console | Sidestep |
|---|---|---|
| Queue/consumer/node status | ✔ read-only observation | — |
| Queue create/update/delete | ✔ preview-confirm | `rjsctl queue apply/delete` |
| View one message | ✖ | `nats stream view/get` |
| Replay dead letters | ✖ | `nats pub` (ticket audit) |
| Pause consumer / purge queue | ✖ | consumer app / `nats stream purge` (dangerous, two-person rule) |
| Alert notifications | ✖ read-only projection | Alertmanager |
| Backup/restore | ✖ | `rjsctl backup` + cron |
