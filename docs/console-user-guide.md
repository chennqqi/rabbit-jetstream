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

## Appendix: page reference

Field-level reference for every console page (labels in English / 简体中文).

**Login** (`/admin/`): Username 用户名 · Password 密码 · Sign in 登录 (primary) · Recovery or automation token 恢复或自动化 Token (collapsed; bearer-token sign-in for automation) · Sign in with SSO 使用企业 SSO 登录 (only when OIDC configured) · language switch top-right. Errors appear under the form (credentials-rejected, auth-disabled, expired).

**Top bar** (all pages): brand · flat identity `actor / role · tenant` · Verified identity 已验证身份 disclosure (actor, role, expiry, resource-read policy, Active tenant selector, Clear local session 清除本机会话) · Evidence indicator (retained drafts/deletions count; red when an outcome is unknown) · language switch.

**Overview 总览**: monitoring issues and coverage (configured endpoints, failed reads), management service and account (name, version, uptime, JetStream memory/storage/streams/consumers), declared Queue total, metric history chart (JetStream storage/messages; requires Prometheus). All cards fold their semantics into "About these numbers".

**Queue list 队列**: Create Queue (primary) · search filter + sort + page size + Refresh · table: Queue, Observed state (badge), Stored messages, Consumers, Declared storage, Requested replicas, Plan revision (Copy) · pagination. Row click opens the detail.

**Queue detail** tabs: 摘要 Summary (Plan revision, ETag, declared read time; observed Stream card with Stored messages 存储消息数 / 存储字节 / Consumers / retention; metrics Pending 待投递, Ack pending 待确认 with attention shading when > 0) · 配置 Configuration (declaration JSON, Plan raw data, DLQ diagnostics 诊断, Export) · 路由 Routing (subject/routing probe) · 消费者 Consumers (filter toolbar + table) · 事件 Events. Header actions: Refresh Queue 页面 · Edit draft and preview 编辑草稿与预览 · Review deletion 审阅删除影响.

**Create Queue 创建 Queue**: form (name, subjects one-per-line, requested replicas 副本数, storage 存储类型, max stored messages) · template selector · JSON draft · Prepare creation draft 准备创建草稿 → Preview changes 预览变更 → authorization checkbox → Apply reviewed draft 应用已审阅草稿. Import single/plural JSON files. Parked drafts and creation history are retained in-page.

**Delete page 删除**: preflight (Stream, ETag, ownership 所有权, Messages, Consumers, Default deletion blocked 默认删除被阻止, Dead-letter dependents 死信依赖方) · Force checkbox + exact-name box + impact acknowledgment · Delete this Queue (destructive, disabled until complete) · Deletion evidence JSON download · uncertain outcomes lock retry.

**Stream list/detail**: observed streams incl. externally created; detail = observed configuration (subjects, storage, replicas, retention, discard), observed state (stored messages, bytes, consumers, first/last sequence), per-consumer table. Read-only.

**Consumer detail**: precise consumer observation (mode, durable, filter, pending 待投递, ack pending 待确认, redelivered, waiting pull requests) + diagnosis hint.

**Node list/detail**: monitoring endpoints, read-state badge, version/runtime/connections, JetStream metrics, source read evidence (varz/routez/jsz), View node connections 连接列表 with per-connection subscriptions.

**Audit 审计**: filters (Request ID 请求 ID, Resource 资源, Actor 操作者, Phase 阶段 intent/outcome, Action 操作, Outcome 记录结果, From/Until 时间) · scanned-window table with evidence details · Prepare export 准备导出 JSON.

**Diagnostics 诊断包**: Create metadata bundle → job manifest (sources/files/bytes) → Prepare download → Save diagnostic ZIP.

**Operational alerts 运维告警**: rule table (Rule 规则, Severity 严重度, State 状态 badge, Threshold expression 阈值表达式, For duration), missing-rules warning, Prometheus console link.

**Bulk changes 批量变更**: upload Queue update package → preview all targets → per-item confirm; no all-at-once submit by design.

**Access and settings 访问与设置**: verified session card (identity, role, expiry, read policy) · permissions · server-side capabilities · language/refresh preferences · compatibility link · local-account management under Tenant access 租户访问管理 (create/edit/disable/delete accounts, tenant memberships and roles, platform admin flag).

**Admin UI conventions**: destructive actions always require explicit confirmation dialogs; drafts/evidence live only in page memory; every page auto-refreshes (interval adjustable in settings) with visible data age; keyboard: skip-to-content link, visible focus rings, Esc closes disclosures/menus.
