# Glossary

[English](glossary.md) | [简体中文](glossary.zh-CN.md)

Terms used across the console, API, CLI, and SDK. Where the console renders a Chinese label, it appears in brackets.

## Topology

- **Queue** [队列] — the declared, managed resource (Kubernetes-style document: `apiVersion`/`kind`/`metadata`/`spec`). Owns subjects, storage, retention, priorities, DLQ policy, and replica count. Managed only through the control plane (preview → confirm → conditional apply).
- **Stream** [流] — the JetStream stream a Queue's plan generates, named `RJSQ_<queue>`. Created/updated by the controller; read-only in the console. WorkQueue retention with delete-on-ack.
- **Consumer** [消费者] — the durable pull consumer(s) a Queue's plan generates, named `RJSQC_<queue>` (plus one per priority level when `maxPriority` is set). Console-visible but created only by the controller.
- **Subjects** — Queue bindings. Each Queue gets an ingress subject `rjs.q.<queue>.ingress`; the SDK publishes per-priority to `rjs.q.<queue>.p.<level>`.
- **Tenant** [租户] — an isolated NATS account managed by the control plane, routed via `/admin/tenants/<tenant>/`. Defined in `RJS_TENANTS_FILE`.
- **Binding** — an Exchange-style binding declaration translated into Queue subjects (no standalone Exchange resource exists).

## Declaration and change control

- **Declaration** [声明] — the stored Queue document in metadata KV, identified by revision hash.
- **Plan** [计划] — the deterministic JetStream resource plan derived from a declaration (`BuildPlan`); what the controller reconciles.
- **Revision** [修订] — the declaration hash shown in the console as "Plan revision"; changes on every applied update.
- **ETag / KV revision** [声明 ETag] — the numeric KV revision used for conditional writes (`If-Match`; creation uses `If-None-Match: *`). Numeric, distinct from the revision hash.
- **Preview** [预览] — a server-side, non-mutating dry run that classifies the change into update / no-op / recreate / reject.
- **recreate (plan class)** — a plan classified as destructive: Stream identity or storage semantics differ, messages will not survive. Requires the backup path or an accepted loss decision.
- **Blocked (plan class)** — apply would violate safety rules (name mismatch, ownership, missing force).
- **Force** — the delete confirmation required when a Stream still contains messages; never bypasses ownership protection.

## Storage and delivery

- **R3 / R5** — three- or five-replica JetStream cluster/stream profile. R3 is the qualified shape.
- **At-least-once** — delivery contract: applications must consume idempotently (stable message IDs, dedupe).
- **Ack / Nak / Term** — consumer settlement: positive ack (delete-on-ack in workqueue), negative ack (prompt redelivery), terminate (poison message, no redelivery).
- **Ack floor** [确认下限] — the lowest consumer sequence fully acknowledged; used in consumer diagnostics.
- **DLQ** [死信] — dead-letter Queue declared via `deadLetter.queue`. The controller moves exhausted messages (MaxDeliver advisory) at-least-once with `Nats-Msg-Id` dedup; per-queue counters are `rjs_dlq_queue_*`.
- **MaxDeliver** — delivery limit before a message dead-letters (default 5).

## Control plane and security

- **Management service** [管理服务] — the control plane binary (`rjs-management`), serving `/admin/`, `/api/v1/*`, and `/metrics`. Never in the message data path.
- **Controller** [控制器] — leader-elected component reconciling declarations toward actual JetStream state; leadership shown by `rjs_controller_leader`.
- **Metadata quorum** — the KV/metadata replication (R3) that must be leader+current before workloads start.
- **Tenant roles** — `operator` (write) and `auditor` (read-only) per tenant; platform admins manage accounts.
- **Sole owner** — the single accountable approver in the release-approval record.

## Release governance

- **Exact-candidate freeze** — no further merges into the tagged release tree.
- **Canary stages** — 1/10/25/50/100 % traffic evidence stages with zero-loss reconciliation.
- **Local-rc evidence** — packaging gate output (`scripts/release/local-rc.ps1`), 11 checked steps.
- **partial dependency** — a functional-parity area with accepted semantic differences (e.g. priority-queue), recorded with rationale in the release-approval record.
