# 部署指南

[English](deployment.md) | [简体中文](deployment.zh-CN.md)

生产安装必须使用不可变发布制品并验证 `SHA256SUMS`。开发 Compose 默认从源码构建；生产环境应设置带 digest 的 `RJS_NATS_IMAGE` 与 `RJS_MANAGEMENT_IMAGE`。全部 `RJS_*` 参数见[配置说明](configuration.md)。

## 单节点开发模式

```bash
export RJS_ADMIN_TOKEN='development-only-secret'
docker compose -f deploy/compose/standalone.yml up -d --build --wait
curl -fsS http://127.0.0.1:8223/readyz
docker compose -f deploy/compose/standalone.yml down -v
```

Admin UI 位于 `http://127.0.0.1:8223/admin/`，NATS 客户端、监控和管理端口分别是 `4222`、`8222`、`8223`。该模式只有一个文件存储 JetStream 节点和一个管理进程，没有仲裁能力，禁止用于生产。停止时不带 `-v` 可保留命名数据卷。

## 三节点 Compose 集群

```bash
export RJS_ADMIN_TOKEN='replace-from-a-secret-manager'
docker compose -f deploy/compose/cluster.yml config
docker compose -f deploy/compose/cluster.yml up -d --build --wait
curl -fsS http://127.0.0.1:8223/api/v1/cluster
docker compose -f deploy/compose/cluster.yml ps
```

该配置创建三个 JetStream 数据卷，元数据副本数为三。它只能在单台 Docker 主机上演示仲裁和故障行为，无法抵御主机、机架或存储故障。生产环境必须配置 NKeys/credentials 与 TLS、隔离监控端口、使用经过延迟验证的持久盘，并将多个管理副本置于负载均衡器后。

## Kubernetes

要求 Kubernetes 1.28+、Helm 3、默认或显式指定的 `StorageClass`、三个可调度故障域，以及预创建的 TLS/认证 Secret。

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

生产 profile 在安全、持久化、镜像 digest、拓扑分散或 NetworkPolicy 参数缺失时会拒绝安装。使用 `make test-helm` 验证模板，原生 Linux 上使用 `make test-kubernetes` 完成安装资格验证。通过 Ingress 暴露 NATS 或 Admin UI 前阅读 [Kubernetes 与 Helm](kubernetes.md)。

## 上线验收

必须确认 readiness、三个可用节点、所有副本 current、controller leader、指标采集、审计写入、Admin UI、Queue 创建/删除、备份校验及回滚制品。全部通过前不得切入生产发布者。
