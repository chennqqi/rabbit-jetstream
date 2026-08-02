# Capacity Planning

Capacity is workload-specific and must be measured with the pinned NATS build, storage class, replica count, message sizes, retention, and consumer behavior.

For each Queue, estimate retained logical bytes as:

```text
logical bytes = ingress messages/s × average stored bytes/message × retention seconds
cluster bytes = logical bytes × replicas × 1.3 overhead factor
per-node bytes = cluster bytes / storage nodes
required PVC = per-node bytes / 0.70
```

Use the larger result from time retention, `maxBytes`, backlog during the longest consumer outage, and DLQ retention. Include the 1 GiB audit Stream, metadata/KV, snapshots, compaction workspace, and other Streams. The 30% free-space reserve is a starting safety floor, not a universal guarantee.

Size throughput for peak, not average traffic. Benchmark sustained publish and drain rates at replica count 3, then keep peak ingress below 70% of the lower measured rate. A recovery target requires drain rate greater than ingress rate:

```text
recovery seconds = backlog messages / (drain messages/s - ingress messages/s)
```

Measure P50/P95/P99 publish acknowledgement latency, consume throughput, redelivery, CPU, RSS, disk latency/IOPS, network, JetStream storage, replica lag, and API error rate. Test small and large payloads separately; per-message overhead makes byte-only estimates unsafe.

Use `tests/performance/jetstream.ps1 -ConsumerStartDelay 00:00:30 -ConsumerDelay 00:00:00.001` to create a controlled slow-consumer backlog. The report records `peak_backlog_messages`, `backlog_at_publish_end`, `drain_seconds`, and `drain_messages_per_second`; retain these workload parameters in baseline evidence so recovery measurements are comparable.

Run `tests/performance/native-sdk-compare.ps1` with the independent SDK checkout available through `outlink/rabbit-jetstream-go`. It executes direct `nats.go` and priority SDK workloads sequentially against the same three-node cluster and gates SDK/direct publish throughput, consume throughput, and PubAck P99 ratios. The direct path uses batch 1 so both clients acknowledge one message at a time.

Alert before exhaustion. The shipped backlog alert is only a placeholder; tune it from the declared outage budget and drain test. Add storage alerts at 70% warning and 85% critical, and require capacity review before changing retention, replicas, priority levels, or DLQ policy. Re-run benchmarks after NATS, filesystem, kernel, instance type, or storage-class changes.
