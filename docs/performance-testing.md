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
```

Override `-PayloadBytes`, `-Publishers`, and `-Batch` to model each production workload. Test small and large payload cohorts separately. `ci` is only a correctness and gross-regression signal. It is not long enough to satisfy release soak acceptance. `scale` defaults to one million messages. `duration` exercises continuous operation but is not release evidence. `soak` rejects durations below 24 hours and requires both a baseline and retained output.

## Baselines and release evidence

Create a baseline on the same dedicated hosts, Docker/NATS limits, storage, kernel, architecture, payload size, publisher count, batch size, and replica topology as the candidate. The gate refuses workload-shape mismatches. By default it fails when publish or consume throughput drops more than 20%, or publish P99 rises more than 30%. Tighten thresholds using the service SLO; do not widen them to make a release pass.

Retain the candidate report, resource samples, baseline, image digests, host/storage specification, command line, and test timestamps with the release. Review resource samples for sustained growth, throttling, disk saturation, compaction behavior, and recovery headroom. A passing integrity report does not by itself prove capacity, and results from Docker Desktop must not be presented as Linux production hardware performance.
