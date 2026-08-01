# Rolling Upgrade and Rollback

Never upgrade more than one NATS replica at a time. A release candidate must be exercised with the exact baseline and target images before production:

```powershell
.\tests\integration\rolling-upgrade.ps1 `
  -BaselineNATSImage registry/rjs-nats:1.2.0 `
  -TargetNATSImage registry/rjs-nats:1.3.0 `
  -BaselineManagementImage registry/rjs-management:1.2.0 `
  -TargetManagementImage registry/rjs-management:1.3.0
```

The drill creates a three-replica Stream and durable Consumer, replaces NATS nodes in order, publishes after each replacement, upgrades management, then rolls everything back in reverse order. It fails unless every accepted message, Consumer metadata, all current replicas, management readiness, and node health survive.

## Production procedure

1. Verify backups, free disk headroom, three healthy nodes, current Stream replicas, and no active data migration.
2. Upgrade one follower. Wait for its health probe and every affected replica to report `current` before proceeding.
3. Repeat for the other follower, then the leader. Never continue while the cluster lacks quorum or replicas lag.
4. Roll management replicas with `maxUnavailable: 0`; verify readiness, controller leadership, metrics, audit writes, and an idempotent Queue apply.
5. Observe redelivery, API errors, route reconnects, storage growth, and publish/consume latency for the release soak window.

Rollback uses the same process in reverse with the preserved baseline images. Stop immediately and roll back on lost quorum, non-current replicas that do not converge, storage-format incompatibility, sustained error-rate breach, or message/metadata mismatch. Do not restore a backup over a live divergent cluster; isolate it and follow the disaster-recovery procedure.

Pin immutable image digests in production and record old/new NATS versions, configuration hashes, timings, and test output in release evidence.
