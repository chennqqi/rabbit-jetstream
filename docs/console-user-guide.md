# Admin Console User Guide

[English](console-user-guide.md) | [简体中文](console-user-guide.zh-CN.md)

Task-oriented guide for operating the Rabbit JetStream admin console at `/admin/`. This guide explains what you can do in the UI and how to read it; architecture and server configuration live in [Operations](operations.md) and [Configuration](configuration.md).

## Signing in

- **Username / password**: local accounts are provisioned by an administrator (see [Local Auth](local-auth.md)). Sessions are short-lived and live only in page memory.
- **SSO**: if the deployment configures OIDC, a "Sign in with SSO" button appears on the login page.
- **Recovery or automation token**: operators can paste a bearer token instead of a password.
- **Session expiry**: when the credential expires the console keeps navigation visible and offers a **Sign in again** button; drafts and evidence stay in page memory until you clear the session.
- **Language**: the top-right button switches between English and 简体中文 immediately; the choice is remembered locally.

## Reading the console

**Navigation groups**: Resources (Overview, Queues, Streams, Consumers, Nodes), Operations (Audit, Diagnostics, Alerts), Governance (Bulk changes, Access and settings, Tenant access).

**Status badges** translate observations into priority:

| Badge | Meaning |
|---|---|
| Consistent / 一致 | The declared Queue's Stream matches the declaration; consumer configuration has not been checked. |
| Degraded / 降级 | Stream degraded or configuration mismatch — investigate. |
| Missing / 缺失 | The declared Stream does not exist. |
| Unavailable / 不可用 | The observation itself failed — counts are unknown, not zero. |

**Semantics to keep in mind**: counts are *observations*, not declarations; different pages read at different times and are not one atomic snapshot; unknown stays unknown (never shown as zero). Details are folded into each page's "About these numbers and states" note. The refresh indicator ("Refreshed … · auto every 10 s") shows data age; a stale banner appears when data is paused, failed, or older than 30 seconds.

**Plan revision values** are truncated hashes; the Copy button places the full revision on the clipboard.

## Queue lifecycle

### Find and inspect

Open **Queue list** to see declared Queues with observed state, stored messages, consumers, storage, and plan revision. Use "Queue name contains" to filter. Click a Queue to open its detail page: the summary shows observed Stream counts next to the declared configuration, with tabs for Configuration (raw declaration), Routing, Consumers, and Events.

### Create

1. Click **Create Queue** on the Queue list header.
2. Fill the name, subjects (one per line), replicas, storage, and limits. Templates can pre-fill fields.
3. Click **Prepare creation draft**, then **Preview changes**. The preview is advisory only — nothing is applied until you confirm.
4. Tick **I reviewed this preview and authorize applying this draft**, then **Apply reviewed draft**. Creation is audited.

### Update

On the Queue detail page, **Edit draft and preview**: edit the JSON draft, click **Preview changes**, review the diff, authorize, and **Apply reviewed draft**. Conflicting concurrent changes are rejected with an ETag error — reload and reapply your edit. Every apply is conditional and audited.

### Delete

**Review deletion impact** runs a preflight: ownership, message count, consumer count, and DLQ dependents. Deletion of a non-empty Queue requires the **force** checkbox, typing the exact Queue name, and the impact acknowledgment. If the outcome is uncertain (an error after dispatch), the console locks retry — record the evidence, check whether the Stream still exists, and decide manually. Download the deletion evidence JSON for the ticket.

## Streams and Consumers

**Stream list** and **Consumer list** are read-only observations, including resources created outside the console. Consumer detail shows pending/ack-pending counts for backlog triage; the Consumer diagnosis hint links the triage steps in [Operations](operations.md).

## Nodes and connections

**Node list** shows each configured monitoring endpoint and its read state (badge). Node detail exposes version, runtime, JetStream state, and per-source read evidence; connections are searchable per node with subscription details on demand.

## Audit

**Audit** pairs every write intent with its outcome. Filter by request ID, resource, actor, phase, action, outcome, and time window (RFC3339), then page through results. **Prepare export** downloads the filtered window as JSON for the ticket. One window scans at most 256 sequence numbers; narrow the time range to reach older records.

## Diagnostics

**Diagnostics** creates a metadata-only ZIP (no messages, no credentials) from the management process: manifest with SHA-256, at most one running job, jobs expire ten minutes after creation and do not survive restarts. Create → review the manifest → **Prepare download** → save the file.

## Operational alerts

**Operational alerts** projects the shipped Prometheus rules (read-only; the console never sends notifications — alerting to humans requires Alertmanager, see the [companion checklist](operations-companion.md)). States: **firing** (bad), **pending** (warn), **recovered** (observed firing-to-inactive transition in this process), **inactive** (not currently firing). "Incomplete alert coverage" warns when a shipped rule is missing from the backend.

## Bulk changes

**Bulk changes** applies a versioned update package Queue-by-Queue: upload, preview all targets, then confirm each item individually. There is no all-at-once submit by design.

## Access and settings

- **Verified session**: your identity, role, expiry, and permissions; **Clear local session** (inside the identity panel) signs out and discards in-memory drafts.
- **Preferences**: language and refresh interval are saved locally.
- **Tenant access** (platform admins): create local accounts, set tenant memberships and roles, disable or delete accounts.
- **Compatibility** (linked from Access and settings): build, embedded-UI identity, SDK contract, and server capabilities.

## Tenant switching

Accounts with several tenants switch via the identity panel's Active tenant selector; the URL carries the tenant (`/admin/tenants/<tenant>/…`). Switching discards the current tenant's drafts after confirmation.
