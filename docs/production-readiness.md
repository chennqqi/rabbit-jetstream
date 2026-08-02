# Production Readiness and Canary Runbook

This runbook promotes the paired server and Native SDK release. It does not certify AMQP compatibility.

## Freeze the Candidate

Record the clean server and SDK 40-character revisions, `SDKVersion`, `ContractVersion`, NATS subtree tag and intended image digests. Run `make test-local-release` and retain `artifacts/local-rc.json` plus its hashed performance report. Any source, dependency, chart or build-argument change invalidates the evidence and requires a new run.

## Native Linux Soak

Use a dedicated native Linux host matching production architecture, kernel, filesystem, storage class, CPU/memory limits and Docker configuration. Docker Desktop and WSL are invalid evidence.

Before starting a multi-day qualification, copy the frozen bundle to a clean checkout of its server revision and run:

```bash
make verify-native-bundle \
  BUNDLE=/srv/rabbit-jetstream/v0.1.0-rc.1 \
  NATIVE_QUAL_OUTPUT=native-linux-preflight.json
```

The preflight fails outside native Linux, on WSL or Docker Desktop, on a dirty or mismatched checkout, on any checksum mismatch, when an OCI archive lacks either `linux/amd64` or `linux/arm64`, or when the current-architecture `rjsctl` and management binaries do not report the bundle version. Retain the exclusive-create JSON output with the soak evidence. Run it once on production-matching amd64 hardware and once on production-matching arm64 hardware when both architectures are release targets; metadata-only cross-build inspection does not replace execution.

If no approved matching baseline exists, first run the same workload for 24 hours and review it as the baseline:

```powershell
pwsh tests/performance/jetstream.ps1 -Mode duration -Duration 24:00:00 `
  -Output performance-baseline.json
```

Then run and verify the candidate from the frozen server revision:

```powershell
pwsh tests/performance/jetstream.ps1 -Mode soak -Duration 24:00:00 `
  -Baseline performance-baseline.json -Output performance-soak.json
make verify-soak
```

Retain all four candidate files: the report, resource samples, copied baseline and evidence manifest. Acceptance requires zero missing/corrupt messages, no node exit/restart, three continuously sampled nodes, publish and consume throughput no worse than 20% below baseline, and publish P99 no more than 30% above baseline. Review CPU, RSS, disk latency/IOPS, network, replica lag and storage trend manually; a mathematically passing report with exhaustion trend is rejected.

## Canary Sequence

Deploy immutable image digests and the paired SDK version. Keep the previous deployment and, for a RabbitMQ migration, the source RabbitMQ topology and rollback journal intact.

1. Deploy an isolated low-risk Queue cohort. Verify readiness, controller leadership, three current replicas, Admin UI/API, metrics, audit and diagnostics.
2. Run synthetic priority traffic covering every priority, duplicate IDs, Ack/Nak/redelivery, backpressure and priority-preserving DLQ. Require zero missing/corrupt messages.
3. Route approximately 1%, 10%, 25%, 50% and then 100% of the selected cohort, with at least one declared peak/backlog recovery window at each material step. Do not advance on elapsed time alone.
4. At or before 10%, stop one NATS node, confirm continued PubAck/consume service and replica convergence, then restore it.
5. Expand by Queue cohort only after application owners approve payload fidelity, idempotency, ordering assumptions and every `partial` compatibility item they use.

At every stage require: JetStream available; controller active; all expected nodes available; no DLQ transfer failures; management 5xx below 5%; storage below 70%; backlog below the workload-specific alert; publish P99 within its SLO and the 30% baseline limit; and throughput at least 80% of baseline. Application duplicate rate must remain within its declared at-least-once budget.

## Stop and Roll Back

Stop promotion immediately on message loss/corruption, unbounded duplicates, lost quorum, replicas that do not converge, PubAck failure, sustained SLO/error/backlog breach, DLQ failure, storage above 85%, metadata mismatch or an unapproved compatibility dependency. Disable new routing to the candidate, preserve diagnostics/audit/performance evidence, drain or reconcile confirmed messages, and roll back management then NATS nodes using the rehearsed reverse order. Never overwrite a live divergent cluster with a backup.

Formal `v0.1.0` approval requires sign-off from the service owner, application owner and on-call operator on the paired revisions, local Release evidence, native-Linux preflight and soak evidence, canary observations, rollback result and known limitations.

Copy [`release-approval.template.json`](release-approval.template.json), replace every placeholder, export one immutable observation file per canary stage plus node-failure and rollback evidence, and calculate each SHA-256. The template must list every `partial` compatibility dependency used by the application; `priority-queue` is mandatory for this release. After the 100% observation window and all three sign-offs, run:

```bash
make verify-release-approval RELEASE_APPROVAL=/srv/rabbit-jetstream/release-approval.json
```

The verifier binds local Release, amd64 and arm64 preflight, soak, canary, fault and rollback artifacts to the paired revisions; enforces the documented thresholds and exact promotion sequence; records the AMQP and at-least-once limitations; and rejects early, duplicate or missing sign-offs. It validates evidence completeness and integrity, not the truthfulness of manually exported metrics, so reviewers must still inspect the referenced observability exports.
