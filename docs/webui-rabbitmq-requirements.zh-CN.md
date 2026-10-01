# WebUI 功能需求调研：对标 RabbitMQ Management

[English](webui-rabbitmq-requirements.md) | [简体中文](webui-rabbitmq-requirements.zh-CN.md)

调研日期：**2026-09-09**。状态：**候选需求，不代表已批准的发布范围**。

下一阶段文档：[WebUI 正式规划设计](webui-design-plan.zh-CN.md)——页面规格、交互契约及交付门槛；当前为草案，不代表功能完成。

## 1. 结论与证据边界

下一轮 WebUI 应从简易看板发展成可用于日常操作的管理控制台，而不只是增加统计卡片。优先做好表单化建队列、资源详情、可信状态、分页、安全变更和审计查询，再补历史指标与连接诊断。消息查看、重放、多租户和跨集群搬运需要单独的后端及安全设计。

本报告基于 RabbitMQ 官方文档（查阅时导航标示 **4.3**）以及仓库 `343ea2930519202f87732309f8007bafdd68d7ec` 的代码。**未实际操作运行中的 RabbitMQ 后台**。“文档明确的 UI”“插件扩展”“API/CLI 能力”不混为一谈；功能是否可用还取决于配置、权限和队列类型。本报告不声称某个补丁版本是最新版本。

下文优先级和验收指标均为本项目建议，不改变已冻结的 rc.2 运行时、正在执行的 24 小时验收及首发资格边界，也不承诺 RabbitMQ 协议兼容。

## 2. RabbitMQ 管理功能清单

