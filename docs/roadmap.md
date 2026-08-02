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
- [x] 节点及 NATS monitoring API 聚合，支持部分节点失败；
- [x] Queue 声明模型、版本化 schema、严格 validate 和字段级 diff；
- [x] Queue 到 Stream/durable Consumer/DLQ 依赖的纯映射和确定性 plan；
- [x] 观测状态到 create/update/noop/recreate/reject 的只读 reconcile；
- [x] Bearer Token 保护的声明式 Queue apply，支持安全 create/update 与重复 apply noop；
- [x] 所有权校验、非空保护和显式确认的幂等 Queue delete；
- [x] 基于 JetStream KV 的 Queue 声明/revision 持久化与重启恢复查询；
- [x] 通过 Queue 声明幂等创建/查询/删除所拥有的 Stream 与 durable Consumer；
- [x] direct、topic、fanout Queue 绑定、队列级 subject 协议与真实 JetStream 路由测试；
- [x] TTL、长度限制、explicit ack/AckWait/MaxDeliver 与 durable advisory DLQ 基础策略；
- [x] CLI/API 的声明式 apply、diff、delete；
- [x] 基于 JetStream KV CAS 租约的 leader election 与 leader-only 持续安全 reconcile；
- [x] 多管理实例 Queue 级 CAS 锁、revision 前置条件与冲突响应；

验收：进程重启及管理实例切换不丢声明；重复 apply 无副作用；端到端测试覆盖发布、消费、重投和 DLQ。

## M2：原生 SDK 与优先级队列

- [x] 服务端发布机器可读的资源命名、消息头、优先级调度和投递语义 `v1alpha1` 契约；
- [x] `rabbit-jetstream-go` 已实现并通过兼容测试，共同固化该契约；
- [x] 多 subject/pull consumer 优先级调度与服务端 `spec.maxPriority` 资源创建；
- [x] 公平性、饥饿保护、动态优先级数和不支持组合的显式拒绝；
- [x] publisher confirm、消费并发和消息数/字节背压；
- [ ] 节点故障、网络抖动、慢消费者和积压恢复故障注入及对比性能基准（代理持久化重启和调度微基准已覆盖）。

验收：优先级行为、故障恢复和至少一次投递有可重复测试；发布 P99 和消费吞吐基准可与原生 JetStream 对照。

## M3：运维产品化与 WebUI

- [x] 队列、consumer、消息积压、节点和集群页面，以及受 RBAC/ETag/审计保护的 Queue apply/delete；
- [x] Prometheus 指标、固定版本部署 profile 和关键告警模板；
- [x] OpenTelemetry OTLP/HTTP traces、W3C 上下文提取、Queue 控制面属性及退出 flush；
- [x] OpenTelemetry OTLP/HTTP metrics 周期导出与退出 flush，并保留 Prometheus endpoint；
- [x] 带脱敏、部分失败记录和 SHA-256 清单的 CLI 诊断包；
- [x] 基于 JetStream 的写前意图/写后结果审计日志、受保护查询 API 与 CLI；
- [x] 静态 operator/auditor 最小 RBAC 与重叠 Token 无中断轮换；
- [x] OIDC discovery/JWKS 联邦认证、operator/auditor 角色映射及签名密钥轮换；
- [x] 三/五节点 StatefulSet、双管理副本、PVC/PDB/NetworkPolicy 的 Helm chart；
- [x] 三节点滚动升级/回滚自动化演练及容量规划手册；
- [x] 全 account Stream/KV/Consumer 备份、校验、恢复与数据卷销毁演练；
- [x] 内嵌 OpenAPI v1 契约、路由一致性测试及破坏性变更 CI 门禁。

验收：三节点故障与滚动升级演练通过；关键 SLI 有仪表盘和告警；权限与审计测试通过。

## M4：RabbitMQ 迁移能力

- [x] RabbitMQ definitions 到 Queue 声明的严格转换、兼容报告与校验工具；
- [x] 基于稳定消息 ID、SHA-256 和大小的影子数据核对 CLI、阈值门禁与不可覆盖证据报告；
- [x] RabbitMQ 独立 shadow queue 与 JetStream Limits-retention shadow Stream 实际采集适配器；
- [x] RabbitMQ mandatory publisher confirm、JetStream PubAck 与可恢复双确认 journal 的批量双写适配器；
- [x] 基于连续对账证据、计划摘要、幂等动作与持久 journal 的自动化切流和逆序回滚编排；
- [x] 常见 AMQP/NATS 客户端迁移指南、机器可读兼容矩阵及防漂移契约测试；
- [x] 三副本文件持久化的大规模压测、基线回归门禁与强制 24 小时长期稳定性测试工具链（发布仍须保留实际运行证据）。

## M5：AMQP 0-9-1 协议网关（未来研究）

- 独立网关原型与协议兼容矩阵；
- exchange/queue/binding、confirm、QoS、cancel 等方法映射；
- 评估事务、exclusive/auto-delete、channel 语义的成本；
- 明确网关性能预算及水平扩展模型。

此阶段可能需要更深的 NATS 集成，但任何 JetStream 源码改动必须形成单独决策记录；它不属于当前项目边界。

## 横向质量门槛

每个里程碑都必须包含：公开 API 兼容说明、自动化测试、性能回归基线、安全审查、升级/回滚步骤和可观测性。未经测量的“比 RabbitMQ 更快”不作为结论；基准需公开负载模型、持久化策略、副本数、消息大小与硬件。
