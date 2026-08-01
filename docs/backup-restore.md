# Backup and Restore

`rjsctl backup` orchestrates JetStream's network snapshot protocol through the pinned `nats` CLI included in the operator image. It backs up every Stream in the current account, including hidden KV/Object Store Streams, message data and durable Consumer state.

## Create and Verify

```bash
docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .
docker run --rm --network <nats-network> -v /secure/backups:/backup \
  rabbit-jetstream/operator:local backup create \
  --server nats://nats:4222 --output /backup/2026-08-01

docker run --rm -v /secure/backups:/backup:ro \
  rabbit-jetstream/operator:local backup verify --input /backup/2026-08-01
```

Creation checks every Stream before snapshotting, includes Consumers, writes into a staging directory and only publishes the final directory after all snapshots succeed. Existing output paths are never overwritten. `manifest.json` records tool versions, Stream names, file sizes and SHA-256 digests.

## Restore Drill

Stop management/controller instances and publishers, then restore into an empty JetStream account:

```bash
docker run --rm --network <nats-network> -v /secure/backups:/backup:ro \
  rabbit-jetstream/operator:local backup restore \
  --server nats://nats:4222 --input /backup/2026-08-01 \
  --confirm RESTORE --replicas 3
```

Restore verifies every file before contacting NATS and refuses to proceed if any destination Stream already exists. It is not transactional across Streams: if a later restore fails, earlier Streams can already exist and must be assessed before retrying. Use `--replicas 1`, `3`, or `5` when the recovery cluster differs from the source.

## Production Policy

- Stream snapshots are sequential, not an account-wide atomic snapshot. Quiesce publishers and controllers when cross-Stream consistency is required.
- Backup data contains message payloads and may contain application secrets. Encrypt it, restrict access, copy it off-cluster and apply retention policy.
- SHA-256 detects corruption but does not prove authenticity. Protect or externally sign `manifest.json` in untrusted storage.
- Never put credentials in `--server`; the CLI rejects URL userinfo. Mount a credentials file or inject standard `NATS_*` authentication environment variables.
- Run `tests/integration/backup-restore.ps1` regularly against disposable infrastructure. A stored backup without a successful restore drill is not considered recoverable.

The automated drill creates a Queue, publishes three messages, snapshots its data plus the metadata KV Stream, destroys the JetStream volume, restores into a clean node, and verifies the Queue declaration, durable Consumer and message count.