| 功能领域 | 官方文档中的能力及边界 | 对本项目的启发 |
| --- | --- | --- |
| 总览 | 展示代理服务、资源统计及近期活动；后台历史数据不是长期监控存储。[Management](https://www.rabbitmq.com/docs/management) | 增加有诊断价值的摘要，另行接入历史数据源。 |
| 节点与资源压力 | 内存、磁盘告警可能阻塞发布，可通过连接状态辅助诊断；这是 RabbitMQ 自身语义。[Alarms](https://www.rabbitmq.com/docs/alarms) | 显示真实 NATS 健康证据，不虚构等价的阻塞告警。 |
| 连接 | 客户端身份、连接状态及连接频繁建立/断开的情况支持故障定位。[Connections](https://www.rabbitmq.com/docs/connections) | 增加客户端、节点关联及连接检索。 |
| Channel | AMQP Channel 在连接上复用操作。[Channels](https://www.rabbitmq.com/docs/channels) | 不为 NATS 生造“Channel 管理”。 |
| 队列 | 队列类型、生命周期、容量限制及投递特性均影响运维。[Queues](https://www.rabbitmq.com/docs/queues) | 使用表单、单位与说明，使配置可理解。 |
| Quorum Queue | 当前文档包含优先级计数、延迟重试状态的 UI 展示，受版本及类型限制。[Quorum queues](https://www.rabbitmq.com/docs/quorum-queues) | 仅对已有契约支持的优先级、重试状态提供展示。 |
| Stream | UI 可通过队列类型选择创建 Stream；保留策略及 offset 不同于取走即删除的队列消费。[Streams](https://www.rabbitmq.com/docs/streams) | 将 JetStream Stream 与业务 Queue 分开展示。 |
| 消费者 | ACK、prefetch、活动状态及消费能力有助于定位积压。[Consumers](https://www.rabbitmq.com/docs/consumers) | 展示待投递、待确认、重投及配置，并解释准确含义。 |
| Exchange / Binding | 交换器类型和绑定承担路由，并非所有类型都可直接映射为 NATS。[Exchanges](https://www.rabbitmq.com/docs/exchanges) | 展示现有 Queue 路由计划，不虚构独立交换器资源。 |
| 优先级 | 当前 Quorum Queue 支持 32 级严格优先级；“Quorum 不支持优先级”的旧结论已不适用。[Priority](https://www.rabbitmq.com/docs/priority) | 本项目的资格范围独立定义，不据此推断兼容。 |
| 死信 | 死信交换器在满足相应条件后路由消息。[Dead lettering](https://www.rabbitmq.com/docs/dlx) | 解释本项目目标 Queue、控制器及转移失败证据。 |
| Policy | 匹配规则、优先级及操作员约束用于集中配置；可禁用操作员策略修改。[Policies](https://www.rabbitmq.com/docs/policies) | 后续考虑模板和约束，先定义覆盖顺序。 |
| Virtual Host | 按虚拟主机划分资源、权限并施加限制。[Virtual hosts](https://www.rabbitmq.com/docs/vhosts) | 多账号隔离是架构能力，不只是增加下拉框。 |
| 用户与授权 | 资源配置、写入、读取权限与后台管理标签是不同维度。[Access control](https://www.rabbitmq.com/docs/access-control)、[Management](https://www.rabbitmq.com/docs/management) | API 强制鉴权不能被按钮显隐代替。 |
| SSO | OAuth 集成包含管理界面的登录配置。[OAuth](https://www.rabbitmq.com/docs/oauth2) | 后端能验证 JWT 不代表浏览器登录流程已完成。 |
| Definitions | 支持拓扑和配置导入导出；不等同于消息备份。[Definitions](https://www.rabbitmq.com/docs/definitions) | 将声明迁移与完整快照恢复分开。 |
| 发布、取消息、清空 | HTTP 操作包含发布、消息投递获取、清空 Ready 消息。取消息即使重新入队也改变队列状态，不是被动查看。[HTTP API](https://www.rabbitmq.com/docs/http-api-reference) | 不用消费接口实现所谓“只读预览”。 |
| 危险操作 | API 包含带条件的队列删除及连接关闭。[HTTP API](https://www.rabbitmq.com/docs/http-api-reference) | 限权、预览当前影响、明确确认并审计。 |
| Federation | 管理 UI 能力需要 federation management 扩展。[Federation](https://www.rabbitmq.com/docs/federation) | 跨集群联邦应独立立项。 |
| Shovel | 管理扩展提供搬运状态；配置涉及动态运行参数。[Shovel](https://www.rabbitmq.com/docs/shovel) | 需要工作进程、凭据和重试语义，不只是一个页面。 |
| Tracing | 跟踪插件提供采集 GUI；捕获载荷有性能及保密成本。[Firehose/tracing](https://www.rabbitmq.com/docs/firehose) | 默认只采集元数据并脱敏。 |
| 事件与审计 | event-exchange 插件发布代理服务事件，本身不等于持久化、防篡改的审计控制台。[Event exchange](https://www.rabbitmq.com/docs/event-exchange) | 利用现有审计流，并如实说明其保证边界。 |
| 升级管理 | 有特性标志管理接口，也有文档明确的弃用功能面板；本调研未验证每个标志都有 GUI 控件。[Feature flags](https://www.rabbitmq.com/docs/feature-flags)、[Deprecated features](https://www.rabbitmq.com/docs/deprecated-features) | 先展示版本及能力，不直接提供主机升级按钮。 |
| 长期监控 | 官方推荐使用 Prometheus/Grafana 做生产监控。[Monitoring](https://www.rabbitmq.com/docs/monitoring) | 对接或跳转外部监控，不暗示已内置告警通知服务。 |

不要把旧版经典队列镜像配置照搬进新设计。同样，CLI 支持某项操作不代表 GUI 有同等功能：[rabbitmqadmin](https://www.rabbitmq.com/docs/management-cli) 是 HTTP API 客户端，并不能替代所有节点管理工具。

## 3. 当前实现与具体差距

证据：[HTML](../admin-ui/dist/index.html)、[看板代码](../admin-ui/dist/app.js)、[Queue 编辑器](../admin-ui/dist/management.js)、[HTTP Handler](../management/internal/api/handler.go)、[OpenAPI](../api/openapi.yaml)、[现有浏览器测试](../tests/admin-ui/admin-ui.spec.js)。

| 当前实现 | 差距 / 后续需求 |
| --- | --- |
| 内嵌静态 `/admin/`，Overview、Queues、Nodes 导航和四张摘要卡片 | 缺少独立 Stream、Consumer、连接和审计工作区。 |
| 通过 JSON 创建/修改 Queue，详情弹窗，输入名称确认删除 | 增加表单、校验、影响预览和字段解释，保留专家 JSON 模式。 |
| Queue、Stream 只请求 `limit=200`，本地筛选，以数组长度计数 | 第一页以外的资源是没有加载，不是不存在；应完整分页并采用服务端总数。 |
| 只要找到同名关联 Stream 就标记 Ready | 资源存在不等于已收敛、消费者可用或副本健康。 |
| 创建模板固定 `replicas: 3` | 单实例演示需要明确部署模式；不能把三节点故障后只剩一个在线节点误判为单机模式。 |
| 打开编辑器时读取声明，保存前重新读取最新 ETag | 静态检查发现覆盖并发修改的风险：编辑内容可能早于新获取的版本；应绑定最初编辑的版本。 |
| 删除也会重新读取最新版本 | 删除应绑定用户实际查看并确认过的版本及影响。 |
| 每 10 秒刷新，使用 `Promise.allSettled` 处理部分失败 | 缺少取消、退避、过期标识、后台标签暂停、防刷新重叠；当前没有历史曲线。 |
| 输入操作员 Token；后端支持静态 Token/OIDC 校验 | 尚无浏览器 SSO 和完整读权限策略。资源读取 API 通常无需认证；写入、审计分别受角色限制。隐藏按钮不能保护读取接口。 |
| 已有审计接口及结构化资源数据 | 优先接入审计和丰富详情，不必重复建设同类服务。 |
| 已有 CRUD、鉴权错误、无障碍及响应式测试 | 需补真实版本冲突、分页、混合故障和下述流程；旧测试不能证明新流程就绪。 |

文档存在漂移：[管理 API 文档](management-api.md) 仍称后台为“read-only”，而代码已有写操作。后续实现验收时应同步修正。

## 4. 概念映射：借鉴操作流程，不照搬不兼容对象

| RabbitMQ 概念 | Rabbit-JetStream 的对应关系 / 限制 |
| --- | --- |
| Queue | 业务 Queue 声明及生成的 Stream/Consumer 计划；同时展示期望状态和实际状态。 |
| Stream / Consumer / Connection | 三类不同对象；持久 Consumer 不等于持续在线的客户端，更不是 AMQP Channel。[NATS Consumers](https://docs.nats.io/learn/jetstream/pull-consumers) |
| Exchange / Binding | 现有 `bindings` 在 Queue 计划内支持 direct/topic/fanout，生成的 subject 具有 Queue 作用域；没有独立交换器 CRUD API，通配符转换必须按契约验证。 |
| Vhost | NATS Account 可隔离命名空间，但必须设计真实账号、凭据和控制器边界；标签不构成隔离。[NATS Accounts](https://docs.nats.io/learn/security/accounts-and-multitenancy) |
| 优先级 | 遵循原生 SDK 契约；首发验收范围为 **0–7**、三节点 **R3**、至少一次投递、**linux/amd64**。解析器接受更多配置不代表已经取得生产资格。 |
| DLX | 当前使用 `deadLetter.queue` 和控制器转移，不是 AMQP 交换器；不承诺相同死信头、TTL 或顺序语义。 |
| Definitions / Backup | 声明配置迁移与包含消息、消费状态的快照不同；现有恢复要求空账号、停止写入及控制器，不保证跨 Stream 原子性。 |
| 消息测试 | 管理服务必须保持在消息数据路径之外；未来载荷工具需要批准独立的 SDK 执行边界。 |

契约证据：[Queue 定义](../internal/topology/queue.go)、[计划生成](../internal/topology/plan.go)、[迁移边界（英文）](rabbitmq-migration.md)、[生产就绪](production-readiness.zh-CN.md)、[备份恢复（英文）](backup-restore.md)。

## 5. 建议信息架构

```text
总览 — 健康、资源总数、当前问题、近期变更
队列 — 检索/列表 → 概况 | 配置 | 路由 | 消费者 | 事件
Streams — 只读列表与详情；区分本系统管理和外部资源
节点 — 节点详情 → 连接（P1）
运维 — 审计（P0） | 诊断、声明导出（P1）
设置 — 身份与能力（P0） | SSO、访问策略（P1）
```

第一阶段从 Queue/Stream 进入 Consumer 详情，全局 Consumer 检索及历史曲线放在 P1。不将未实现的导航入口伪装成已完成功能。

## 6. 需求清单、优先级与验收标准

> **状态回填（2026-10-01）：**本 backlog 落后于多个已实现的候选能力，落地时未回填状态。已在当前开发候选中实现：WEB-017（历史指标）、WEB-022（DLQ 诊断）、WEB-024（诊断任务）、WEB-026（运维告警投影），以及 WEB-002/003/005/007/010/012/018/019/021/023/025/027/034 的部分内容——见 `webui-development.md` 与 `webui-api-contracts.md`。2026-10-01 的评审加固工作（告警空后端崩溃修复、会话过期态、登录前 OIDC 配置、按队列 DLQ 指标、删除预检 DLQ 依赖方检查）已在之上交付；完整计划见 `review-improvement-plan.zh-CN.md`。WEB-028–033 仍未实现，需单独批准。

**P0：**下一轮可用控制台。**P1：**P0 后补齐运维深度。**P2：**单独批准的扩展。依赖标记：**F** 现有 API/前端工作；**B** 后端新增或调整；**A** 架构/安全决策；**O** 外部集成。组合标记表示同时依赖。即使只有一名负责人，operator/auditor 仍是权限概念，不意味着新增审批人员。

| 编号 | 优先级 / 依赖 | 需求及最低验收标准 |
| --- | --- | --- |
| WEB-001 | P0 / F | 控制台导航：详情有稳定 URL，支持浏览器前进/后退和刷新定位；不存在的资源显示正确的未找到状态。 |
| WEB-002 | P0 / F+B | Queue/Stream 分页：使用服务端总数；201 个资源全部可访问；全局筛选、排序先于分页，筛选变更重置页码。 |
| WEB-003 | P0 / F | Queue 表单：subjects 与 bindings 互斥；校验名称、存储、保留、ACK、投递上限、优先级和 DLQ；JSON/表单双向切换保留全部支持字段及标签。 |
| WEB-004 | P0 / B | 部署能力感知：显式声明单机/集群及已验收限制；模式未知时要求选择副本数；三节点降级不能静默改为 R1 默认值。 |
| WEB-005 | P0 / F+B | 变更预览：对比当前和目标配置，展示受影响的生成资源，区分安全修改与拒绝项；预览不得写入。 |
| WEB-006 | P0 / F | 并发控制：保留打开编辑器时的 ETag；冲突时展示服务器/本地差异，明确选择重载或重新提交；禁止静默刷新前置版本。 |
| WEB-007 | P0 / F+B | 安全删除：精确名称、已确认版本、资源归属及可获取的积压/消费者影响；force 默认关闭，影响变化重新确认，不自动重试。 |
| WEB-008 | P0 / F+B | 可信健康：区分未知、过期、缺失、收敛中、降级、健康；不能仅凭 Stream 存在判定健康，证据缺失不显示绿色零值。 |
| WEB-009 | P0 / F | Queue 详情：声明版本、保留/容量、subjects、生成 Stream、优先级 Consumer、待投递/待确认及副本 leader/current/lag；指标带单位和定义。 |
| WEB-010 | P0 / F | Stream/Consumer 查看：使用现有接口提供独立只读详情；标明外部管理资源，不提供不受支持的底层修改按钮。 |
| WEB-011 | P0 / F | 节点详情：版本、运行时长、CPU、内存、连接、JetStream 容量字段及查询错误；已用字节不冒充未知的主机磁盘上限。 |
| WEB-012 | P0 / F | 稳健刷新：默认 10 秒，支持手动刷新、最后成功时间；最多一个刷新在途，取消失效请求、隐藏标签暂停，失败指数退避上限 60 秒。 |
| WEB-013 | P0 / F+B | 审计工作区：关联意图/结果、资源、操作者、时间；支持分页，全量筛选需要服务端支持；不确定结果提示“核实后再重试”。 |
| WEB-014 | P0 / F+B+A | 访问安全基线：明确并落实读/写/审计开放策略；测试直接越权调用 API；秘密不进入 URL、浏览器持久存储或日志。 |
| WEB-015 | P0 / F | 错误恢复：区分校验、鉴权、冲突、不可用和结果不确定；给出可执行提示及可获取的关联标识，不影响其他正常面板。 |
| WEB-016 | P0 / F | 中英文与无障碍：翻译导航、表单、错误及单位；全部操作可用键盘，弹窗恢复焦点，状态不能仅靠颜色识别。 |
| WEB-017 | P1 / B+O | 历史指标：基于明确历史数据源提供 15 分钟/1 小时/24 小时范围，统一时间戳，显示缺口及重置，不拼接虚构数据。 |
| WEB-018 | P1 / B | 积压诊断：结合待投递、待确认、重投、消费配置及节点健康；建议附证据，不以单个瞬时值宣称已确定根因。 |
| WEB-019 | P1 / B | 客户端连接：服务端分页，按客户端/节点/身份检索，限制订阅详情规模；敏感元数据脱敏，浏览器不直接访问公开 NATS 监控端口。 |
| WEB-020 | P1 / F+B | 路由工作区：图或表展示 Queue、Binding、生成 subject、Stream；无需实际发消息即可校验受支持的匹配，标明不支持的转换。 |
| WEB-021 | P1 / B | 全局 Consumer：按 Queue/Stream/durable/状态分页检索；不能仅因没有活跃客户端连接就将持久 Consumer 判为故障。 |
| WEB-022 | P1 / F+B | DLQ 诊断：目标配置及是否存在、控制器健康、转移证据；全局计数明确标为全局，单 Queue 历史必须补采集。 |
| WEB-023 | P1 / B | 声明导入导出：版本/Schema 校验、依赖预览、脱敏、dry run、逐项结果；不宣传原子回滚，明确不含消息备份。 |
| WEB-024 | P1 / B | 诊断包下载：经受限任务/API 复用 CLI 脱敏规则；默认仅元数据，设置大小和有效期并审计访问；不接受任意主机路径或命令。 |
| WEB-025 | P1 / B+A | 浏览器 SSO/会话：选择经过审查的 OIDC 登录方案，处理过期、退出及服务端授权；后端 Token 校验只是依赖，不代表 UI 已实现。 |
| WEB-026 | P1 / B+O | 运维告警：展示来源、阈值、持续时间及恢复状态；未另建通知服务时跳转外部系统，不虚报“通知已发送”。 |
| WEB-027 | P1 / F+B | 模板及批量变更：模板可查看，逐目标预览，批次有上限，逐项报告结果；不能把部分失败展示成全部成功。 |
| WEB-028 | P2 / A+B | 多账号隔离：批准账号选择和凭据生命周期，缓存及服务端授权按账号隔离；跨账号负向测试通过后才能开放。 |
| WEB-029 | P2 / A+B | 消息诊断：单独批准 SDK 路径，明确状态变更警告、载荷上限、脱敏、专门权限及审计；不提供误导的只读 peek，不让管理服务转发数据面消息。 |
| WEB-030 | P2 / A+B | 清空/重放/重试任务：定义 ready/in-flight 范围、顺序、重复、限速、取消和幂等；dry run、输入确认；不得套用删除 Queue 的语义。 |
| WEB-031 | P2 / A+B | 策略/配额：定义模板、Queue 声明、操作员约束的优先级；预览受影响资源，拒绝不受支持的在线转换。 |
| WEB-032 | P2 / A+B | 联邦/搬运：工作进程、凭据、失败重试语义及归属须独立验收；不能把普通 NATS 连接叫作联邦。 |
| WEB-033 | P2 / A+B | 备份恢复编排：保留空账号、停止写入、快照一致性约束；启用 UI 恢复前先通过隔离恢复演练。 |
| WEB-034 | P1 / F+B | 兼容性页面：运行时/构建/SDK 契约，区分支持与已验收能力；没有单独授权的 API，不提供主机升级、重启或功能开关按钮。 |

P0 的依赖不可忽略：表单、详情可立即开始，但扩大网络开放范围必须先满足 WEB-014，可信状态依赖 WEB-008，安全写入依赖 WEB-006/007。P0 不要求建设指标数据库或消息试验台。

## 7. API 与指标设计输入

现有路由以 [Handler](../management/internal/api/handler.go) 和 [OpenAPI](../api/openapi.yaml) 为准。下表“需要扩展”均为建议，**不是已实现接口**。

| 现有能力 | 界面用途 | 需要扩展 |
| --- | --- | --- |
| `GET /api/v1/info`、`/cluster`、`/nodes`、`/controller` | 总览、节点、控制器健康 | 显式部署模式、能力边界及收敛健康原因。 |
| `GET /api/v1/queues`、`/streams` | 资源列表 | 全局筛选/排序；评估后端枚举成本，而不仅是响应大小。 |
| `GET /api/v1/queues/{queue}`，同路径 `PUT`/`DELETE` | 查看、编辑、删除 | 绑定读取版本的预览；按需新增无副作用校验契约。 |
| `GET /api/v1/streams/{stream}` 及其下 `/consumers` | Stream/Consumer 详情 | 按需新增跨 Stream 索引、受限查询和诊断。 |
| `GET /api/v1/audit` | 审计意图/结果 | 全量筛选及受控导出。 |
| `GET /metrics` | 监控集成 | 历史查询与鉴权边界；浏览器不抓取任意 URL。 |
| 暂无连接、导入或诊断任务 API | 新 P1 页面 | 服务端统一访问监控和受限任务；有 CLI 不代表有 HTTP API。 |

现有集合接口采用 offset/limit，默认 50、最大 200，返回 `items`、`total`、`offset`、`limit`。部分后端路径先枚举再切片，因此 UI 分页不能单独证明扩展性。创建使用 `If-None-Match: *`，更新/删除要求 `If-Match`，删除还要求 `X-RJS-Confirm-Queue`。保留后端对不安全变更的拒绝，不用自动删除重建绕过。

指标口径要求：

- Stream 存储消息数不能直接称为“业务待消费消息数”；不要累加重叠 Consumer 视图得到 Queue 总量。
- Consumer `Delivered` 是消费序列，不是业务处理成功数；没有明确契约，不能把 `Redelivered` 当作单调递增事件计数器。
- 区分瞬时值、累计计数、速率窗口、单位、采样时间及进程重置；缺失样本保持未知。
- 节点流量速率不自动等于原生 SDK 确认吞吐；优先级 Consumer 聚合必须说明公式。
- 副本 current/offline/lag 带观测时间；当前未暴露的主机文件系统容量、文件描述符占用必须另取真实证据。
- NATS `/connz` 可供连接诊断，`/routez` 描述路由；路由池意味着路由连接数不等于独立对端数。监控接口保持私有，服务端使用限定范围的查询。[NATS 监控接口](https://docs.nats.io/learn/monitoring/monitoring-endpoints)

实现证据：[JetStream Client](../management/internal/jetstream/client.go)、[Monitoring Client](../management/internal/monitoring/client.go)、[审计保证（英文）](audit.md)、[OIDC（英文）](oidc.md)、[诊断包（英文）](diagnostics.md)。

## 8. 验收流程与质量门槛

1. **创建 Queue：**明确部署模式，填写绑定或 subjects、保留及投递配置，预览后单次提交，再查看真实观测状态。不支持的配置在写入前拒绝，错误不清空表单。
2. **定位积压：**从 Queue 进入 Consumer、Node，区分待投递与待确认，看到观测时间和证据缺口；不能把存储消息直接解释为处理失败。
3. **并发编辑：**两个编辑器读取同一版本，第一个保存后，第二个得到冲突及差异；不得因 UI 刷新 ETag 而覆盖第一个修改。
4. **影响变化后的删除：**预览后被其他操作修改，原确认必须失败并要求重新查看；force 始终显式且默认关闭。
5. **部分故障及不确定写入：**一个接口失败，其他面板仍可使用且新鲜度可见；写入结果或审计记录不确定时，引导核实，不自动重发。

以下为建议的非功能验收目标，实现签收前需要冻结具体硬件和负载配置：

- 数据规模：夹具包含 10,000 个 Queue、100,000 个 Consumer；UI 默认每页 50，服务端响应上限 200。除 Mock UI 测试外，单独验证后端成本有界。
- 性能：在约定的本地测试配置下，列表/筛选 API p95 ≤ 1 秒，首个可用列表 ≤ 2 秒；报告数据量、并发、延迟和冷/热状态。这是目标，不是当前已测结果。
- 可靠性：最多一个看板刷新在途；默认间隔 10 秒，失败退避上限 60 秒；一个来源失败不清空其他正常面板。
- 无障碍：目标 WCAG 2.2 AA，自动扫描无 serious/critical 问题，并人工验证键盘、焦点；测试 375 像素和 1440 像素布局及中文文案扩展。[WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- 浏览器：保留 Chromium、Firefox 回归门槛；其他浏览器只有真实执行证据后才列为已验证，不能从引擎相似推断。
- 安全：覆盖直接越权请求、Token 泄露、资源名称/元数据 HTML 注入、适用时的跨账号隔离，以及危险操作冲突路径。
- 文档：中英文指南、API Schema、截图及错误示例与实际流程一致；不允许占位按钮或误导性的空状态成功。

## 9. 推进顺序与负责人决策

**增量 A——可用且安全（P0）：**实现 WEB-001–016；先明确读访问策略和部署模式，复用已有详情、审计 API。五条验收流程、API 授权负向测试和双语文档全部通过后才能完成。

**增量 B——运维深度（P1）：**选择指标存储、受限诊断/查询契约后，推进 WEB-017–027 和 WEB-034。历史曲线必须来自真实时序数据，连接诊断必须经受保护的监控访问。

**增量 C——单独批准的扩展（P2）：**WEB-028–033，每项需要架构决策记录及相应安全、资格证据。“对标 RabbitMQ UI”本身不构成批准。

当前仅一名负责人，可直接在交付任务或文档记录范围选择及验收结论，不引入多人审批链。逻辑权限仍应遵循最小授权。不要因为存在此需求清单，就扩大正在执行的冻结版本验收范围。

开展相关工作前需记录的决策：

1. **读取开放策略：**建议非回环地址部署均要求后台读取认证；如保留本地演示例外，需要明确选择并记录。
2. **部署模式：**建议服务端显式声明单机/集群意图，不仅依据当前可达节点数推断。
3. **指标系统：**建议优先集成外部 Prometheus/Grafana，而非自建时序库；确定查询授权和保留周期。
4. **租户范围：**建议第一轮保持单账号，增加账号选择前先实现真实隔离。
5. **载荷操作：**建议 P0/P1 不包含消息正文、清空及重放；确有需要再批准独立原生 SDK 执行方案。

本轮仅产出需求文档，没有实现上述功能、执行竞品后台实测或修改运行中的 Docker/裸机服务。
