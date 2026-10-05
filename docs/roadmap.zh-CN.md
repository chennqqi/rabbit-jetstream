# Roadmap

[English](roadmap.md) | [简体中文](roadmap.zh-CN.md)

Roadmap 以能力验收为准，不绑定未经评估的日期。路线图从两个视角记录同一份工作：[版本规划](#版本规划)按版本记录功能规划，[功能规划](#功能规划)按能力域记录功能规划。版本状态与发布门禁见版本规划，功能明细与验收记录见功能规划。

## 版本规划

按版本记录的功能规划。

### 版本范围

首个正式版本使用独立 Native SDK `rabbit-jetstream-go`，目标是在功能层实现 RabbitMQ 优先级队列及配套管理、运维和部署能力。Native SDK 基于 NATS JetStream 客户端，不要求基于 AMQP。RabbitMQ/AMQP 线协议完全兼容不属于首版验收范围，统一记录在功能规划的 M5 中。

首版候选版本的双仓库版本集、发布门禁与生产证据要求见 [First Release Plan](first-release-plan.md)。

### v0.1：首个正式版本

#### 已交付

- 固定版本、未改动的上游 NATS Server subtree 与可复现镜像。
- 版本化 Queue 声明、确定性规划、安全 reconcile、CAS revision、所有权校验与持久化元数据。
- direct、topic、fanout 路由；TTL、容量、显式确认、重投、DLQ 与动态优先级数。
- Native SDK 调度、发布确认、背压、公平性与服务端/SDK 契约测试。
- 管理 API、`rjsctl`、内嵌 Admin UI、审计、RBAC/OIDC、指标、traces、诊断、备份/恢复与迁移工具。
- 单机 Compose、三节点 Compose 与生产门禁 Helm 部署。
- 故障、race、安全、性能、滚动升级、恢复与原生 Linux 资格认证（最终修订的容量 soak 见下方发布收尾）。

v0.1 的完整功能记录与验收标准见[功能规划](#功能规划)的 M0–M4。

#### 发布收尾（进行中）

M0–M4 的产品与资格认证工作已完成；剩余工作是让批准包可验证并完成发布。状态详情见[发布就绪度分析 2026-10-04](release-readiness-analysis-2026-10-04.md)。

- [ ] 在 2 vCPU/4 GB 资格主机（`jdcloudremote`）以最终冻结修订完成档位 1 的 24 小时容量 soak：持续并发发布+消费 ≥ 3,000 msg/s、发布 P99 ≤ 10 ms、完整性零缺陷；用 `tools/perfevidence -create-inaugural` 产出绑定最终修订的标准证据。（2026-10-04 21:40 CST 已启动，预计 2026-10-05 21:40 CST 完成。）
- [ ] 获得 4 vCPU/8 GB 主机后，以 24 小时 soak 正式化档位 2 包络（5,000 msg/s）；在此之前该档以 32 核 24 小时 soak 锚点加 2 核标定外推标注为 pending formalization，不作为已认证声明。
- [ ] 将双档性能包络写入 rc.3 发布说明的 qualified profile，中英语义一致。
- [ ] 在最终冻结修订以 Release 模式重跑 `make test-local-release`，并恢复 releaseapproval 工具的严格 `mode == "release"` 断言；接受 quick 模式属于放宽门禁。
- [ ] 将全部证据（批准 JSON、soak、金丝雀、集群、local-rc）统一重绑到单一最终冻结修订；批准提交保持在分支最后，或每次提交后重新绑定；在证据说明中记录"自 `033010ee` 起运行时代码 0 差异"。
- [ ] 修复证据卫生问题：五份字节相同的 gzip `canary-XXX-stage.json` 必须改为真实的分阶段 JSON。
- [ ] `make verify-release-approval` 在最终修订通过。
- [ ] 确认最终修订 GitHub Actions 全绿，重点关注 `rabbitmq-migration` 与 `kubernetes-smoke`。
- [ ] 推送最终标签并执行 `release.yml`：镜像发布与 attestation、Helm 打包、SHA256SUMS、草稿转正式。
- [ ] 完成公开签名（GPG/cosign）与再分发条款决策。
- [ ] 将 `docs/remaining-release-work.md` 与 `docs/completion-assessment.md` 回填至 rc.3 状态。
- [ ] 在批准记录中写明金丝雀演练替代边界（资格主机演练 vs 真实部署），或在真实环境执行金丝雀。
- [ ] 收集 RC 观察期反馈并关闭发布阻塞缺陷，且不改变已发布的兼容性契约。

验收：`make verify-release-approval` 在打标签修订通过且发布工作流完成；晋升 `v0.1.0` GA 遵循 [First Release Plan](first-release-plan.md) 退出标准。

### v0.2：运维扩展与真实 OIDC/IdP 集成

- [ ] 对接并验收真实外部 OIDC/IdP，包括提供方专用客户端注册、登录／回调、角色映射、过期、退出、密钥轮换、失败处理和运维手册；该工作明确延期，不阻塞当前本地 WebUI 与后端开发范围完成，现有静态 operator／auditor Bearer Token 路径继续支持；
- [ ] 扩展工作负载及命名空间租户策略、配额报告和安全自助工作流；
- [ ] 增加长期容量基线、存储／CNI 资格矩阵和自动化灾难恢复目标；
- [ ] 改进仪表盘、告警路由、审计导出和集群级诊断；
- [ ] 在获得原生硬件后正式验证 `linux/arm64`；
- [ ] 建设独立网站，介绍与功能并重：内容侧提供项目简介、文档、发布说明与下载指引；功能侧将内嵌 Admin UI 控制台拆分为可独立部署的 Web 应用，支持与管理服务分离部署、独立升级。

仓库已经包含 OIDC Bearer 校验以及通过本地测试的浏览器 Authorization Code + PKCE 实现。上述待办仅指进一步的真实提供方集成与资格验收。在恢复该事项前，不得把外部 IdP 可用性或生产 IdP 验收计入当前开发完成条件。

功能规划 M3 中的内置账号／会话与应用级多租户属于当前本地 WebUI／后端的有效需求；它们不同于延期的外部 OIDC／IdP 资格验收，也不同于 v0.2 更广泛的配额与自助租户能力。

### 未来（未排期）

- AMQP 0-9-1 协议网关研究 — 完整记录见[功能规划](#功能规划)的 M5。

## 功能规划

按能力域（M0–M5）记录的功能规划。每条记录标注交付版本；版本状态与发布门禁见上方[版本规划](#版本规划)。

### M0：可运行骨架（v0.1 已交付）

- [x] 固定版本的官方 NATS Server Git subtree；
- [x] 独立的管理服务与只读状态 CLI；
- [x] 环境变量/参数配置、结构化日志、优雅退出；
- [x] NATS/JetStream 连接、存活/就绪检查与账户摘要；
- [x] 单机与三节点 Compose 示例、容器镜像；
- [x] 基础单元测试与构建入口。
- [x] 生产级测试分层、覆盖率目标与发布门禁定义。

验收：`go test ./...`、`go build ./...` 通过；连接启用 JetStream 的 NATS 后 `/readyz` 返回 200。

### M1：队列控制面 MVP（v0.1 已交付）

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
- [x] 多管理实例 Queue 级 CAS 锁、revision 前置条件与冲突响应。

验收：进程重启及管理实例切换不丢声明；重复 apply 无副作用；端到端测试覆盖发布、消费、重投和 DLQ。

### M2：原生 SDK 与优先级队列（v0.1 已交付）

- [x] 服务端发布机器可读的资源命名、消息头、优先级调度和投递语义 `v1alpha1` 契约；
- [x] `rabbit-jetstream-go` 已实现并通过兼容测试，共同固化该契约；
- [x] 多 subject/pull consumer 优先级调度与服务端 `spec.maxPriority` 资源创建；
- [x] 公平性、饥饿保护、动态优先级数和不支持组合的显式拒绝；
- [x] publisher confirm、消费并发和消息数/字节背压；
- [x] 节点故障、代理重启、TCP 延迟/断线、慢消费者和积压恢复故障注入；
- [x] 原生 SDK 与直接 JetStream 客户端的同机三节点发布 P99、发布吞吐和消费吞吐对比基准及比例门禁。

验收：优先级行为、故障恢复和至少一次投递有可重复测试；发布 P99 和消费吞吐基准可与原生 JetStream 对照。

### M3：运维产品化与 WebUI（v0.1 已交付）

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
- [x] 内嵌 OpenAPI v1 契约、路由一致性测试及破坏性变更 CI 门禁；
- [x] 将主要的手工 Bearer Token 登录替换为内置账号密码认证，成功后返回供 `Authorization` Header 携带的短时效 Access Token；该 Token 只保存在页面内存，不得把密码或 Token 持久化到 `localStorage`、Cookie 或 URL，静态 Bearer 仅保留给恢复与自动化 API；
- [x] 增加应用级多租户：浏览器 URL 与 API Header 显式携带租户身份，具备成员关系校验和 UI 租户选择器，并按租户隔离 NATS 凭据／连接、监控、控制器、Consumer 缓存、诊断所有权及审计证据；API 与无头浏览器隔离测试对跨租户访问 fail closed；
- [x] 增加本地平台管理员访问管理，覆盖账户生命周期与租户成员关系，并保证密码哈希不出现在 API、保护最后一个平台管理员、原子持久化，以及账户授权变化后立即使旧 Token 失效；租户后端凭据仍为受保护的启动配置；
- [x] 增加租户级成员角色，使同一身份可在一个租户为 operator、另一个租户为 auditor；后端在权限判断前先选择目标租户角色，UI 切换租户时替换而不是合并权限；
- [x] Queue／Stream 与全局 Consumer 列表筛选区采用紧凑桌面布局，同时保留移动端纵向布局，并保证 375px 下页面级横向溢出为零；
- [x] 以 Chromium 与 Firefox E2E、无障碍检查和运维错误场景完成 Admin UI 工作流资格认证。

验收：三节点故障与滚动升级演练通过；关键 SLI 有仪表盘和告警；权限与审计测试通过。

### M4：RabbitMQ 迁移能力（v0.1 已交付）

- [x] RabbitMQ definitions 到 Queue 声明的严格转换、兼容报告与校验工具；
- [x] 基于稳定消息 ID、SHA-256 和大小的影子数据核对 CLI、阈值门禁与不可覆盖证据报告；
- [x] RabbitMQ 独立 shadow queue 与 JetStream Limits-retention shadow Stream 实际采集适配器；
- [x] RabbitMQ mandatory publisher confirm、JetStream PubAck 与可恢复双确认 journal 的批量双写适配器；
- [x] 基于连续对账证据、计划摘要、幂等动作与持久 journal 的自动化切流和逆序回滚编排；
- [x] 常见 AMQP/NATS 客户端迁移指南、机器可读兼容矩阵及防漂移契约测试；
- [x] 三副本文件持久化的大规模压测、基线回归门禁与强制 24 小时长期稳定性测试工具链（发布仍须保留实际运行证据）。

### M5：AMQP 0-9-1 协议网关（未来研究）

此里程碑用于完整 RabbitMQ/AMQP 协议兼容，不属于首个正式版本，也不作为 Native SDK 或优先级队列功能发布的阻塞条件。

- 独立网关原型与协议兼容矩阵；
- exchange/queue/binding、confirm、QoS、cancel 等方法映射；
- 评估事务、exclusive/auto-delete、channel 语义的成本；
- 明确网关性能预算及水平扩展模型。

此阶段可能需要更深的 NATS 集成，但任何 JetStream 源码改动必须形成单独决策记录；它不属于当前项目边界。

## 未来：RabbitMQ/AMQP 兼容

完整的 AMQP 0-9-1 兼容明确不属于首个正式版本。研究记录见[功能规划](#功能规划)的 M5。未来可能引入独立的协议网关，覆盖 exchange/queue/binding、confirm、QoS、cancel、事务、exclusive/auto-delete 语义。任何需要修改上游 NATS 的方案都必须以单独的架构决策提出，并附兼容性与性能预算。

## 横向质量门槛

每个里程碑都必须包含：公开 API 兼容说明、自动化测试、性能回归基线、安全审查、升级/回滚步骤和可观测性。未经测量的“比 RabbitMQ 更快”不作为结论；基准需公开负载模型、持久化策略、副本数、消息大小与硬件。
