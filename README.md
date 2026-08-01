# rabbit-jetstream

以官方 NATS Server/JetStream 为核心构建的生产级 RabbitMQ 替代发行版。

官方 `nats-server` 以 Git subtree 固定在 `upstream/nats-server/`，默认不做源码修改。本仓库主体负责 Server 发行、部署、管理后端、Admin UI、运维工具和生产验证。配套 Go SDK 位于独立仓库；`outlink/rabbit-jetstream-go` 只是开发期符号链接，不参与本仓库构建。

> 当前版本不兼容 AMQP 线协议，现有 RabbitMQ 客户端不能无改动直连。准确边界见[客户端兼容矩阵](docs/client-migration.md)，协议网关属于后续 Roadmap。

## 快速开始

要求：Go 1.24+；本地运行 NATS 时须启用 JetStream。

```bash
docker compose -f deploy/compose/standalone.yml up -d
go run ./management/cmd/rjs-management
go run ./tools/rjsctl status
```

默认管理端点为 `http://127.0.0.1:8223`：

- `/admin/`：内嵌 Admin UI，状态查询默认只读，Queue 变更需要 operator 凭据；
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
RJS_ADMIN_TOKEN=secret go run ./tools/rjsctl audit list --url http://127.0.0.1:8223 --limit 100
go run ./tools/rjsctl queue list --url http://127.0.0.1:8223
go run ./tools/rjsctl diagnostics collect --url http://127.0.0.1:8223 --output diagnostics.zip
go run ./tools/rjsctl migrate rabbitmq-definitions --output converted tests/fixtures/rabbitmq-definitions.json
docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream --namespace messaging --create-namespace
```

配置通过 `RJS_*` 环境变量或启动参数注入，详见 [配置说明](docs/configuration.md)。总体设计、[Queue Schema](docs/queue-schema.md)、[管理 API](docs/management-api.md)、[API 版本策略](docs/api-versioning.md)、[RabbitMQ 迁移](docs/rabbitmq-migration.md)、[客户端迁移与兼容矩阵](docs/client-migration.md)、[性能与长稳测试](docs/performance-testing.md)、[正式发布流程](docs/releasing.md)、[审计日志](docs/audit.md)、[凭据轮换](docs/credential-rotation.md)、[升级回滚](docs/upgrade-rollback.md)、[容量规划](docs/capacity-planning.md)、[可观测性](docs/observability.md)、[诊断包](docs/diagnostics.md)、[备份恢复](docs/backup-restore.md)、[Kubernetes 部署](docs/kubernetes.md)、上游维护、测试门禁和分期计划分别见 [架构设计](docs/architecture.md)、[上游管理](docs/upstream.md)、[测试策略](docs/testing.md) 与 [Roadmap](docs/roadmap.md)。

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
