# Operations Runbook

[English](operations.md) | [简体中文](operations.zh-CN.md)

## Routine Checks

```bash
rjsctl status --url http://management:8223
rjsctl diagnostics collect --url http://management:8223 --output diagnostics.zip
curl -fsS http://management:8223/readyz
curl -fsS http://management:8223/metrics
```

Check cluster availability, JetStream storage headroom, non-current replicas, Queue backlog, redelivery, DLQ failures, controller leadership, route reconnects, API errors, and certificate expiry. Review audit events for every Queue mutation. Alert before storage reaches the thresholds in [Capacity Planning](capacity-planning.md).

## Incident Triage

1. Freeze topology changes; do not restart multiple NATS members.
2. Record readiness, node/cluster/stream/controller state and collect a diagnostic bundle.
3. Determine whether the fault is control-plane only or affects publish/consume acknowledgements.
4. Preserve logs, metrics, audit events, image digests, configuration hashes, and timestamps.
5. Restore quorum one member at a time. Never delete volumes to make a failed member start.
6. If integrity is uncertain, isolate the cluster and follow [Backup and Restore](backup-restore.md).

Admin UI is a convenience interface; API, CLI, metrics, and audit remain authoritative during incidents.

## Backup and Recovery

Create account snapshots on a defined schedule, encrypt and copy them off-cluster, then run periodic restore drills. Verify every backup manifest before retention or restore decisions. Back up Kubernetes authentication/TLS Secrets separately because JetStream snapshots do not contain them. A backup without a successful restore drill is not considered recoverable.

## Upgrade and Rollback

Before change: verify a recent backup, disk headroom, current replicas, immutable old/new images, and tested rollback commands. Upgrade one NATS follower at a time, wait for every replica to become current, then upgrade the leader and management replicas. Stop and roll back on quorum loss, persistent replica lag, message mismatch, or SLI breach. Follow [Rolling Upgrade and Rollback](upgrade-rollback.md).

## Credential Rotation

Use overlapping operator/auditor tokens or OIDC signing keys: add the new credential, verify both, migrate clients, remove the old credential, then verify rejection and audit evidence. Never place credentials in diagnostic archives, command history, Git, or NATS URLs. See [Credential Rotation](credential-rotation.md) and [OIDC](oidc.md).

## Escalation Evidence

Attach the diagnostic ZIP, release/tag and image digests, incident window in UTC, affected Queues, client acknowledgement errors, node events, storage status, and actions already attempted. Do not attach message payloads unless explicitly approved and encrypted.
