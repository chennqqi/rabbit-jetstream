# Message Migration, Shadow Verification, and Cutover

Topology conversion does not move messages. Production migration uses explicit phases: prepare, dual-publish, shadow-verify, cut over consumers, stop RabbitMQ publishing, drain, then retire. Keep RabbitMQ topology and rollback credentials until the final observation window passes.

## Evidence contract

Both shadow consumers must emit one NDJSON observation per received delivery without invoking business side effects:

```json
{"id":"order-0001","sha256":"<64 lowercase hex>","size":128}
```

`id` is the immutable application message ID shared by RabbitMQ `message_id` and JetStream `Nats-Msg-Id`; it must identify one logical publish. The adapters compute `sha256` and `size` over the exact broker payload bytes, so dual publishers must send identical encodings. Do not include payloads or secrets in evidence files. A redelivery produces a second identical observation and is reported as a duplicate; conflicting observations for one ID invalidate the evidence.

Compare each bounded window before cutover:

```sh
rjsctl migrate reconcile \
  --source rabbitmq.ndjson --target jetstream.ndjson \
  --output reconciliation-20260802T1200Z.json
```

The default gate requires at least one unique source message and allows zero missing, unexpected, mismatched, or duplicate observations. Raise `--min-source` to the declared window volume floor; reviewed tolerances are available through `--max-missing`, `--max-unexpected`, `--max-mismatch`, and `--max-duplicates`. The report is created with mode `0600`, never overwrites existing evidence, caps detailed IDs at 1,000 by default, and still retains complete counters.

## Dual-publish safety

Publishing independently to two brokers cannot be atomic. Use a transactional application outbox with one stable message ID and independently recorded RabbitMQ and JetStream confirms. Retry JetStream with the same `Nats-Msg-Id`; never generate a new ID on retry. Do not acknowledge or delete an outbox row until both confirms are durable. Alert on oldest incomplete row and preserve the outbox through rollback.

For controlled migration batches, the operator provides a recoverable dual-write adapter. Input is NDJSON; `payload_base64` contains the exact bytes sent to both brokers:

```json
{"id":"order-0001","payload_base64":"eyJvcmRlciI6MX0=","content_type":"application/json","headers":{"tenant":"one"}}
```

```sh
RJS_RABBITMQ_URL='amqps://...' RJS_NATS_URL='nats://...' \
rjsctl migrate dual-write --input outbox.ndjson --journal confirms.ndjson \
  --exchange orders --routing-key created --subject rjs.q.orders.ingress
```

RabbitMQ publishes are persistent, mandatory, and require publisher confirms; unroutable returns fail the batch. JetStream publishes require PubAck and force `Nats-Msg-Id` to the input ID. Each broker confirmation is appended and synchronized to the journal separately. Re-running with the same journal skips confirmed sides. The command refuses duplicate input IDs, concurrent journal writers, corrupt journal events, or any payload/content-type/header drift for an existing ID.

This journal narrows but cannot eliminate the crash interval between a broker confirm and its journal `fsync`. JetStream retries are deduplicated by `Nats-Msg-Id`; RabbitMQ can redeliver that interval, so downstream handlers must remain idempotent by message ID. A stale `.lock` must only be removed after proving the recorded PID is gone and archiving the journal. Never edit or truncate a journal without preserving the original as incident evidence.

Create a RabbitMQ shadow queue bound to the same exchange/routing keys as the source queue. For JetStream, create a dedicated **Limits-retention** shadow Stream capturing the same ingress subject before the window starts. Never attach a shadow consumer to the managed Queue Stream: it uses WorkQueue retention, where an overlapping consumer is unsafe and may be rejected. The CLI enforces this restriction.

Start both captures before publishing:

```sh
RJS_RABBITMQ_URL='amqps://...' rjsctl migrate capture rabbitmq \
  --queue orders.shadow --count 10000 --timeout 15m --output rabbitmq.ndjson
RJS_NATS_URL='nats://...' rjsctl migrate capture jetstream \
  --stream ORDERS_SHADOW --filter rjs.q.orders.ingress \
  --count 10000 --timeout 15m --output jetstream.ndjson
```

Supply connection secrets through environment variables or credentials files, not command arguments. Each adapter writes and synchronizes evidence before ACK; a crash can create a visible duplicate but cannot silently acknowledge an unrecorded sample. Run at least one peak-load and one failure/recovery window. Zero-count windows do not qualify.

## Cutover and rollback

Cut over one queue cohort at a time only after consecutive reconciliation windows pass, lag is within the declared SLO, DLQ/redelivery tests pass, and rollback capacity is available. Stop new RabbitMQ publishes only after JetStream consumers are healthy; then drain confirmed RabbitMQ backlog.

Rollback immediately on reconciliation failure, unbounded lag, confirm failure, or business invariant breach: pause JetStream consumers, restore RabbitMQ publishing/consumers, keep the same message IDs, and reconcile the rollback window. Never replay both brokers into side-effecting consumers simultaneously. Preserve reports, outbox state, broker metrics, deployment revisions, and audit events as release evidence.

### Automated orchestration

Use a reviewed JSON plan to execute one queue cohort. Every command is an argv array executed directly (never through a shell), must be safe to repeat, and needs an inverse action. At least two passing, time-ordered reconciliation reports are required and their SHA-256 digests bind the exact evidence to the plan:

```json
{
  "schema": "rabbit-jetstream.io/cutover-plan/v1alpha1",
  "migration_id": "orders-2026-08-02",
  "reconciliation_reports": [
    {"path": "window-1.json", "sha256": "<64 hex characters>"},
    {"path": "window-2.json", "sha256": "<64 hex characters>"}
  ],
  "steps": [{
    "name": "route-consumers",
    "action": ["kubectl", "apply", "-f", "consumers-jetstream.yaml"],
    "rollback": ["kubectl", "apply", "-f", "consumers-rabbitmq.yaml"],
    "timeout": "2m",
    "idempotent": true
  }]
}
```

```bash
rjsctl migrate cutover apply --plan cutover.json --journal cutover.ndjson \
  --confirm orders-2026-08-02 --max-evidence-age 30m
rjsctl migrate cutover status --plan cutover.json --journal cutover.ndjson
rjsctl migrate cutover rollback --plan cutover.json --journal cutover.ndjson \
  --confirm orders-2026-08-02
```

The append-only journal is synced before and after each action and protected by a single-writer lock. A restart skips completed actions and repeats an interrupted action, which is why `idempotent=true` is mandatory. Rollback runs completed actions in reverse order and is itself resumable. Do not edit a plan after execution begins: its canonical digest is permanently bound to the journal.
