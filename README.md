# rabbit-jetstream

以官方 NATS Server/JetStream 为核心构建的生产级 RabbitMQ 替代发行版。

官方 `nats-server` 以 Git subtree 固定在 `upstream/nats-server/`，默认不做源码修改。本仓库主体负责 Server 发行、部署、管理后端、Admin UI、运维工具和生产验证。配套 Go SDK 位于独立仓库；`outlink/rabbit-jetstream-go` 只是开发期符号链接，不参与本仓库构建。

> 当前是项目骨架，不宣称兼容 AMQP 或 RabbitMQ 客户端。协议兼容属于后续 Roadmap。

## 快速开始

要求：Go 1.24+；本地运行 NATS 时须启用 JetStream。

```bash
docker compose -f deploy/compose/standalone.yml up -d
go run ./management/cmd/rjs-management
go run ./tools/rjsctl status
```

默认管理端点为 `http://127.0.0.1:8223`：

- `/admin/`：内嵌只读 Admin UI；
- `GET /metrics`：Prometheus 运行指标；
- `GET /healthz`：进程存活；
- `GET /readyz`：JetStream 可用；
- `GET /api/v1/info`：服务及 JetStream 账户摘要。
- `GET /api/v1/controller`：controller 选主和最近 reconcile 状态。

## 常用命令

```bash
go test ./...
make build
docker compose -f deploy/compose/cluster.yml up -d
docker compose -f deploy/compose/standalone.yml --profile observability up -d
make test-linux-smoke
make test-linux-fault
go run ./tools/rjsctl queue validate examples/queues/orders.yaml
go run ./tools/rjsctl queue plan examples/queues/orders.yaml
go run ./tools/rjsctl queue reconcile --url http://127.0.0.1:8223 tests/fixtures/queue-basic.yaml
RJS_ADMIN_TOKEN=secret go run ./tools/rjsctl queue apply --url http://127.0.0.1:8223 tests/fixtures/queue-basic.yaml
RJS_ADMIN_TOKEN=secret go run ./tools/rjsctl queue delete --url http://127.0.0.1:8223 --confirm basic basic
go run ./tools/rjsctl queue list --url http://127.0.0.1:8223
go run ./tools/rjsctl diagnostics collect --url http://127.0.0.1:8223 --output diagnostics.zip
docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .
```

配置通过 `RJS_*` 环境变量或启动参数注入，详见 [配置说明](docs/configuration.md)。总体设计、[Queue Schema](docs/queue-schema.md)、[管理 API](docs/management-api.md)、[可观测性](docs/observability.md)、[诊断包](docs/diagnostics.md)、[备份恢复](docs/backup-restore.md)、上游维护、测试门禁和分期计划分别见 [架构设计](docs/architecture.md)、[上游管理](docs/upstream.md)、[测试策略](docs/testing.md) 与 [Roadmap](docs/roadmap.md)。

## 仓库结构

```text
upstream/nats-server/   固定版本的官方 NATS Server Git subtree
management/             Go 管理 API 与控制面
admin-ui/               内嵌 Web 管理界面及静态资源
tools/rjsctl/           运维 CLI
deploy/                 单机和集群编排
packaging/              NATS Server 与管理面的发行镜像
tests/                  集成、故障、兼容性、性能与长稳测试
docs/                   架构、配置、测试策略和 Roadmap
outlink/                外部客户端 SDK 链接（不纳入版本库）
```
