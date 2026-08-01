# Kubernetes and Helm

The chart under `deploy/helm/rabbit-jetstream` deploys the server distribution as a three-node JetStream StatefulSet and a two-replica management Deployment.

## Install

Publish the repository images with immutable tags, then provide those tags and a production StorageClass:

```bash
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream \
  --namespace messaging --create-namespace \
  --set nats.image.repository=registry.example/rabbit-jetstream/nats-server \
  --set nats.image.tag=2.14.1-rjs.1 \
  --set management.image.repository=registry.example/rabbit-jetstream/management \
  --set management.image.tag=0.1.0 \
  --set nats.storage.storageClass=fast-retain
helm test rabbit-jetstream --namespace messaging
```

Defaults create a persistent 3-node cluster, two management replicas, per-pod PVCs, readiness/liveness probes, topology spreading, PodDisruptionBudgets and namespace-scoped NetworkPolicies. The chart generates NATS credentials and an admin token once and preserves them across upgrades using `lookup`.

For GitOps or disaster recovery, create a Secret with `nats-username`, `nats-password`, `nats-password-bcrypt`, and `admin-token`, then set `auth.existingSecret`. Optional comma-separated `admin-tokens` and `audit-tokens` keys enable overlapping operator rotation and read-only auditors. The bcrypt value must hash `nats-password`; the NATS pods never receive the plaintext password. Back up this Secret separately; JetStream snapshots do not contain Kubernetes Secrets. See [Management Credential Rotation](credential-rotation.md) before changing live credentials.

## Production Checklist

- Use immutable image tags/digests and a `Retain` StorageClass; the `local` image defaults are for development only.
- Keep `nats.replicaCount` at 3 or 5 and ensure nodes span failure domains. A one-node chart is allowed only for development/recovery.
- Keep management and NATS client services private. If Ingress is enabled, add authentication at the ingress and TLS; read-only management endpoints are otherwise unauthenticated.
- NATS username/password and NetworkPolicy provide a baseline, not transport encryption. Use a service mesh or a future chart TLS profile before crossing untrusted networks.
- Enable `serviceMonitor` only when the Prometheus Operator CRD is installed.
- Run the backup/restore drill and record RPO/RTO before production cutover.

## Rolling Changes

The StatefulSet uses ordered rolling updates and the NATS PDB retains quorum during voluntary disruption. Upgrade one NATS pod at a time, wait for JetStream replicas to become current, and only then continue. Management uses `maxUnavailable: 0`. Kubernetes primitives reduce risk but do not replace the release-specific rolling upgrade/rollback drill required by `docs/testing.md`.

Validate all chart modes without a Kubernetes cluster:

```powershell
.\tests\deployment\helm.ps1
```
