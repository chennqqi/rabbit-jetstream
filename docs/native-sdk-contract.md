# Native SDK Contract

`api/native-sdk-contract.json` is the machine-readable boundary between this server distribution and the independent `rabbit-jetstream-go` client. It is marked `native-sdk-implemented-unreleased`: the SDK implementation and production-oriented tests exist, but applications must not depend on it until the independent repository publishes a versioned release.

Running management instances expose the identical document at `GET /api/v1/native-sdk-contract.json`; clients may use it for startup compatibility checks. The response is public metadata, contains no credentials and is cached for five minutes.

The contract fixes queue-scoped Stream, Consumer and subject names plus required message headers. Publishers must supply a stable `Nats-Msg-Id`, the logical Queue, the Queue plan revision and the contract version. Priority publishers additionally use `Rjs-Priority` and publish to `rjs.q.{queue}.p.{priority}`. Out-of-range priorities are rejected instead of silently clamped.

Each priority level has a non-overlapping pull Consumer. Higher numbers are preferred, but already delivered messages are never preempted. Implementations must use the specified bounded strict-priority loop: after a configurable high-priority burst, probe lower levels so sustained high traffic cannot permanently starve them. A Go `select` across subscriptions is not a priority algorithm.

The contract promises at-least-once delivery and JetStream PubAck publisher confirmation. SDK implementations must bound outstanding fetch count and bytes, preserve message IDs across redelivery, and pass repeatable ordering, starvation, backpressure and broker-failure tests before changing `availability` or the compatibility matrix.

Declare a managed priority Queue with `spec.maxPriority`. The control plane creates exact `p.0 ... p.N` Stream subjects and one explicit-ack durable pull Consumer per level. DLQ transfer preserves `Rjs-Priority`: a priority target receives the corresponding `p.N` subject and must support the source priority range; an ordinary target receives the message through its ingress subject with the priority header retained for diagnostics.
