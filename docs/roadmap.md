# Roadmap

[English](roadmap.md) | [简体中文](roadmap.zh-CN.md)

Milestones are accepted by evidence, not by unreviewed dates. The roadmap keeps two views of the same work: the [Version Plan](#version-plan) records feature planning per version, and the [Features Plan](#features-plan) records feature planning per capability area. Version status and release gates live in the Version Plan; feature detail and acceptance records live in the Features Plan. The first stable release uses the separate native `rabbit-jetstream-go` SDK and targets RabbitMQ-style priority Queue behavior, management, operations, and deployment. It does not accept AMQP clients unchanged.

## Version Plan

Feature planning by version.

### Version Scope

The first formal release uses the independent native SDK `rabbit-jetstream-go` and targets RabbitMQ-style priority queues with matching management, operations, and deployment capabilities at the functional layer. The native SDK is built on the NATS JetStream client and does not require AMQP. Full RabbitMQ/AMQP wire compatibility is outside the first release's acceptance scope and is recorded under M5 in the Features Plan.

The dual-repository version set, release gates, and production evidence requirements for the first release candidate are defined in the [First Release Plan](first-release-plan.md).

### v0.1: First Stable Release

#### Delivered

- Pinned, unmodified upstream NATS Server subtree and reproducible images.
- Versioned Queue declaration, deterministic planning, safe reconciliation, CAS revisions, ownership checks, and persistent metadata.
- Direct, topic, and fanout routing; TTL, capacity, explicit acknowledgement, redelivery, DLQ, and dynamic priority levels.
- Native SDK scheduling, publisher acknowledgement, backpressure, fairness, and server/SDK contract tests.
- Management API, `rjsctl`, embedded Admin UI, audit, RBAC/OIDC, metrics, traces, diagnostics, backup/restore, and migration tooling.
- Standalone Compose, three-node Compose, and production-gated Helm deployment.
- Fault, race, security, performance, rolling upgrade, restore, and native Linux qualification (the exact-revision capacity soak is tracked in the release closure below).

The complete feature records and acceptance criteria for v0.1 are M0–M4 in the [Features Plan](#features-plan).

#### Release Closure (in progress)

The M0–M4 product and qualification work is complete; what remains is making the approval package verifiable and publishing. Current status detail: [release readiness analysis 2026-10-04](release-readiness-analysis-2026-10-04.md).

- [ ] Complete the tier-1 24-hour capacity soak on the 2 vCPU/4 GB qualification host (`jdcloudremote`) against the final frozen revision: sustained concurrent publish+consume ≥ 3,000 msg/s, publish P99 ≤ 10 ms, zero integrity defects; convert the result with `tools/perfevidence -create-inaugural` bound to the final revision. (Started 2026-10-04 21:40 CST, scheduled to finish 2026-10-05 21:40 CST.)
- [ ] Formalize the tier-2 envelope (4 vCPU/8 GB, 5,000 msg/s) with a 24-hour soak once a host is available; until then it remains anchored by the 32-core 24-hour soak plus 2-core calibration extrapolation and is marked pending formalization, not a certified claim.
- [ ] Write the dual-tier performance envelope into the qualified profile of the rc.3 release notes, semantically identical in English and Chinese.
- [ ] Re-run `make test-local-release` (Release mode) at the final frozen revision and restore the releaseapproval tool's strict `mode == "release"` assertion; accepting quick mode weakens the gate.
- [ ] Re-bind all evidence (approval JSON, soak, canary, cluster, local-rc) to the single final frozen revision; keep the approval commit last or re-bind after every commit; record "zero runtime diff since `033010ee`" in the evidence notes.
- [ ] Fix evidence hygiene: the five byte-identical gzip `canary-XXX-stage.json` files must become real per-stage JSON.
- [ ] Make `make verify-release-approval` pass at the final revision.
- [ ] Confirm GitHub Actions green on the final revision, watching `rabbitmq-migration` and `kubernetes-smoke`.
- [ ] Push the final tag and run `release.yml`: image publication with attestations, Helm packaging, SHA256SUMS, and draft-to-published release.
- [ ] Decide and implement public signing (GPG/cosign) and redistribution terms.
- [ ] Backfill `docs/remaining-release-work.md` and `docs/completion-assessment.md` to the rc.3 state.
- [ ] Record the canary drill-substitution boundary (qualification-host drill versus live deployment) in the approval record, or execute a live canary.
- [ ] Collect RC observation feedback and close release-blocking defects without changing the published compatibility contract.

Acceptance: `make verify-release-approval` passes at the tagged revision and the release workflow completes; promotion to `v0.1.0` GA follows the [First Release Plan](first-release-plan.md) exit criteria.

### v0.2: Operations Expansion and Real OIDC/IdP Integration

- [ ] Integrate and qualify browser SSO against a real external OIDC/IdP deployment, including provider-specific client registration, login/callback, role mapping, expiry, logout, key rotation, failure handling, and operator runbooks. This work is explicitly deferred and is not a blocker for completing the current local WebUI/backend development scope; the existing static operator/auditor bearer-token path remains supported.
- [ ] Expand workload and namespace tenancy policies, quota reporting, and safe self-service workflows.
- [ ] Add longer capacity baselines, storage/CNI qualification matrices, and automated disaster-recovery objectives.
- [ ] Improve dashboards, alert routing, audit export, and fleet-level diagnostics.
- [ ] Production-qualify `linux/arm64` when native hardware is available.
- [ ] Build a standalone website covering both introduction and functionality: a content side with project overview, documentation, release notes, and download guidance, and a functional side that decouples the embedded Admin UI console into an independently deployable web application, deployable and upgradable separately from the management service.

The repository already contains OIDC bearer verification and a locally tested browser Authorization Code + PKCE implementation. The roadmap item above covers further real-provider integration and qualification only. Until that item is resumed, do not treat external IdP availability or production IdP acceptance as part of the active development completion criteria.

The built-in account/session and application-level tenancy items under M3 are active local WebUI/backend requirements; they are distinct from the deferred external OIDC/IdP qualification and the broader v0.2 quota/self-service tenancy work.

### Future (unscheduled)

- AMQP 0-9-1 protocol gateway research — full record in M5 of the [Features Plan](#features-plan).

## Features Plan

Feature planning by capability area (M0–M5). Each record states its delivering version; per-version status and release gates live in the [Version Plan](#version-plan).

### M0: Runnable Skeleton (delivered in v0.1)

- [x] Pinned official NATS Server Git subtree at a fixed version;
- [x] Standalone management service and read-only status CLI;
- [x] Environment-variable/flag configuration, structured logging, and graceful shutdown;
- [x] NATS/JetStream connection, liveness/readiness checks, and an account summary;
- [x] Standalone and three-node Compose examples and container images;
- [x] Basic unit tests and build entry points;
- [x] Production-grade test layering, coverage targets, and release gate definitions.

Acceptance: `go test ./...` and `go build ./...` pass; after connecting to a JetStream-enabled NATS, `/readyz` returns 200.

### M1: Queue Control Plane MVP (delivered in v0.1)

- [x] Read-only account, Stream, and Consumer APIs with stable models, pagination, and error structures;
- [x] Node and NATS monitoring API aggregation that tolerates partial node failure;
- [x] Queue declaration model, versioned schema, strict validation, and field-level diff;
- [x] Pure Queue → Stream/durable Consumer/DLQ dependency mapping and a deterministic plan;
- [x] Read-only reconcile from observed state to create/update/noop/recreate/reject;
- [x] Bearer-token-protected declarative Queue apply with safe create/update and idempotent noop on repeat;
- [x] Ownership checks, non-empty protection, and explicit-confirm idempotent Queue delete;
- [x] JetStream KV-backed Queue declaration/revision persistence with restart-recovery queries;
- [x] Idempotent creation, query, and deletion of owned Streams and durable Consumers through Queue declaration;
- [x] Direct, topic, and fanout Queue bindings, a queue-level subject protocol, and real JetStream routing tests;
- [x] TTL, length limits, explicit ack/AckWait/MaxDeliver, and durable advisory DLQ baseline policies;
- [x] Declarative apply, diff, and delete via CLI and API;
- [x] JetStream KV CAS lease-based leader election with leader-only continuous safe reconcile;
- [x] Multi-instance Queue-level CAS locks, revision preconditions, and conflict responses.

Acceptance: declarations survive process restarts and management instance failover; repeated apply has no side effects; end-to-end tests cover publish, consume, redelivery, and DLQ.

### M2: Native SDK and Priority Queues (delivered in v0.1)

- [x] Server-published machine-readable `v1alpha1` contract for resource naming, message headers, priority scheduling, and delivery semantics;
- [x] `rabbit-jetstream-go` implemented and passing compatibility tests, jointly solidifying that contract;
- [x] Multi-subject/pull-consumer priority scheduling and server-side `spec.maxPriority` resource creation;
- [x] Fairness, starvation protection, dynamic priority counts, and explicit rejection of unsupported combinations;
- [x] Publisher confirm, consume concurrency, and message-count/byte backpressure;
- [x] Fault injection covering node failure, broker restart, TCP latency/disconnection, slow consumers, and backlog recovery;
- [x] Same-host three-node publish P99, publish throughput, and consume throughput benchmarks comparing the native SDK against direct JetStream clients, with ratio gates.

Acceptance: priority behavior, fault recovery, and at-least-once delivery have repeatable tests; publish P99 and consume throughput benchmarks are comparable against native JetStream.

### M3: Operations Productization and WebUI (delivered in v0.1)

- [x] Queue, consumer, backlog, node, and cluster pages plus RBAC/ETag/audit-protected Queue apply/delete;
- [x] Prometheus metrics, pinned-version deployment profiles, and key alert templates;
- [x] OpenTelemetry OTLP/HTTP traces with W3C context extraction, Queue control-plane attributes, and shutdown flush;
- [x] OpenTelemetry OTLP/HTTP periodic metrics export with shutdown flush, retaining the Prometheus endpoint;
- [x] CLI diagnostics bundle with redaction, partial-failure records, and a SHA-256 manifest;
- [x] JetStream-based write-ahead-intent/write-behind-result audit log with a protected query API and CLI;
- [x] Static operator/auditor minimal RBAC with overlapping-token zero-downtime rotation;
- [x] OIDC discovery/JWKS federated authentication, operator/auditor role mapping, and signing-key rotation;
- [x] Helm chart with three/five-node StatefulSets, dual management replicas, and PVC/PDB/NetworkPolicy;
- [x] Three-node rolling upgrade/rollback automation drills and a capacity planning handbook;
- [x] Full-account Stream/KV/Consumer backup, verification, restore, and data-volume destruction drills;
- [x] Embedded OpenAPI v1 contract, route consistency tests, and a breaking-change CI gate;
- [x] Replace the primary manual bearer-token login with built-in username/password authentication that returns a short-lived access token for the `Authorization` header. Keep it only in page memory; do not persist passwords or tokens in `localStorage`, cookies, or URLs. Retain static bearer tokens only for recovery and automation APIs;
- [x] Application-level multi-tenancy with explicit tenant identity in browser URLs and API headers, membership checks, a UI tenant selector, and per-tenant isolation of NATS credentials/connections, monitoring, controllers, Consumer caches, diagnostics ownership, and audit evidence. API and headless-browser isolation tests fail closed across tenant boundaries;
- [x] Local platform-administrator access management covering account lifecycle and tenant memberships, with password-hash non-disclosure, last-administrator protection, atomic persistence, and immediate token invalidation after authorization changes. Tenant backend credentials remain protected startup configuration;
- [x] Tenant-local membership roles so one identity can be an operator in one tenant and an auditor in another; backend authorization selects the target tenant role before checking permission, and UI tenant switching replaces rather than merges permissions;
- [x] Compact desktop filter layout for the Queue/Stream and global Consumer lists while preserving the stacked mobile layout and zero page-level horizontal overflow at 375 px;
- [x] Admin UI workflow qualification with Chromium and Firefox E2E, accessibility checks, and operational error scenarios.

Acceptance: three-node fault and rolling upgrade drills pass; key SLIs have dashboards and alerts; permission and audit tests pass.

### M4: RabbitMQ Migration Capability (delivered in v0.1)

- [x] Strict conversion from RabbitMQ definitions to Queue declarations with compatibility reports and validation tooling;
- [x] Shadow data verification CLI based on stable message IDs, SHA-256, and size, with threshold gates and non-overridable evidence reports;
- [x] Real collection adapters for a RabbitMQ standalone shadow queue and a JetStream Limits-retention shadow Stream;
- [x] RabbitMQ mandatory publisher confirm, JetStream PubAck, and a recoverable dual-acknowledgement journal batch dual-write adapter;
- [x] Automated cutover and reverse-order rollback orchestration based on continuous reconciliation evidence, plan summaries, idempotent actions, and a persistent journal;
- [x] Migration guides for common AMQP/NATS clients, a machine-readable compatibility matrix, and anti-drift contract tests;
- [x] Large-scale load tests with three-replica file persistence, baseline regression gates, and a mandatory 24-hour long-term stability test toolchain (a release must still retain actual run evidence).

### M5: AMQP 0-9-1 Protocol Gateway (future research)

This milestone covers full RabbitMQ/AMQP protocol compatibility. It is not part of the first formal release and is not a blocker for the Native SDK or priority-queue feature releases.

- Independent gateway prototype and a protocol compatibility matrix;
- Method mapping for exchange/queue/binding, confirm, QoS, cancel, and related verbs;
- Cost assessment for transactions, exclusive/auto-delete, and channel semantics;
- An explicit gateway performance budget and horizontal scaling model.

This stage may require deeper NATS integration, but any JetStream source change must be recorded as a separate decision; it remains outside the current project boundary.

## Future: RabbitMQ/AMQP Compatibility

Full AMQP 0-9-1 compatibility is explicitly outside the first release. The research record lives in M5 of the [Features Plan](#features-plan). Future work may introduce an independent protocol gateway for exchange/queue/binding, confirm, QoS, cancel, transaction, exclusive, and auto-delete semantics. Any need to modify upstream NATS must be proposed as a separate architecture decision with compatibility and performance budgets.

## Quality Policy

Every milestone requires public compatibility notes, automated tests, performance regression baselines, security review, observability, and tested upgrade/rollback steps. Claims relative to RabbitMQ must publish workload, durability, replica count, message size, and hardware; unmeasured superiority is not a release claim.
