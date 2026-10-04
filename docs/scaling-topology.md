# Scaling and Topology Changes

[English](scaling-topology.md) | [简体中文](scaling-topology.zh-CN.md)

Operational procedures for changing the size or shape of a Rabbit JetStream deployment: Queue replica changes, NATS cluster node operations, management replica changes, and R-level migrations. This document complements [Deployment](deployment.md), [Kubernetes](kubernetes.md), and [Capacity Planning](capacity-planning.md).

## Before any topology change

1. Verify a recent backup: `rjsctl backup verify --input <dir>`; a backup without a successful restore drill is not considered recoverable (see [Backup and Restore](backup-restore.md)).
2. Review [Capacity Planning](capacity-planning.md): storage headroom, drain rates, and alert thresholds must be re-checked after replica, retention, or node changes.
3. Freeze unrelated topology changes for the maintenance window ([Incident Triage](operations.md) rule 1 applies equally to planned changes): one member at a time, never restart multiple NATS members simultaneously.
4. Capture the pre-change state: `rjsctl status`, `/api/v1/cluster`, `kubectl -n <ns> get pods`, and a diagnostics bundle (`rjsctl diagnostics collect`).

## Queue replica changes (1 ↔ 3 ↔ 5)

Queue replicas are part of the declarative Queue document (`spec.replicas`) and are changed through the preview-confirm control plane, never by touching JetStream directly.

1. Edit the Queue document (Admin UI "Edit draft and preview", `rjsctl queue diff`, or API `POST /api/v1/queues/{queue}/preview`).
2. Read the preview plan carefully: **if the plan reports a `recreate` operation, the change is destructive** — the Stream identity or storage semantics differ and messages will not survive. Recreate-class changes require the backup path (below) or an accepted data-loss decision recorded in a ticket.
3. Apply only safe (update-class) plans with the conditional write (`If-Match`), then verify: `/api/v1/queues/{queue}` shows the new replica count and every replica `current`, and the Queue's alert state stays quiet.

Raising `spec.replicas` on a populated Queue may be plan-classified as `recreate` because JetStream Streams cannot change their replica factor in place. For R-level changes with data preservation, use the migration path below.

## NATS cluster node operations (Kubernetes)

- **Scale up**: `kubectl -n <ns> scale statefulset <release>-rabbit-jetstream-nats --replicas=<n>`. New Pods join the NATS cluster and JetStream metadata quorum, but **existing Streams do not rebalance onto new Pods automatically** — replica placement is set at Stream creation. Existing Queues keep their current redundancy until their declarations change (see above).
- **Scale down**: never delete a Pod that holds a JetStream peer without first removing its peer membership. Drain the peer through the NATS CLI (`nats server cluster peer step-down` / `peer remove`, available in the operator image), confirm `jsz` shows no missing replicas for any Stream, then scale the StatefulSet. Directly deleting the Pod leaves a lost peer that blocks quorum-sensitive operations.
- PVCs are retained on scale-down by design; decide retention per [Kubernetes](kubernetes.md).

## NATS cluster node operations (bare metal / Compose)

Bare-metal nodes are configured with per-node stanzas (`deploy/nats/cluster-N.conf`): same `cluster.name`, one distinct `server_name`, client/monitor/cluster ports, and the full route list on every node.

- **Add a node**: provision storage, copy the stanza template with a new `server_name` and ports, append the new route to every existing node's `routes` list, rolling-restart existing members one at a time (quorum retained), then start the new node. Confirm the metadata leader reports all peers current and caught up (the same warm-up rule [baremetal-qualification.md](baremetal-qualification.md) enforces) before declaring the change complete.
- **Remove a node**: step down its JetStream peers and remove the peer with the NATS CLI so the cluster forgets it; update every remaining node's `routes` with a rolling restart; only then stop the old process. Never delete a node's data directory to make a failed member start ([Incident Triage](operations.md) rule 5).

## Management replicas

The management service is stateless apart from its NATS connection: scaling is safe in both directions. Helm default is two replicas behind a Service; bare-metal deployments run one process per host and should put replicas behind a load balancer ([Deployment](deployment.md)). After scaling, verify `readyz` on every replica and that `/api/v1/cluster` reports the expected profile.

## R-level migration (R3 → R5, or recovery at a different R)

JetStream replica factor is fixed at Stream creation, so an R-level change with preserved data is a backup → restore cycle:

1. `rjsctl backup create --output <dir>` against the current cluster and `rjsctl backup verify --input <dir>`.
2. Provision the target cluster at the new R (three or five JetStream nodes).
3. `rjsctl backup restore --input <dir> --confirm RESTORE --replicas 5` — restore requires an empty account with writers and the controller stopped, and is not transactional across Streams ([Backup and Restore](backup-restore.md), [remaining-release-work](remaining-release-work.md)).
4. Reconcile with `rjsctl migrate capture` + `reconcile` or the canary reconciler, then re-point clients.

## Verification after every change

1. `kubectl rollout status` (K8s) or process/health checks (bare metal) for every touched member.
2. `/api/v1/cluster` and `jsz` show all replicas `current` with no missing peer.
3. The canary reconciler (see `tests/soak/`): publish a short burst, confirm zero missing/corrupt and management 5xx = 0.
4. Watch the shipped alerts for one evaluation cycle (JetStream availability, controller stall, node availability) and confirm a quiet state.
5. Record the change window, digests, and evidence per [Escalation Evidence](operations.md) expectations.
