# Message Migration, Shadow Verification, and Cutover

Topology conversion does not move messages. Production migration uses explicit phases: prepare, dual-publish, shadow-verify, cut over consumers, stop RabbitMQ publishing, drain, then retire. Keep RabbitMQ topology and rollback credentials until the final observation window passes.

## Evidence contract

Both shadow consumers must emit one NDJSON observation per received delivery without invoking business side effects:

```json
{"id":"order-0001","sha256":"<64 lowercase hex>","size":128}
```

`id` is the immutable application message ID shared by RabbitMQ `message_id` and JetStream `Nats-Msg-Id`; it must identify one logical publish. `sha256` is computed over the canonical application payload before transport encoding, and `size` is that canonical byte length. Do not include payloads or secrets in evidence files. A redelivery produces a second identical observation and is reported as a duplicate; conflicting observations for one ID invalidate the evidence.

Compare each bounded window before cutover:

```sh
rjsctl migrate reconcile \
  --source rabbitmq.ndjson --target jetstream.ndjson \
  --output reconciliation-20260802T1200Z.json
```

The default gate requires at least one unique source message and allows zero missing, unexpected, mismatched, or duplicate observations. Raise `--min-source` to the declared window volume floor; reviewed tolerances are available through `--max-missing`, `--max-unexpected`, `--max-mismatch`, and `--max-duplicates`. The report is created with mode `0600`, never overwrites existing evidence, caps detailed IDs at 1,000 by default, and still retains complete counters.

## Dual-publish safety

Publishing independently to two brokers cannot be atomic. Use a transactional application outbox with one stable message ID and independently recorded RabbitMQ and JetStream confirms. Retry JetStream with the same `Nats-Msg-Id`; never generate a new ID on retry. Do not acknowledge or delete an outbox row until both confirms are durable. Alert on oldest incomplete row and preserve the outbox through rollback.

Shadow consumers use separate RabbitMQ/JetStream consumer identities, disable external side effects, and record evidence only after validating/decrypting the same canonical payload. Run at least one peak-load and one failure/recovery window. Zero-count windows do not qualify.

## Cutover and rollback

Cut over one queue cohort at a time only after consecutive reconciliation windows pass, lag is within the declared SLO, DLQ/redelivery tests pass, and rollback capacity is available. Stop new RabbitMQ publishes only after JetStream consumers are healthy; then drain confirmed RabbitMQ backlog.

Rollback immediately on reconciliation failure, unbounded lag, confirm failure, or business invariant breach: pause JetStream consumers, restore RabbitMQ publishing/consumers, keep the same message IDs, and reconcile the rollback window. Never replay both brokers into side-effecting consumers simultaneously. Preserve reports, outbox state, broker metrics, deployment revisions, and audit events as release evidence.
