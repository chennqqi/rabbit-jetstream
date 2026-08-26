# rabbit-jetstream

[English](README.md) | [简体中文](README.zh-CN.md)

基于官方 NATS Server/JetStream 构建的生产级 RabbitMQ 替代发行版。

官方 `nats-server` 以 Git subtree 固定在 `upstream/nats-server/`，默认不修改源码。本仓库负责发行打包、部署、管理控制面、内嵌 Admin UI、运维工具和生产验证。配套 Go SDK 在独立仓库开发；`outlink/rabbit-jetstream-go` 仅为开发期符号链接。

> 首版通过原生 `rabbit-jetstream-go` SDK 提供 RabbitMQ 风格的优先级队列，不实现 AMQP 线协议，现有 RabbitMQ 客户端不能无修改直连。完整 RabbitMQ/AMQP 兼容属于未来 Roadmap。

## 快速开始

要求：Go 1.25（固定 toolchain `go1.25.13`）和启用 Linux 引擎的 Docker Desktop。

```bash
docker compose -f deploy/compose/standalone.yml up -d --build --wait
go run ./tools/rjsctl status
```

管理端点为 `http://127.0.0.1:8223`。Admin UI、Prometheus 指标、健康与就绪检查分别位于 `/admin/`、`/metrics`、`/healthz` 和 `/readyz`。

## 常用命令

```bash
go test ./...
make build
docker compose -f deploy/compose/cluster.yml up -d
go run ./tools/rjsctl queue validate examples/queues/orders.yaml
go run ./tools/rjsctl queue plan examples/queues/orders.yaml
RJS_ADMIN_TOKEN=secret go run ./tools/rjsctl queue apply --url http://127.0.0.1:8223 tests/fixtures/queue-basic.yaml
go run ./tools/rjsctl diagnostics collect --url http://127.0.0.1:8223 --output diagnostics.zip
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream --namespace messaging --create-namespace
```

配置使用 `RJS_*` 环境变量或命令行参数。请先阅读[部署指南](docs/deployment.zh-CN.md)，其中包含单节点开发、三节点集群和 Kubernetes 安装；日常巡检与故障处置见[运维手册](docs/operations.zh-CN.md)。其他资料包括[架构](docs/architecture.md)、[配置](docs/configuration.md)、[Queue Schema](docs/queue-schema.md)、[Native SDK 契约](docs/native-sdk-contract.md)、[测试策略](docs/testing.zh-CN.md)、[发布流程](docs/releasing.md)和 [Roadmap](docs/roadmap.zh-CN.md)。

## 仓库结构

```text
upstream/nats-server/   固定版本的官方 NATS Server subtree
management/             Go 管理 API 与控制面
admin-ui/               内嵌 Web 管理界面
tools/rjsctl/           运维 CLI
deploy/                 单机和集群部署资源
packaging/              发行镜像定义
tests/                  集成、故障、安全和性能测试
docs/                   架构与运维文档
outlink/                不纳入版本库的外部 SDK 链接
```
