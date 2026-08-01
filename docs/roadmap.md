# Roadmap

Roadmap 以能力验收为准，不绑定未经评估的日期。

## M0：可运行骨架（当前）

- [x] 固定版本的官方 NATS Server Git subtree；
- [x] 独立的管理服务与只读状态 CLI；
- [x] 环境变量/参数配置、结构化日志、优雅退出；
- [x] NATS/JetStream 连接、存活/就绪检查与账户摘要；
- [x] 单机与三节点 Compose 示例、容器镜像；
- [x] 基础单元测试与构建入口。
- [x] 生产级测试分层、覆盖率目标与发布门禁定义。

验收：`go test ./...`、`go build ./...` 通过；连接启用 JetStream 的 NATS 后 `/readyz` 返回 200。

## M1：队列控制面 MVP

- [x] 只读账户、Stream、Consumer API，包含稳定模型、分页和错误结构；
- [ ] 节点及 NATS monitoring API 聚合；
- 队列声明模型及版本化 schema；
- 幂等创建/查询/删除 stream 与 consumer；
- direct、topic、fanout 路由约定；
- TTL、长度限制、ack/redelivery、DLQ 基础策略；
- CLI/API 的声明式 apply、diff、delete；
- 基于 JetStream KV 的元数据与 leader election。

验收：进程重启及管理实例切换不丢声明；重复 apply 无副作用；端到端测试覆盖发布、消费、重投和 DLQ。

## M2：原生 SDK 与优先级队列

- 与 `rabbit-jetstream-go` 固化资源命名和消息头协议；
- 多 subject/pull consumer 优先级调度；
- 公平性、饥饿保护、动态优先级数和降级策略；
- publisher confirm、消费并发和背压；
- 故障注入及性能基准。

验收：优先级行为、故障恢复和至少一次投递有可重复测试；发布 P99 和消费吞吐基准可与原生 JetStream 对照。

## M3：运维产品化与 WebUI

- 队列、consumer、消息积压、节点和集群页面；
- Prometheus/OpenTelemetry、告警模板和诊断包；
- OIDC/RBAC、审计日志、凭据轮换；
- Helm chart、滚动升级、备份恢复及容量手册；
- 管理 API 版本兼容策略。

验收：三节点故障与滚动升级演练通过；关键 SLI 有仪表盘和告警；权限与审计测试通过。

## M4：RabbitMQ 迁移能力

- RabbitMQ 拓扑到本项目声明的转换/校验工具；
- 双写、影子消费、数据核对和回滚流程；
- 常见客户端迁移指南及兼容矩阵；
- 大规模压测与长期稳定性测试。

## M5：AMQP 0-9-1 协议网关（未来研究）

- 独立网关原型与协议兼容矩阵；
- exchange/queue/binding、confirm、QoS、cancel 等方法映射；
- 评估事务、exclusive/auto-delete、channel 语义的成本；
- 明确网关性能预算及水平扩展模型。

此阶段可能需要更深的 NATS 集成，但任何 JetStream 源码改动必须形成单独决策记录；它不属于当前项目边界。

## 横向质量门槛

每个里程碑都必须包含：公开 API 兼容说明、自动化测试、性能回归基线、安全审查、升级/回滚步骤和可观测性。未经测量的“比 RabbitMQ 更快”不作为结论；基准需公开负载模型、持久化策略、副本数、消息大小与硬件。
