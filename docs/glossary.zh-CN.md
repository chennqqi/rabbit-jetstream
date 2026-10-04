# 术语表

[English](glossary.md) | [简体中文](glossary.zh-CN.md)

控制台、API、CLI 与 SDK 中使用的术语。控制台界面渲染的中文标签在方括号中标注。

## 拓扑

- **Queue（队列）**[队列] — 声明式被管资源（Kubernetes 风格文档：`apiVersion`/`kind`/`metadata`/`spec`）。拥有 Subject、存储、保留策略、优先级、DLQ 策略与副本数。只能经控制面变更（预览 → 确认 → 条件写入）。
- **Stream（流）**[流] — Queue 的计划生成的 JetStream 流，命名 `RJSQ_<队列名>`。由控制器创建/更新；控制台中只读。WorkQueue 保留策略，确认即删除。
- **Consumer（消费者）**[消费者] — Queue 计划生成的 durable pull 消费者，命名 `RJSQC_<队列名>`（设置 `maxPriority` 时每个优先级各一个）。控制台可见，但只由控制器创建。
- **Subjects** — Queue 绑定。每个 Queue 有入口 subject `rjs.q.<queue>.ingress`；SDK 按优先级发布到 `rjs.q.<queue>.p.<级别>`。
- **租户（Tenant）**[租户] — 控制面管理的隔离 NATS 账户，路由为 `/admin/tenants/<租户>/`。定义于 `RJS_TENANTS_FILE`。
- **Binding（绑定）** — Exchange 风格的绑定声明，翻译为 Queue 的 subject（不存在独立 Exchange 资源）。

## 声明与变更控制

- **声明（Declaration）**[声明] — 元数据 KV 中存储的 Queue 文档，以修订哈希标识。
- **计划（Plan）**[计划] — 从声明推导的确定性 JetStream 资源计划（`BuildPlan`）；控制器据此对账。
- **修订（Revision）**[修订] — 控制台显示为"Plan 版本"的声明哈希；每次成功更新都会变化。
- **ETag / KV 修订**[声明 ETag] — 条件写入使用的数字 KV 修订（更新用 `If-Match`；创建用 `If-None-Match: *`）。数字类型，区别于修订哈希。
- **预览（Preview）**[预览] — 服务端非变更的试运行，把变更分类为 update / no-op / recreate / reject。
- **recreate（计划分类）** — 被判定为破坏性的计划：Stream 身份或存储语义不同，消息无法保留。需走备份路径或明确接受丢失并记录。
- **blocked（计划分类）** — 应用会违反安全规则的计划（名称不匹配、所有权、缺少 force）。
- **Force（强制）** — Stream 仍含消息时删除所需的确认；绝不绕过所有权保护。

## 存储与投递

- **R3 / R5** — 三副本/五副本 JetStream 集群与流形态。R3 是已取得资格的形态。
- **At-least-once** — 投递契约：应用必须幂等消费（稳定消息 ID、去重）。
- **Ack / Nak / Term** — 消费者结算：肯定确认（workqueue 中即删除）、否定确认（立即重投递）、终止（毒消息，不再重投）。
- **确认下限（Ack floor）**[确认下限] — 完整确认的最低消费者序号；用于 Consumer 诊断。
- **DLQ（死信）**[死信] — 经 `deadLetter.queue` 声明的死信队列。控制器对耗尽投递的消息（MaxDeliver advisory）按 at-least-once 加 `Nats-Msg-Id` 去重搬运；按队列计数为 `rjs_dlq_queue_*`。
- **MaxDeliver** — 消息进入死信前的投递上限（默认 5）。

## 控制面与安全

- **管理服务**[管理服务] — 控制面二进制（`rjs-management`），提供 `/admin/`、`/api/v1/*` 与 `/metrics`。绝不进入消息数据路径。
- **控制器**[控制器] — 选举产生的组件，把声明对账到实际 JetStream 状态；领导权由 `rjs_controller_leader` 展示。
- **元数据 quorum** — KV/元数据复制（R3），工作负载启动前必须 leader 在位且副本 current。
- **租户角色** — 每租户 `operator`（写）与 `auditor`（只读）；平台管理员管理账户。
- **Sole owner** — 发布审批记录中唯一负全责的签字人。

## 发布治理

- **exact-candidate freeze** — 冻结已打 tag 的发布树，不再合入。
- **Canary 阶段** — 1/10/25/50/100% 流量的证据阶段，含零丢失对账。
- **local-rc 证据** — 打包门禁输出（`scripts/release/local-rc.ps1`），11 项检查。
- **部分依赖（partial dependency）** — 已接受语义差异的功能对等领域（如优先级队列），在发布审批记录中附理由。
