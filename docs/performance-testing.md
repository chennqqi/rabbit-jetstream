# Performance and Soak Testing

The performance harness runs against the repository's pinned NATS Server build in a real three-node cluster. Streams use file storage, WorkQueue retention, and three replicas. Publishers wait for a JetStream PubAck for every message while a durable pull Consumer drains concurrently. Each deterministic payload contains its sequence; the run fails on a publish error, missing message, duplicate, corruption, consumer timeout, NATS node exit, or container restart.

The JSON report records the NATS version, OS/architecture, CPU count, workload shape, exact counts, elapsed time, publish and consume rates, and publish acknowledgement P50/P95/P99/max. Latency storage is a bounded histogram, so a 24-hour run does not grow benchmark memory with message count. Timestamped Docker CPU, memory, network, block-I/O, and PID samples are written beside the report as `.resources.ndjson`.

## Workload profiles

Run directly through Docker Desktop; WSL is not used:

```powershell
# Pull-request signal: 20,000 × 1 KiB messages
.\tests\performance\jetstream.ps1 -Mode ci -Output performance-ci.json

# Large-scale integrity and throughput run: 1,000,000 × 1 KiB messages
.\tests\performance\jetstream.ps1 -Mode scale -Output performance-scale.json

# Exercise the time-based path during development (not a release soak)
.\tests\performance\jetstream.ps1 -Mode duration -Duration 00:10:00

# Required release soak: at least 24 hours and compared with an approved baseline
.\tests\performance\jetstream.ps1 -Mode soak -Duration 24:00:00 `
  -Baseline performance-baseline.json -Output performance-soak.json

# First release only: establish the inaugural baseline with explicit absolute gates
.\tests\performance\jetstream.ps1 -Mode soak -Duration 24:00:00 `
  -InauguralBaseline -MinPublishMessagesPerSecond 4900 `
  -MinConsumeMessagesPerSecond 4900 -MaxPublishLatencyP99Millis 10 `
  -Output performance-soak.json
```

Override `-PayloadBytes`, `-Publishers`, and `-Batch` to model each production workload. Test small and large payload cohorts separately. `ci` is only a correctness and gross-regression signal. It is not long enough to satisfy release soak acceptance. `scale` defaults to one million messages. `duration` exercises continuous operation but is not release evidence. `soak` rejects durations below 24 hours and requires retained output plus either an approved baseline or the first-release-only inaugural mode.

## Baselines and release evidence

Create a baseline on the same dedicated hosts, Docker/NATS limits, storage, kernel, architecture, payload size, publisher count, batch size, and replica topology as the candidate. The gate refuses workload-shape mismatches. By default it fails when publish or consume throughput drops more than 20%, or publish P99 rises more than 30%. Tighten thresholds using the service SLO; do not widen them to make a release pass.

When no prior release exists, one reviewed 24-hour run may establish the inaugural baseline. Inaugural evidence has no prior-baseline artifact and therefore must declare positive absolute publish, consume, and P99 limits. It retains every integrity, topology, native-Linux, duration, provenance, and resource-continuity gate. Preserve its report as the mandatory comparison baseline for subsequent releases; inaugural mode is not valid merely because a later candidate lacks a convenient matching baseline.

Comparison soak mode writes four inseparable release artifacts: the candidate report, `.resources.ndjson`, `.baseline.json`, and `.evidence.json`. Inaugural mode omits only `.baseline.json`. The evidence manifest records SHA-256 for every referenced artifact, source revision, built NATS image ID, exact workload parameters, comparison or absolute limits, host/kernel/CPU/memory/storage details, and sample interval. It is independently verified before the command succeeds.

Recheck retained evidence before promotion or after copying it:

```powershell
go run ./tools/perfevidence -evidence performance-soak.json.evidence.json -require-soak
```

The verifier rejects modified or missing artifacts, a run shorter than 24 hours, incomplete/corrupt messages, mismatched workload shape, excessive throughput/P99 regression, sparse resource sampling, or fewer than three sampled nodes. Review resource samples for sustained growth, throttling, disk saturation, compaction behavior, and recovery headroom. A passing integrity report does not by itself prove capacity, and results from Docker Desktop must not be presented as Linux production hardware performance.

Native-host sampling produced by `tests/helpers/resource-sampler` can be audited without loading the NDJSON into memory:

```bash
go run ./tools/resourceaudit -input performance-soak.resources.ndjson \
  -output performance-soak.resources.summary.json
```

The audit requires exactly three equally sampled nodes and fails on malformed samples, unhealthy nodes, slow consumers, stalled clients, or non-monotonic node timestamps. Its summary records CPU, memory, JetStream storage, metadata pending, and minimum host free-space watermarks for operational review.

For a first-release native-host run, create the manifest from the retained preflight rather than hand-writing artifact hashes:

```bash
go run ./tools/perfevidence -create-inaugural \
  -evidence evidence.json -report candidate.json -resources resources.ndjson \
  -native-preflight native-linux-preflight.json \
  -nats-image-id sha256:<64-hex-image-id> -sample-interval 60 \
  -min-publish 4900 -min-consume 4900 -max-p99 10 \
  -command-json '["jetstream-bench","--duration","24h","--publish-rate","5000"]'
```

The generator requires the report, samples, and new manifest to share one directory, hashes both artifacts, derives host and frozen revision provenance from the preflight, writes the manifest exclusively, and immediately runs the independent 24-hour verifier.
