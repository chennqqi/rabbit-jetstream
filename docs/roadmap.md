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

- [x] Replace the primary manual bearer-token login with built-in username/password authentication that returns a short-lived access token for the `Authorization` header. Keep it only in page memory; do not persist passwords or tokens in `localStorage`, cookies, or URLs. Retain static bearer tokens only for recovery and automation APIs.
- [x] Add application-level multi-tenancy with explicit tenant identity in browser URLs and API headers, membership checks, a UI tenant selector, isolated NATS credentials/connections, monitoring, controllers, Consumer caches, diagnostics ownership, and audit evidence. API and headless-browser isolation coverage fails closed across tenant boundaries.
- [x] Add local platform-administrator access management for account lifecycle and tenant memberships, with password-hash non-disclosure, last-administrator protection, atomic persistence, and immediate token invalidation after authorization changes. Tenant backend credentials remain protected startup configuration.
- [x] Add tenant-local membership roles so one identity can be an operator in one tenant and an auditor in another; backend authorization selects the target tenant role before checking permission, and UI tenant switching replaces rather than merges permissions.
- [x] Give Queue/Stream and global Consumer list filters a compact desktop layout while preserving the stacked mobile layout and zero page-level horizontal overflow at 375 px.
- Qualify Admin UI workflows with Chromium and Firefox E2E, accessibility checks, and operational error scenarios.
- Complete a controlled production canary, rollback rehearsal, and signed release approval for the exact immutable artifacts.
- Collect operator feedback and close release-blocking defects without changing the published compatibility contract.

## v0.2 Operations and Scale

- Integrate and qualify browser SSO against a real external OIDC/IdP deployment, including provider-specific client registration, login/callback, role mapping, expiry, logout, key rotation, failure handling, and operator runbooks. This work is explicitly deferred and is not a blocker for completing the current local WebUI/backend development scope; the existing static operator/auditor bearer-token path remains supported.
- Expand workload and namespace tenancy policies, quota reporting, and safe self-service workflows.
- Add longer capacity baselines, storage/CNI qualification matrices, and automated disaster-recovery objectives.
- Improve dashboards, alert routing, audit export, and fleet-level diagnostics.
- Production-qualify `linux/arm64` when native hardware is available.

## Future: RabbitMQ/AMQP Compatibility

Full AMQP 0-9-1 compatibility is explicitly outside the first release. Future research may introduce an independent protocol gateway for exchange/queue/binding, confirm, QoS, cancel, transaction, exclusive, and auto-delete semantics. Any need to modify upstream NATS must be proposed as a separate architecture decision with compatibility and performance budgets.

## Quality Policy

Every milestone requires public compatibility notes, automated tests, security review, performance evidence, observability, and tested upgrade/rollback steps. Claims relative to RabbitMQ must publish workload, durability, replica count, message size, and hardware; unmeasured superiority is not a release claim.

The repository already contains OIDC bearer verification and a locally tested browser Authorization Code + PKCE implementation. The roadmap item above covers further real-provider integration and qualification only. Until that item is resumed, do not treat external IdP availability or production IdP acceptance as part of the active development completion criteria.

The built-in account/session and application-level tenancy items above are active local WebUI/backend requirements; they are distinct from the deferred external OIDC/IdP qualification and the broader v0.2 quota/self-service tenancy work.
