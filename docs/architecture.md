# 总体架构

## 1. 目标与原则

项目目标是提供接近 `rabbitmq-server` 使用体验的 JetStream 队列发行版：统一部署、管理 API/CLI、WebUI、监控诊断，以及与原生 SDK 协作的队列能力。

核心约束：

1. JetStream 是底层消息存储与分发引擎，不修改其源代码；
2. 首选原生 NATS 协议，服务端不进入正常消息收发链路；
3. RabbitMQ 的“使用体验兼容”和 AMQP“线协议兼容”分开演进；
4. 所有高级语义都要有明确的一致性、故障和降级定义。

## 2. 逻辑架构

```text
                          管理面
 Operator ── CLI/WebUI ── HTTP API ── Policy / Topology / Diagnostics
                                      │
                                      ▼
 Application ── native SDK ─────── NATS JetStream cluster
               (priority/router)      streams + consumers + KV
                          数据面

 Future: RabbitMQ client ── AMQP gateway ── native SDK / JetStream
```

本项目的主体是包含固定版本 NATS Server/JetStream 的可部署发行版。管理服务是其中一个无状态控制面组件，并非消息服务器本身。队列定义、策略和运行状态最终存储在 JetStream/KV，多个管理实例可以横向扩展。SDK 直接访问 JetStream，避免消息经过管理服务而形成性能瓶颈或单点。

## 3. 核心领域映射

| RabbitMQ 概念 | JetStream 实现建议 | 说明 |
|---|---|---|
| Virtual host | NATS account（强隔离）或 subject namespace（轻隔离） | 首版采用 namespace；生产多租户推荐 account |
| Exchange | SDK 路由规则 + subject | direct/topic/fanout 可由 subject 与多订阅表达 |
| Queue | Stream + durable consumer | 队列声明由控制面幂等创建 |
| Binding | subject/filter subject | 元数据登记用于管理展示与校验 |
| Ack/重投 | JetStream explicit ack / AckWait / MaxDeliver | SDK 暴露 RabbitMQ 风格抽象 |
| DLX/DLQ | durable MaxDeliver advisory stream + leader-only mover | 目标确认后删除源消息，提供至少一次转移与消息 ID 去重 |
| TTL/长度 | MaxAge / MaxMsgs / MaxBytes | 映射到 stream limit policy |
| Publisher confirm | JetStream publish ack | SDK 统一错误与超时模型 |
| Priority queue | 每优先级一个 subject/filter + SDK 调度 | 见下节 |

## 4. 优先级队列

逻辑队列 `orders` 映射为 `orders.p0 ... orders.pN`，数值越大优先级越高。每级使用独立 consumer，SDK 的消费调度器总是先探测高优先级，并通过“高优先级突发额度 + 低优先级保底额度”避免低优先级永久饥饿。

首选 pull consumer：SDK 可以控制每一级的 fetch 批次和并发。仅靠 Go `select` 不能保证严格优先级，因此调度算法必须显式排序，不依赖随机选择。发布端将非法优先级按队列策略拒绝或钳制，并把逻辑队列、实际 subject 和策略版本写入消息头。

语义限制：分布式系统中“严格全局优先级”成本很高。项目承诺的是单个 consumer-group 调度器观察范围内的优先消费；已经投递给客户端的低优先级消息不会被抢占。

## 5. 服务端组件

- `management-api`：队列/策略/用户可见拓扑、健康与诊断接口；
- `operator-cli`：部署检查、状态、声明、导入导出和带脱敏/校验清单的诊断包；
- `webui`：复用管理 API，不直接持有 NATS 管理凭据；
- `controller`：把声明式队列策略收敛为 JetStream streams/consumers；
- `observability`：内嵌 Prometheus 指标、JetStream advisories、告警规则、持久化审计日志，以及可选 OTLP/HTTP traces 与 metrics；
- `amqp-gateway`（未来）：独立可选组件，不侵入 JetStream。

当前控制面已将成功 apply 的规范化 Queue plan、revision 和操作时间保存到 JetStream KV；管理进程重启后可恢复查询。只读 Admin UI 内嵌在管理二进制并通过 `/admin/` 提供集群、Queue、Stream、Consumer、controller 和 DLQ 状态；写操作、OIDC/RBAC 与更完整策略控制继续按 Roadmap 增量加入。

`rjsctl diagnostics collect` 通过只读管理 API 采集运行快照，不读取消息负载或 credentials 文件。诊断 ZIP 允许单个端点失败，清单记录每个文件的状态、大小与 SHA-256；敏感字段和 URL 用户信息在写盘前脱敏。

Queue 管理写操作先向独立的 JetStream 审计 Stream 持久化意图，再执行资源变更，最后写入相关联的结果事件。意图落盘失败时拒绝变更；结果落盘失败时返回状态不确定错误，要求运维先核查资源。

Controller 使用同一 KV bucket 中的 CAS 租约选出唯一 leader。Leader 周期读取声明并仅执行安全的 create/update/noop；需要 recreation、保留策略缩减或未实现能力的计划保持 blocked，不自动执行破坏性操作。每个声明前续租，优雅退出时条件释放租约；实例异常退出时由 TTL 保证接管上界。

所有 Queue apply、delete 和 controller reconcile 共享 Queue 级 KV CAS 锁，避免两个管理实例同时修改同一组 Stream/Consumer。HTTP 写入和 controller 快照都必须携带声明 KV revision；因此后到达的旧配置会收到冲突，而不会覆盖已提交的新配置，已删除 Queue 也不会被旧 reconcile 快照复活。锁包含过期时间，异常退出后可自动接管。

## 6. 高可用与安全

- 单机方案用于开发和低风险环境；集群方案默认 3 个 JetStream 节点、资源副本数 3；
- 管理服务保持无状态并至少运行 2 个实例，由负载均衡提供入口；
- Helm chart 使用 StatefulSet/PVC 部署 JetStream，Deployment 部署双管理副本，并通过 PDB、拓扑分散和 NetworkPolicy 建立 Kubernetes 高可用基线；
- 生产使用 NKeys/JWT 或 credentials、TLS/mTLS，不在配置或 API 中回显密钥；
- 管理 API 最终采用 OIDC/RBAC，变更操作写入不可抵赖审计日志；
- 备份以 JetStream snapshot/restore 为基础，覆盖 account 内普通与隐藏 KV/Object Store Stream；恢复前校验清单并定期执行数据卷销毁演练。多 Stream 快照为顺序采集，需要业务静默才能获得跨 Stream 一致时间点。

## 7. 兼容边界

“RabbitMQ 兼容”在当前阶段表示常用队列语义与运维体验兼容，并不表示 RabbitMQ 插件、Erlang API、AMQP 0-9-1 或管理 HTTP API 的逐字段兼容。未来 AMQP 网关需要单独定义兼容矩阵，涵盖协议方法、错误码、事务、confirm、消费取消与流控。
