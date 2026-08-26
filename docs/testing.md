# Production Test Strategy

[English](testing.md) | [简体中文](testing.zh-CN.md)

Replacing RabbitMQ makes correctness and recoverability release requirements, not optional QA work.

## Test Pyramid

1. **Unit and contract tests** cover configuration, API validation, resource naming, policy reconciliation, and failure branches.
2. **Integration tests** run against the exact vendored NATS Server version in single-node and three-node modes.
3. **Fault tests** exercise leader loss, rolling restart, network interruption, full/slow disk, process crash, duplicate delivery, and reconnect.
4. **Compatibility tests** verify documented queue, ack, redelivery, TTL, DLQ, routing, ordering, and priority semantics jointly with every SDK.

Run `pwsh tests/integration/native-sdk.ps1` from a checkout whose `outlink/rabbit-jetstream-go` resolves to the independent SDK repository. The script uses Docker Desktop only: the server control plane creates the managed priority topology, then the Linux SDK test publishes and consumes without calling its standalone provisioning helper.

The SDK repository's isolated `test/fault` module uses an embedded persistent NATS server and a real TCP forwarding proxy. Linux `go test -race` verifies recovery from broker restart, bidirectional latency, and forced client disconnect without adding a proxy product to the deployment architecture.
5. **Performance and soak tests** compare pinned baselines using declared hardware, persistence, replicas, payload sizes, and workloads.

## Merge and Release Gates

- New or changed Go code needs meaningful tests and must not reduce coverage. The repository requires 80% statement coverage; `internal/topology` and `management/internal/controller` independently require 90%, with explicit boundary and failure-path tests for branch behavior.
- Pull requests run formatting, vet, race-enabled unit tests, integration tests, vulnerability scanning, and deterministic compatibility tests.
- A release candidate must pass three-node failure recovery, backup/restore, rolling upgrade/rollback, a 24-hour soak, and performance-regression gates.
- Flaky tests are production defects. Quarantine requires an owner, linked issue, and expiry date.

The repository-wide 80% gate is executable and fails below the threshold:

```bash
make coverage-check
```

The security gate runs `govulncheck v1.6.0`, builds all three release images, and rejects fixable HIGH/CRITICAL findings using the digest-pinned Trivy 0.70.0 scanner:

```powershell
make test-security
```

The NATS image may apply only documented, version-pinned dependency security overrides at build time; it never edits the upstream subtree. Scanner database updates can change findings without a source change and must be triaged as release-blocking evidence, not silently allowlisted.

From PowerShell, without WSL:

```powershell
.\tests\coverage\check.ps1
```

Set `RJS_COVERAGE_MIN` or `RJS_CRITICAL_COVERAGE_MIN` only to raise thresholds in stricter release pipelines; lowering committed baselines is not an acceptable merge workaround.

Test evidence and benchmark reports are retained with each release. Claims of RabbitMQ replacement or performance advantage require published, repeatable workloads.

The client compatibility contract is stored in `api/client-compatibility.json`. `go test ./tests/contract` rejects unknown fields, unsafe or duplicate entries, missing implementation evidence, missing guide coverage, and any accidental claim that AMQP or the native SDK is already available. Update the contract, guide, implementation evidence, and compatibility tests together; a documentation-only status upgrade is not acceptable.

## Linux Production Baseline

Linux containers are the primary production target. Every pull request builds Linux images and runs the standalone smoke test plus a three-node, three-replica single-node-outage scenario. Image builds cover `linux/amd64` and `linux/arm64`. Windows tests are developer feedback only and cannot replace these gates.

Helm changes run `tests/deployment/helm.ps1`: chart lint, values-schema rejection, Kubernetes API schema validation, rendered NATS configuration parsing, authenticated standalone readiness, a real mutual-TLS handshake/readiness check, and a real three-node JetStream quorum/controller-leader check through Docker Desktop or a Linux CI runner.

CI also executes every management black-box scenario independently: API/OpenAPI, read-only reconcile, idempotent apply, protected delete, durable audit, RBAC rotation, direct/topic/fanout routing, DLQ transfer, metrics/alerts, diagnostic bundles, and multi-instance controller failover. Adding a scenario to `docker-desktop.ps1` requires adding it to the workflow matrix unless another mandatory job proves the same behavior.

The mandatory Kubernetes smoke gate creates a pinned three-worker kind cluster on native Linux, loads locally built NATS and management images, installs the Helm chart, waits for the StatefulSet and Deployment, verifies three Bound PVCs and three distinct NATS nodes, runs the chart test hook, and checks management readiness plus the embedded Admin UI through a Service port-forward. This is stronger than template validation but still does not replace a production StorageClass, CNI, ingress, or 24-hour soak drill.

Test helper containers are referenced by immutable registry digests, and GitHub Actions by full commit SHA. Update these deliberately when upgrading tool behavior; floating tags such as `latest` are rejected by contract tests.

Run the same checks through Docker Desktop's Linux engine:

```bash
make test-linux-smoke
make test-linux-fault
```

On Windows, call Docker Desktop directly from PowerShell; WSL is not required:

```powershell
.\tests\integration\docker-desktop.ps1 standalone
.\tests\integration\docker-desktop.ps1 api
.\tests\integration\docker-desktop.ps1 reconcile
.\tests\integration\docker-desktop.ps1 apply
.\tests\integration\docker-desktop.ps1 delete
.\tests\integration\docker-desktop.ps1 audit
.\tests\integration\docker-desktop.ps1 auth
.\tests\integration\docker-desktop.ps1 routing
.\tests\integration\docker-desktop.ps1 dlq
.\tests\integration\docker-desktop.ps1 metrics
.\tests\integration\docker-desktop.ps1 diagnostics
.\tests\integration\docker-desktop.ps1 controller
.\tests\integration\docker-desktop.ps1 fault
.\tests\integration\rolling-upgrade.ps1 -BuildLocal
.\tests\integration\migration.ps1
.\tests\integration\shadow-capture.ps1
.\tests\integration\backup-restore.ps1
.\tests\performance\jetstream.ps1 -Mode ci -Output performance-ci.json
```

The CI profile verifies a three-replica persistent workload and retains its report. Large-scale and release-soak profiles, baseline rules, resource evidence, and interpretation limits are defined in [Performance and Soak Testing](performance-testing.md). The release profile enforces a minimum 24-hour duration; the short CI profile never satisfies that gate.
