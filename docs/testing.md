# Production Test Strategy

Replacing RabbitMQ makes correctness and recoverability release requirements, not optional QA work.

## Test Pyramid

1. **Unit and contract tests** cover configuration, API validation, resource naming, policy reconciliation, and failure branches.
2. **Integration tests** run against the exact vendored NATS Server version in single-node and three-node modes.
3. **Fault tests** exercise leader loss, rolling restart, network interruption, full/slow disk, process crash, duplicate delivery, and reconnect.
4. **Compatibility tests** verify documented queue, ack, redelivery, TTL, DLQ, routing, ordering, and priority semantics jointly with every SDK.
5. **Performance and soak tests** compare pinned baselines using declared hardware, persistence, replicas, payload sizes, and workloads.

## Merge and Release Gates

- New or changed Go code needs meaningful tests and must not reduce coverage. The initial target is 80% line coverage; safety-critical reconciliation and message-semantics packages require 90% branch-oriented coverage.
- Pull requests run formatting, vet, race-enabled unit tests, integration tests, vulnerability scanning, and deterministic compatibility tests.
- A release candidate must pass three-node failure recovery, backup/restore, rolling upgrade/rollback, a 24-hour soak, and performance-regression gates.
- Flaky tests are production defects. Quarantine requires an owner, linked issue, and expiry date.

Test evidence and benchmark reports are retained with each release. Claims of RabbitMQ replacement or performance advantage require published, repeatable workloads.

## Linux Production Baseline

Linux containers are the primary production target. Every pull request builds Linux images and runs the standalone smoke test plus a three-node, three-replica single-node-outage scenario. Image builds cover `linux/amd64` and `linux/arm64`. Windows tests are developer feedback only and cannot replace these gates.

Run the same checks through Docker Desktop's Linux engine:

```bash
make test-linux-smoke
make test-linux-fault
```
