# Deployment Guide

[English](deployment.md) | [简体中文](deployment.zh-CN.md)

Use immutable release artifacts and verify `SHA256SUMS` before production installation. Development Compose files build from source; production should set digest-pinned `RJS_NATS_IMAGE` and `RJS_MANAGEMENT_IMAGE` values. See [Configuration](configuration.md) for every `RJS_*` setting.

## Single-node Development

```bash
export RJS_ADMIN_TOKEN='development-only-secret'
docker compose -f deploy/compose/standalone.yml up -d --build --wait
curl -fsS http://127.0.0.1:8223/readyz
docker compose -f deploy/compose/standalone.yml down -v
```

Open `http://127.0.0.1:8223/admin/`. NATS client, monitoring, and management ports are `4222`, `8222`, and `8223`. This mode uses one file-backed JetStream node and one management process; it has no quorum and is not production-safe. Removing `-v` preserves the named data volume.

## Three-node Compose Cluster

```bash
export RJS_ADMIN_TOKEN='replace-from-a-secret-manager'
docker compose -f deploy/compose/cluster.yml config
docker compose -f deploy/compose/cluster.yml up -d --build --wait
curl -fsS http://127.0.0.1:8223/api/v1/cluster
docker compose -f deploy/compose/cluster.yml ps
```

The profile creates three JetStream volumes and metadata replication factor three. Compose demonstrates quorum and failure behavior on one Docker host; it does not protect against host, rack, or storage failure. For production, configure NKeys/credentials and TLS, isolate monitoring ports, use persistent disks with measured latency, and run management replicas behind a load balancer.

## Kubernetes

Requirements: Kubernetes 1.28+, Helm 3, a default or explicitly selected `StorageClass`, three schedulable failure domains, and pre-created TLS/auth Secrets.

```bash
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream \
  --namespace messaging --create-namespace \
  --set auth.existingSecret=rabbit-jetstream-auth \
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
  --set networkPolicy.natsClients.allowSameNamespace=false \
  --set networkPolicy.egress.enabled=true \
  --set production.enabled=true

kubectl -n messaging rollout status statefulset/rabbit-jetstream-nats
kubectl -n messaging rollout status deployment/rabbit-jetstream-management
helm test rabbit-jetstream --namespace messaging
```

The production profile fails closed unless required security, persistence, image-digest, spreading, and NetworkPolicy inputs are present. Validate changes with `make test-helm`; native Linux qualification uses `make test-kubernetes`. Review [Kubernetes and Helm](kubernetes.md) before exposing NATS or Admin UI through an ingress.

## Post-install Acceptance

Confirm readiness, three available nodes, current replicas, controller leadership, metrics scraping, audit writes, Admin UI access, a Queue create/delete cycle, backup creation/verification, and rollback artifacts. Do not direct production publishers to the cluster until these checks pass.
