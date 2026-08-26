# Roadmap

[English](roadmap.md) | [简体中文](roadmap.zh-CN.md)

Milestones are accepted by evidence, not by unreviewed dates. The first stable release uses the separate native `rabbit-jetstream-go` SDK and targets RabbitMQ-style priority Queue behavior, management, operations, and deployment. It does not accept AMQP clients unchanged.

## Delivered for v0.1

- Pinned, unmodified upstream NATS Server subtree and reproducible images.
- Versioned Queue declaration, deterministic planning, safe reconciliation, CAS revisions, ownership checks, and persistent metadata.
- Direct, topic, and fanout routing; TTL, capacity, explicit acknowledgement, redelivery, DLQ, and dynamic priority levels.
- Native SDK scheduling, publisher acknowledgement, backpressure, fairness, and server/SDK contract tests.
- Management API, `rjsctl`, embedded Admin UI, audit, RBAC/OIDC, metrics, traces, diagnostics, backup/restore, and migration tooling.
- Standalone Compose, three-node Compose, and production-gated Helm deployment.
- Fault, race, security, performance, rolling upgrade, restore, and 24-hour native Linux qualification.

## Before v0.1.0 Stable

- Qualify Admin UI workflows with Chromium and Firefox E2E, accessibility checks, and operational error scenarios.
- Complete a controlled production canary, rollback rehearsal, and signed release approval for the exact immutable artifacts.
- Collect operator feedback and close release-blocking defects without changing the published compatibility contract.

## v0.2 Operations and Scale

- Expand workload and namespace tenancy policies, quota reporting, and safe self-service workflows.
- Add longer capacity baselines, storage/CNI qualification matrices, and automated disaster-recovery objectives.
- Improve dashboards, alert routing, audit export, and fleet-level diagnostics.
- Production-qualify `linux/arm64` when native hardware is available.

## Future: RabbitMQ/AMQP Compatibility

Full AMQP 0-9-1 compatibility is explicitly outside the first release. Future research may introduce an independent protocol gateway for exchange/queue/binding, confirm, QoS, cancel, transaction, exclusive, and auto-delete semantics. Any need to modify upstream NATS must be proposed as a separate architecture decision with compatibility and performance budgets.

## Quality Policy

Every milestone requires public compatibility notes, automated tests, security review, performance evidence, observability, and tested upgrade/rollback steps. Claims relative to RabbitMQ must publish workload, durability, replica count, message size, and hardware; unmeasured superiority is not a release claim.
