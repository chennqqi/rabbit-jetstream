# Kubernetes and Helm

The chart under `deploy/helm/rabbit-jetstream` requires Kubernetes 1.28 or newer and deploys the server distribution as a three-node JetStream StatefulSet and a two-replica management Deployment.

## Install

Publish the repository images, resolve their immutable registry digests, then provide those digests and a production StorageClass. When `digest` is non-empty it takes precedence over `tag`:

```bash
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream \
  --namespace messaging --create-namespace \
  --set nats.image.repository=registry.example/rabbit-jetstream/nats-server \
  --set-string nats.image.digest=sha256:<64-hex-digest> \
  --set management.image.repository=registry.example/rabbit-jetstream/management \
  --set-string management.image.digest=sha256:<64-hex-digest> \
  --set operator.image.repository=registry.example/rabbit-jetstream/operator \
  --set-string operator.image.digest=sha256:<64-hex-digest> \
  --set nats.storage.storageClass=fast-retain \
  --set nats.tls.enabled=true \
  --set nats.tls.serverSecret=rjs-nats-server-tls \
  --set nats.tls.clientSecret=rjs-management-nats-tls \
  --set networkPolicy.egress.enabled=true \
  --set production.enabled=true
helm test rabbit-jetstream --namespace messaging
```

Defaults create a persistent 3-node cluster, two management replicas, per-pod PVCs, readiness/liveness probes, topology spreading, PodDisruptionBudgets and namespace-scoped NetworkPolicies. StatefulSet deletion and scale-down both retain PVCs explicitly; the StorageClass should independently retain the underlying PV. The chart generates NATS credentials and an admin token once and preserves the password, bcrypt hash, and token byte-for-byte across upgrades using `lookup`, avoiding credential changes and GitOps drift from a newly salted hash.

For GitOps or disaster recovery, create a Secret with `nats-username`, `nats-password`, `nats-password-bcrypt`, and `admin-token`, then set `auth.existingSecret`. Optional comma-separated `admin-tokens` and `audit-tokens` keys enable overlapping operator rotation and read-only auditors. The bcrypt value must hash `nats-password`; the NATS pods never receive the plaintext password. Back up this Secret separately; JetStream snapshots do not contain Kubernetes Secrets. See [Management Credential Rotation](credential-rotation.md) before changing live credentials.

Production clusters should enable mutual TLS for NATS client and route traffic. Create a server Secret and a distinct management-client Secret, each containing `ca.crt`, `tls.crt`, and `tls.key`. The server certificate SANs must cover the client Service and every StatefulSet/headless-Service DNS name. Install with `--set nats.tls.enabled=true --set nats.tls.serverSecret=rjs-nats-server-tls --set nats.tls.clientSecret=rjs-management-nats-tls`. Override `nats.tls.serverName` only when the certificate uses a different stable DNS name. The chart mounts each Secret only into its intended workload and never generates private keys.

`production.enabled=true` is the fail-closed deployment profile. Helm then requires three or five NATS replicas, at least two management replicas, a named StorageClass, distinct server/client mTLS Secrets, digest-pinned NATS/management/operator images, ingress and egress NetworkPolicies, PodDisruptionBudgets, a private `ClusterIP` management Service, and secure OIDC/telemetry transport. NATS and management replicas must span distinct hostname domains; insufficient nodes leave Pods pending instead of silently removing fault tolerance. An enabled Ingress must also declare TLS. Keep this switch enabled in GitOps values so an unsafe override fails during rendering rather than reaching the cluster.

Production egress isolation is enabled with `networkPolicy.egress.enabled=true`. The built-in rules permit DNS, NATS route traffic, and management access to NATS client/monitoring ports only. If OIDC or OTLP is configured, add narrowly scoped Kubernetes NetworkPolicy rules under `networkPolicy.egress.additionalRules`; the production profile rejects those endpoints while the list is empty. Adjust the configurable DNS namespace/Pod selectors if the cluster DNS labels differ from the Kubernetes defaults.

The `operator.image` value records the digest-pinned, on-demand `rjsctl` image shipped with the same release. The chart deliberately does not create a permanent operator Pod; run that image only for an approved administration, backup, restore, diagnostic, or migration operation.

## Production Checklist

- Use immutable image tags/digests and a `Retain` StorageClass; the `local` image defaults are for development only.
- Keep `nats.replicaCount` at 3 or 5 and provide at least that many schedulable nodes with distinct `kubernetes.io/hostname` values. The chart uses `minDomains` plus `DoNotSchedule`, so it fails closed instead of co-locating durable replicas. A one-node chart is allowed only for development/recovery.
- Keep management and NATS client services private. If Ingress is enabled, add authentication at the ingress and TLS; read-only management endpoints are otherwise unauthenticated.
- With both Ingress and NetworkPolicy enabled, also set `networkPolicy.ingress.enabled=true` and provide non-empty `namespaceSelector` and `podSelector` maps matching only the ingress controller. The chart rejects an unrestricted cross-namespace path instead of opening port 8223 globally.
- Keep `networkPolicy.egress.enabled=true`; explicitly allow only approved IdP and telemetry Collector peers in `networkPolicy.egress.additionalRules`.
- Enable `nats.tls` before crossing untrusted networks. The profile requires verified client certificates on both client and cluster-route ports; distribute dedicated client certificates to external publishers and consumers.
- Enable `serviceMonitor` only when the Prometheus Operator CRD is installed.
- Run the backup/restore drill and record RPO/RTO before production cutover.

## Rolling Changes

The StatefulSet uses ordered rolling updates and the NATS PDB retains quorum during voluntary disruption. Upgrade one NATS pod at a time, wait for JetStream replicas to become current, and only then continue. Management uses `maxUnavailable: 0`. Kubernetes primitives reduce risk but do not replace the release-specific rolling upgrade/rollback drill required by `docs/testing.md`.

Validate all chart modes without a Kubernetes cluster:

```powershell
.\tests\deployment\helm.ps1
```
