# Documentation Map

[English](README.md) | [简体中文](README.zh-CN.md)

Start here. Documents are grouped by audience; bilingual counterparts use the `.zh-CN.md` suffix.

## I use the admin console (operators, on-call)

| Document | Purpose |
|---|---|
| [Console User Guide](console-user-guide.md) | Task-oriented manual: sign-in, every page, badges, Queue lifecycle, audit, diagnostics. |
| [Troubleshooting](troubleshooting.md) | Symptom → cause → action for console/API/SDK/CLI errors. |
| [Glossary](glossary.md) | Terms used everywhere (Queue, Stream, Plan revision, DLQ, tenants, …). |
| [Operational Alerts](webui-operational-alerts.md) | The alerts page's rules and states. |
| [Consumer Diagnosis](webui-consumer-diagnosis.md) / [DLQ Diagnostics](webui-dlq-diagnostics.md) | Backlog and dead-letter triage pages. |

## I operate the deployment (platform / SRE)

| Document | Purpose |
|---|---|
| [Deployment](deployment.md) | Single-node, Compose cluster, Kubernetes install. |
| [Kubernetes](kubernetes.md) | Chart details, production checklist, rolling changes. |
| [Scaling and Topology Changes](scaling-topology.md) | Queue replicas, node add/remove, management replicas, R-level migration. |
| [Operations](operations.md) + [Companion Checklist](operations-companion.md) | Routine checks, incident triage, backup, upgrade/rollback, alert-to-human, runbooks. |
| [Observability](observability.md) | Metrics, Prometheus, shipped alerts. |
| [Configuration](configuration.md) | Every `RJS_*` setting. |
| [Capacity Planning](capacity-planning.md) | Sizing, thresholds, drain tests. |
| [Backup and Restore](backup-restore.md) / [Upgrade-Rollback](upgrade-rollback.md) / [Credential Rotation](credential-rotation.md) / [Diagnostics](diagnostics.md) | Deep-dives for the operations.md sections. |
| [Local Auth](local-auth.md) / [OIDC](oidc.md) / [Multi-tenancy](multi-tenancy.md) | Identity and isolation. |

## I develop applications against the SDK

| Document | Purpose |
|---|---|
| [SDK Quickstart](sdk-quickstart.md) | First priority-queue app end to end (declare → publish → consume). |
| SDK repo `outlink/rabbit-jetstream-go` | README (bilingual), `docs/api-v0.1.md` (API contract), `examples/priority`. |
| [Native SDK Contract](native-sdk-contract.md) / [API Versioning](api-versioning.md) | Wire-level contract and stability rules. |
| [Client Migration](client-migration.md) | Moving clients from RabbitMQ. |

## I develop the product itself

| Document | Purpose |
|---|---|
| [Architecture](architecture.md) / [Queue Mapping](queue-mapping.md) | How the system works; RabbitMQ concept mapping. |
| [Management API](management-api.md) + `api/openapi.yaml` | Endpoint reference; codegen via [openapi-codegen](openapi-codegen.md). |
| [Testing](testing.md) / [Performance Testing](performance-testing.md) | Gate inventory and workloads. |
| WebUI docs (`docs/webui-*.md`) | Console feature specs and qualification records. |
| [Release Notes](releases/) + [Remaining Release Work](remaining-release-work.md) | Per-candidate evidence and open gates. |

## Governance and review records

[Release approval template](release-approval.template.json), signed records under `docs/releases/` (soak, cluster, canary/node-failure/rollback evidence for rc.3), [ops review](ops-review.md), [console design review](../admin-ui/design-review.md), and the [improvement plan](review-improvement-plan.md).

## AI agents

Skills for autonomous operation live in `skills/` (`rjsctl-operations`, `rjs-incident-response`, `rjs-release-drills`, `rjs-console-automation`); repository working conventions are in [AGENTS.md](../AGENTS.md).
