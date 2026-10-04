# SDK Quickstart: your first priority queue

[English](sdk-quickstart.md) | [简体中文](sdk-quickstart.zh-CN.md)

End-to-end tutorial for the Native Go SDK (`github.com/chennqqi/rabbit-jetstream-go`): declare a priority Queue through the control plane, publish with priorities, consume with idempotent handlers. The full API contract is [api-v0.1 in the SDK repo](../../outlink/rabbit-jetstream-go/docs/api-v0.1.md); the runnable example is `examples/priority`.

## 0. Prerequisites

- A running deployment (see [Deployment](deployment.md)): NATS at `nats://127.0.0.1:4222` and the management service at `127.0.0.1:8223` with an operator credential.
- Go 1.25+, and the SDK module available (`outlink/rabbit-jetstream-go`).

## 1. Declare the Queue (control plane, not the SDK)

Production topology is owned by the control plane. Prefer the console's **Create Queue** wizard or the CLI:

```bash
rjsctl queue apply --url http://127.0.0.1:8223 --token <operator-token> orders.yaml
```

with `orders.yaml`:

```yaml
apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata:
  name: orders
  labels: {team: commerce}
spec:
  subjects: ["orders.>"]
  replicas: 1            # 3 for production R3
  storage: file
  maxPriority: 7         # qualified range 0..7
```

Check the generated topology: Stream `RJSQ_orders`, primary consumer `RJSQC_orders`, plus one durable consumer per priority level (`rjs.q.orders.p.0..7`). Verify with the console Queue detail or `rjsctl queue list`.

> `EnsurePriorityQueue` in the SDK exists for standalone deployments and tests only — it validates but does not rewrite topology. Production declares through the control plane.

## 2. Publish with priorities

```go
client, err := rabbitjetstream.Connect(nats.DefaultURL)
if err != nil { log.Fatal(err) }
defer client.Close()

publisher, err := client.Publisher("orders", "queue-plan-revision", 7) // max priority bound
if err != nil { log.Fatal(err) }

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
ack, err := publisher.Publish(ctx, rabbitjetstream.PublishMessage{
    ID:       "order-42",              // stable ID for deduplication
    Priority: 9,                        // 0..max; validated against the Queue
    Data:     []byte(`{"order_id":42}`),
})
fmt.Printf("stream=%s sequence=%d duplicate=%t\n", ack.Stream, ack.Sequence, ack.Duplicate)
```

Rules the SDK enforces for you:

- **Every Publish needs a stable message ID** — at-least-once delivery means the ID is your deduplication key.
- **Publish returns only after JetStream PubAck** — no fire-and-forget; treat an error as "not yet accepted".
- **Priorities are validated** against the Queue's `maxPriority` before the wire.
- Reserved contract headers are overwritten by the SDK; do not set `Nats-Msg-Id`, `Rjs-*` yourself.

## 3. Consume with idempotent handlers

```go
consumer, err := client.Consumer("orders")
if err != nil { log.Fatal(err) }

for {
    msg, err := consumer.Next(ctx) // scans priority buffers high→low, no per-level round trip
    if err != nil { /* handle */ }

    if err := handle(msg); err != nil {
        msg.Nak()               // prompt redelivery
        continue
    }
    msg.Ack()                   // only after your work is durable
}
```

Delivery semantics:

- **At-least-once**: the same message can arrive more than once (NAK, ack-wait expiry, restart). Deduplicate on the application side using the publish ID.
- **Ack only after durable work**; `Nak` for immediate retry, `Term` for poison messages (no redelivery — route them to your DLQ process).
- **Close during graceful shutdown** so buffered messages are NAKed and promptly redelivered by another consumer.
- **Prefetch** (`ConsumerConfig.Prefetch`) is the per-process local buffer budget; the default splits 256 across levels. Prefetched lower-priority messages may already be assigned when a higher-priority message arrives — the same practical boundary RabbitMQ has.

## 4. Verify in the console

Open the Queue detail: **stored messages** should track your publish rate, the consumer shows pending/ack-pending for backlog triage, and **Audit** records the declaration change you made in step 1.

## 5. Next steps

- Retention, DLQ policy, and delivery limits: declare them in `spec` (see [Queue document schema](webui-queue-schema.md) and the [glossary](glossary.md)).
- Disaster recovery: `rjsctl backup create/verify/restore` ([Backup and Restore](backup-restore.md)).
- Migrating definitions from RabbitMQ: [rjsctl migrate](rabbitmq-migration.md).
- Errors: [Troubleshooting](troubleshooting.md).
