# RabbitMQ Client Migration and Compatibility

The first production release targets RabbitMQ priority-queue functionality through the independent native `rabbit-jetstream-go` SDK. The SDK is based on NATS JetStream APIs and is not required to implement AMQP. This release is not an AMQP 0-9-1 server, so existing RabbitMQ clients cannot point at a new host and continue unchanged. Full RabbitMQ/AMQP wire compatibility is a future roadmap item and is not a first-release gate.

The authoritative machine-readable matrix is [`api/client-compatibility.json`](../api/client-compatibility.json). The client's resource, header and scheduler boundary is fixed in the [native SDK contract](native-sdk-contract.md); `native-sdk-implemented-unreleased` still means no supported SDK version is released. `supported` means the server behavior exists and is tested; it does not imply AMQP wire compatibility. `partial` means an important semantic or client abstraction differs. `planned` is unavailable in the current release. `not_supported` must block cutover unless the application removes that dependency.

## Client decision

| Existing integration | Current decision |
|---|---|
| Go `amqp091-go` | Cannot connect; keep RabbitMQ or rewrite against JetStream contracts. |
| Java RabbitMQ Client / Spring AMQP | Cannot connect; no drop-in listener or Spring transport exists. |
| Python Pika / aio-pika / Celery AMQP | Cannot connect; no AMQP or Celery transport exists. |
| .NET RabbitMQ.Client / MassTransit | Cannot connect; no AMQP or MassTransit transport exists. |
| Official NATS clients | Usable for expert integration, but the application must implement routing resolution, idempotency, pull-consumer flow control, and future priority policy. |
| Native `rabbit-jetstream-go` SDK | Implemented and tested in its independent repository; wait for a versioned release before production adoption. |

## Semantic checklist

| Contract ID | Current status | Migration requirement |
|---|---|---|
| `amqp-wire` | not supported | Application code or a future gateway must change. |
| `durable-queue` | supported | Apply the Queue declaration before publishing. |
| `direct-routing`, `topic-routing` | supported | Resolve bindings to the queue-scoped subjects in the Queue plan. |
| `fanout-routing` | partial | Publish once per matched Queue; one NATS publish is not fanout persistence. |
| `publisher-confirm` | partial | Require JetStream PubAck; the provided dual-write adapter also enforces RabbitMQ confirm/mandatory return. |
| `stable-message-id` | supported | Preserve one logical ID in `message_id` and `Nats-Msg-Id`; consumers must be idempotent. |
| `manual-ack`, `redelivery` | partial | Use explicit Ack and size `AckWait`/`MaxDeliver`; RabbitMQ reject/requeue APIs are not reproduced. |
| `prefetch-backpressure` | partial | The SDK bounds in-flight messages and bytes; size these limits from slow-consumer and backlog-recovery benchmarks rather than translating prefetch numerically. |
| `priority-queue` | partial | Control-plane provisioning and SDK scheduling are tested; cutover remains blocked until a versioned SDK release and priority-DLQ policy are available. |
| `queue-ttl`, `length-limit` | partial | Revalidate expiration and overflow behavior; JetStream MaxAge is not per-message TTL. |
| `dead-letter` | partial | Only one declared Queue target is supported; verify advisory mover lag and provenance headers. |
| `ordering` | partial | Concurrent delivery and retry can reorder; key and serialize workloads that require strict ordering. |
| `transactions` | not supported | Replace AMQP transactions with an application outbox and stable IDs. |
| `exclusive-auto-delete`, `headers-plugin-exchanges`, `rabbitmq-stream` | not supported | Redesign or retain RabbitMQ for those workloads. |
| `quorum-queue` | partial | Replication maps to JetStream replicas, not identical RabbitMQ quorum semantics. |
| `rpc-reply-to` | planned | Define an application request/reply contract; RabbitMQ properties are not translated automatically. |

## Per-application cutover

Inventory every exchange, queue argument, client library, confirm mode, ack/requeue path, prefetch setting, retry policy, message property, RPC convention, ordering assumption, and side effect. Convert definitions strictly and treat every converter error as a blocker. For each workload, record the matrix contract IDs it uses and obtain explicit approval for every `partial` item; any `planned` or `not_supported` dependency blocks production cutover.

During adaptation, make the application message ID durable before publishing and make consumers idempotent. Use a transactional outbox for dual publishing. Execute shadow capture and at least two strict reconciliation windows, then use the digest-bound cutover plan described in [Message Migration, Shadow Verification, and Cutover](migration-cutover.md). Keep RabbitMQ producers, consumers, credentials, capacity, and topology ready until the rollback observation window closes.

Acceptance requires application-level tests for payload/property fidelity, routing, confirms, duplicates, redelivery, DLQ, ordering, outage recovery, and backpressure. Priority workloads additionally require the external SDK compatibility suite; low-level NATS success alone is not sufficient evidence.
