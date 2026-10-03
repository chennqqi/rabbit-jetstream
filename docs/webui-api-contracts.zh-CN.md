# WebUI API 接入契约

[English](webui-api-contracts.md) | [简体中文](webui-api-contracts.zh-CN.md)

日期：2026-09-09。版本：0.6。状态：**会话身份、Consumer 读取、建议性预览和无损可编辑 Queue 回读已在源码实现、尚未发布；其余扩展仍为提案**。

细化[设计规划](webui-design-plan.zh-CN.md)的 C-01–07 和[写操作恢复原型](webui-mutation-design.zh-CN.md)。下文区分初始 API 基线、扩展提案和实现更新。v0.1 仅修改文档；v0.2 新增精确 Consumer 详情源码、OpenAPI 和测试。生产 UI、冻结制品和远程环境保持不变。一位负责人即可审批，但审批不能代替测试。

## 删除预检实现更新

仅 operator 可用的 `GET /api/v1/queues/{queue}/delete-preview` 及单次提交候选界面已在源码实现，详见[契约与安全边界](webui-delete-preview.zh-CN.md)。下方历史基线早于此新增项。未知结果保持锁定并提供只读证据；持久化判定及完整删除体验验收仍待完成，不代表已发布。

## 1. 已核实的 v0.1 接口基线（v0.2 新增项见 C-06）

依据：[HTTP handler](../management/internal/api/handler.go)、[OpenAPI](../api/openapi.yaml)、[后端](../management/internal/jetstream/client.go)、[声明](../internal/topology/declaration.go)、[资源模型](../management/internal/jetstream/models.go)。

| 当前路由 | 实际返回/用途 | 重要限制 |
| --- | --- | --- |
| `GET /api/v1/info` | 名称/版本/运行时间、脱敏 NATS URL、账号资源总量 | 不是身份、权限、部署意图或资格信息；错误可能为 `{error: string}` |
| `GET /api/v1/cluster` | 脱敏服务 URL 和账号统计 | 不是集群成员或健康状态 |
| `GET /api/v1/nodes` | 监控快照和逐节点证据/错误 | 目标是配置的监控端点，不是权威成员清单 |
| `GET /api/v1/queues` | 声明分页 `{items,total,offset,limit}` | 无全局搜索/筛选/排序契约，也无 Queue 观测健康 |
| `GET /api/v1/queues/{queue}` | 声明及 `ETag` 中带引号的 KV 版本 | 不是原始可编辑 Queue 文档，也不是新资源观测 |
| `GET /api/v1/streams`、`GET /api/v1/streams/{stream}` | Stream 分页或精确观测对象 | 资源模型无逐响应观测时间 |
| `GET /api/v1/streams/{stream}/consumers` | Consumer 观测分页 | 无精确 Consumer GET；先枚举全部，再排序/切页 |
| `PUT /api/v1/queues/{queue}` | 条件应用，成功或阻断时返回 `ReconcileResult` | 同步写入尝试，不是 `202` 异步任务，也不是健康保证 |
| `DELETE /api/v1/queues/{queue}` | 条件删除，返回 `DeleteResult` | 计数不是原子空/未使用保护；无预览接口 |
| `GET /api/v1/audit` | 意图/结果事件分页，序号由新到旧 | 无资源/请求/操作者/时间筛选或精确事件查找 |
| `GET /api/v1/controller` | 控制器级状态 | 不是逐 Queue 收敛证据 |

资源读取默认要求 operator 或 auditor 认证；只有显式启用且仅绑定 loopback 的本地演示模式允许匿名读取。写入要求 operator，审计允许 operator 或 auditor。所有静态角色 Token 和 OIDC verifier 均未配置时，受保护能力返回相应的 404 禁用响应；否则缺失/无效 bearer 返回 401 `unauthorized`；格式正确但已过期的本地 Access Token 返回 401 `token_expired`，使控制台能够精确标注过期而不是凭空假定；角色不符返回 403 `forbidden`。配置后，公开的 `/api/v1/oidc/config` 与 `/api/v1/oidc/token` 提供无状态浏览器 Authorization Code + PKCE 入口/回调契约；不创建 Cookie 或服务端会话，也不返回 refresh token。

默认分页 offset=0、limit=50，limit 范围 1–200。资源分页将超出总量的 offset 收敛到 total；审计分页保留请求 offset。目前不校验未知查询参数是否为受支持筛选。不能向旧服务发送 `q` 就宣称已全局搜索。

## 2. 字段映射与数值精度

| UI 概念 | 当前来源 | 必须采用的解释 |
| --- | --- | --- |
| 条件写入版本 | Queue GET 的带引号 `ETag`，来自 `kvRevision` | 作为不透明字符串保存，编辑期间固定原值 |
| 期望内容版本 | 声明 `revision`、`plan.revision` | 内容生成的字符串，不是 KV 版本或自增计数 |
| 声明时间 | `appliedAt` | 不是新的 Stream/Consumer 观测时间 |
| Queue 的 Stream | `plan.stream.name` | 从计划解析，不做全账号搜索 |
| 托管 Consumers | `plan.consumer` 加 `plan.priorityConsumers[]` | 优先级 0 在主 Consumer 中；maxPriority=7 是同一 Stream 上八个 Consumer |
| 期望 ACK / 观测 ACK | `plan.consumer.ackWaitNanos` / 观测 `ack_wait_nanos` | 计划 camelCase、观测 snake_case，不能相互覆盖 |
| 期望/观测限制 | 计划 `maxAgeNanos/maxBytes/maxMessages`；Stream `max_age_nanos/max_bytes/max_messages` | 年龄限制不是保留策略，仅按已定义规则规范化不限/默认语义 |
| Consumer 过滤器 | 观测 `filter_subjects`，回退 `filter_subject` | 复数非空优先，否则取单数；没有过滤器时不能捏造 Queue Subject |
| 副本健康 | 独立 `current`、`offline`、`lag`；Leader 在 `cluster.leader` | 在线未必同步；缺失 cluster 数据不是空且健康的 R3 集合 |
| 积压 | Stream `messages`；Consumer `pending`、`ack_pending` | 标明具体 Consumer，不能相加无关口径或仅汇总当前页 |
| 投递证据 | Consumer `delivered`、`redelivered`、`waiting` | delivered 是序号，不是业务处理完成数；不捏造速率或在线客户端数 |

计划/观测中的 int64、uint64 JSON 数字可能超出 JavaScript 安全整数。生产传输层必须在普通 `JSON.parse` 丢失精度**之前**保留精确数字 Token；已舍入数字再转 BigInt 不能恢复精度。ETag 保持文本。任何拟新增的精确字符串字段必须采用兼容增加或版本化，不能悄悄修改 v1 现有类型。Schema 驱动编辑器要保留 labels、路由、可选优先级（含显式零）、DLQ 和大整数限制。[management.js](../admin-ui/dist/management.js) 的现有 `queueFromPlan` 不能证明无损往返。

## 3. 响应到 UI 的映射（当前真实报文行为）

| 响应/证据 | UI 状态与动作 |
| --- | --- |
| 应用返回 200，`blocked:false`、`status:ready` 或 `noop` | 应用调用成功返回，随后独立读取声明和资源。`ready` 是协调计划状态，不是健康/完全收敛。`noop` 可以不改变 KV 版本。 |
| 应用返回 200 但无 ETag | 后续声明读取未提供版本；不能生成 `旧值+1` 或立即允许再次条件写入，应重新 GET Queue。 |
| 409，包含 `ReconcileResult.blocked:true` 和 `operations` | 安全/转换阻断，显示涉及操作及原因；不一定是编辑过期，未定义强制更新绕过。 |
| 409，`error.code=conflict` | 可能是创建目标已存在、KV 版本变化，**也可能是资源锁竞争**。保留草稿，重读当前声明后比较；不能一律标成字段变化或自动重试。 |
| 409 `name_mismatch` | URL 与文档不一致，应修正请求而不是 rebase。 |
| 400 校验/确认/分页错误 | 修正字段/请求并保留输入；当前消息不是结构化字段错误映射。 |
| 401 / 403 / 受保护能力 404 disabled | 重新认证或说明禁止/停用，不能显示为空资源或自动重试写入；不能仅凭 401 诊断过期。 |
| 读取时 404 `not_found` | 请求资源缺失，不是零计数。写入时还需看上下文：依赖缺失不一定是当前 Queue 缺失。 |
| 428 `precondition_required` | 必须获取有效原始 ETag，或创建时用 `If-None-Match: *`，不是允许无条件重试。 |
| 写入时 503 `audit_unavailable` | 同一码覆盖审计后端缺失、**写入前**意图持久化失败、**写入尝试后**结果持久化失败。仅凭机器码不能证明未写入；没有可靠补充证据时视为不确定，不能解析英文错误消息决定重试。 |
| 其他写入 5xx、断线、超时、格式错误/无法识别的结果 | 可能部分生效或结果未知；保留操作上下文并核查，不自动重放请求。审计结果 `failed` 不证明已经回滚。 |

错误解码保留状态码、响应头及安全的原始响应差异，兼容标准 `{error:{code,message}}`、旧字符串错误、非 JSON 传输失败。不能只抛一段文本而丢失 `operations` 或关联信息。`X-Request-ID` 在进入审计意图处理后返回，不保证所有响应都有。调用方 ID 长度 1–128，仅 ASCII 字母/数字或 `-_.:`；每次明确尝试生成新 ID，发送前保存在内存。这是**关联标识，不是幂等键**；相同 ID 不保证重复 PUT/DELETE 安全。

## 4. 扩展契约——各契约单独注明实现进展

下列名称是供实现审查的具体提案。在实现且写入发布 OpenAPI/能力声明前，不得调用或宣称可用。

### C-01 · 身份与暴露策略

**v0.6 更新：**需认证的 `GET /api/v1/session` 返回已验证 actor/role、角色权限、已知 OIDC 过期时间及当前资源读取策略。见[身份语义](webui-session.zh-CN.md)。浏览器 Authorization Code + PKCE 现复用该仅内存会话模型；静态 Bearer 输入仍作为恢复和本地管理入口保留。

拟新增 `GET /api/v1/session`，返回已验证的 `actor`、`role`、`permissions`，仅在已知时返回过期时间。无效凭据 401、禁止能力 403，不以浏览器解析 Token 作为授权依据；静态 Token 未必有过期时间。复用 operator/auditor，不创造 viewer 角色。

非回环受保护读取和明确的本地演示只读例外仍是 D-05 安全决策，**本文未启用该策略**。资源 API、`/info`、启动/能力信息、静态资源、健康/就绪、metrics 分别定义策略。启动信息可公开非敏感能力/Schema 标识，但不能泄露账号资源或凭据。恢复认证保留内存草稿；主动清除会话时警告并清除敏感状态。验证必须直接请求实际 handler，不能只检查 UI 按钮禁用。

### C-02 · 能力与规范化可编辑文档

Schema 发布与消费更新：已实现认证的全字段明确类型编写 Schema、内容版本绑定、Settings 展示及预览/发送前 Schema 读取。详见 [Schema 契约](webui-queue-schema.zh-CN.md)。发布包清单加载及运行时版本／revision／WebUI 精确绑定也已实现；签名验证与最终发布审批证据仍是独立工作。下文缺少 Schema 的表述记录原始缺口。

**实现更新：**Queue GET 的增量 `document` 字段已实现，严格检查完整 Plan 往返；无法表示的声明返回 null 及 `document_error`。见[可编辑文档语义](webui-queue-document.zh-CN.md)。认证保护的控制台能力、显式配置部署期望、能力绑定的预览/发送前检查，以及接收实例的可选 `X-RJS-If-Capabilities-Match` 前置条件已实现，详见[能力语义](webui-capabilities.zh-CN.md)。格式错误返回 400，版本不匹配在审计/后端操作前返回 412；候选 UI 要求此能力，旧客户端不带此头仍兼容。共享读取通知已实现，可使保留的未提交审阅失效，但不会解锁正在处理或结果未知的操作。完整 Schema、发布资格清单接入仍待完成，不代表二进制证明或集群级原子性。

拟新增 `GET /api/v1/console/capabilities`：提供 `schemaVersion`、`features`、服务端声明的部署模式 `standalone|cluster|unknown`、支持值及独立的资格范围。当前解析器支持副本 1/3/5、优先级 0–255；当前生产资格边界仍为 linux/amd64、三节点 R3、优先级 0–7、至少一次投递。可达节点数或解析器支持均不等于资格通过。缺少能力信息表示未知，不能猜测生产默认值。

定义完整版本化 Queue Schema 和精确可编辑文档读取，优先考虑给声明 GET 兼容增加规范化 `document`。文档必须来自已证明的逆向映射或保存的规范声明，不能用有损 JS 重构。不能无损往返的旧记录需明确禁止编辑直到问题解决。规范默认值与原始省略语义分开显示，不要求恢复原始空白格式。当前 storage/ACK/最大投递默认值为 file/30s/5；Queue 校验要求 replicas，浏览器不能独立猜值。

### C-03 · 全局列表查询

在明确宣告支持后扩展 `q`、`sort`、`order` 及各路由过滤器。`q` 定义为去除首尾空白、忽略大小写的字面子串，不支持正则：Queue/Stream 查名称，Consumer 查名称或任一有效过滤 Subject。Consumer mode 接受 `pull|push`，省略表示全部。初始按名称升序，以稳定身份打破同值排序；先过滤，再排序和 offset/limit。验证支持的参数和值，不能静默忽略，并保持原有基础分页兼容。

完整响应保留 `{items,total,offset,limit}`，可附查询/观测元信息；`total` 是筛选后总量，不是已加载行数。未来若明确支持部分响应，使用 `complete:false` 和未知 total，不能返回貌似权威的数字。在部分语义实现前，v1 列表失败仍返回错误。资源变化下 offset 分页不是快照：说明变化、按身份去重、恢复空末页且不无限请求。响应小不等于后端枚举成本有界；宣称规模就绪前必须验证索引/缓存或定向实现及取消请求的成本。

### C-04 · 观测证据

实现更新：节点响应已包含逐来源读取证据。varz 数值标量及 jsz 内存/存储/Stream/Consumer/消息数/元数据集群规模/待处理数保留明确报告的零；来源省略或为 null 的字段保持缺失，jsz 读取失败亦不补零。来源可用仅表示 HTTP/JSON 解码成功，不代表每个字段都存在。候选界面将缺失/无效数值显示为未知。JetStream 启用状态推断、路由集合完整性、来源身份关联及完整观测/健康/新鲜度契约仍需单独验证。下段记录原始缺口，不代表当前数值序列化行为。

拟新增 `GET /api/v1/queues/{queue}/observation`，提供期望版本引用、必需资源集合、分来源时间/状态/错误及明确健康原因。区分响应取得时间、来源观测时间、声明 `appliedAt`。健康 `unknown|missing|degraded|reconciling|healthy` 与新鲜度独立；部分来源缺失不能抹掉已确认降级。未知/不支持字段不能默认填零。

当前节点字段可能使用 `omitempty`：成功采集的某些零指标会被省略，而 `/jsz` 失败也可能留下零值结构。因此仅凭字段缺失/默认值不能区分实测零和数据不可用。监控目标 total=0 时的 `available` 不证明集群健康；total 统计配置的监控端点。通用解释这些指标前，应提供 varz/routez/jsz 逐来源证据。30 秒陈旧阈值、10 秒正常刷新是控制台显示策略，不是 Broker 健康阈值。失败保留最后成功值；资源/查询变化后忽略旧响应；隐藏页暂停/恢复不能伪装新鲜。

### C-05 · 预览、写入阶段与协调

**候选更新：**`POST /api/v1/queues/{queue}/preview` 和真实编辑器接入已实现，见[预览语义及验证](webui-preview.zh-CN.md)。预览只读且要求 operator 权限；阻断计划以建议性 200 响应返回。可选的类型化[接收端单次尝试阶段/影响](webui-mutation-evidence.zh-CN.md)现已实现、展示及导出。它们不解决原始请求的持久结果，也不授权重放；不能据此关闭 C-05。

拟新增仅 operator 可用的 `POST /api/v1/queues/{queue}/preview`，输入完整 Queue 文档及原创建/编辑前置条件，返回规范文档、计划、影响操作、原 ETag/内容版本、警告和观测时间。**预览不得创建/修改 Stream、Consumer、元数据或审计意图。** 草稿、原始版本、能力/Schema 任一变化都使预览失效。实际应用独立重验；预览不是资源预留，也不冻结实时计数。

保留现有条件 PUT。错误兼容增加可信的阶段/影响元信息，例如 `phase=validation|audit_intent|precondition|apply|audit_outcome`、`effects=none|possible|confirmed`，已知时提供冲突种类。只有被证明时才返回 none，超时歧义仍为 possible。未经独立版本决策，不把 200 改成异步任务协议。应通过类型化字段消除歧义，不让浏览器解析错误文本。

删除仍是独立流程：精确名称输入确认、原 ETag、默认 force 关闭、归属及建议性影响证据。现有后端阻断非本 Queue 拥有的 Stream、以及未 force 的非空 Stream，但不提供原子“仍为空/无 Consumers”保证，UI 不得承诺。必须验证预览后新数据进入、Stream 删除后其他步骤失败。本文不授权执行破坏性 API。

### C-06 · 关联资源与精确 Consumer 查询

拟新增 `GET /api/v1/queues/{queue}/consumers` 和 `GET /api/v1/streams/{stream}/consumers/{consumer}`。前者在筛选/分页前解析声明计划和归属元数据，分别返回预期身份和观测状态，主 Consumer 与额外优先级 Consumer 各出现一次。预期缺失资源身份已知但指标未知；不能悄悄把额外资源当托管资源。后者是精确、有界的读取，不依赖当前分页或全账号枚举；路由不匹配时返回缺失，不能匹配其他 Stream 的同名 Consumer。

**v0.2 实现更新：**后一个路由现已在源码注册；Queue 范围的关联集合仍为提案。见[实现及验证记录](webui-consumer-detail.zh-CN.md)。这不代表 C-06 全部完成，也不改变 D-05 访问策略。

**v0.3 更新：**Queue 范围关联集合也已注册，区分预期/观测、明确元数据归属，先按 q/mode 筛选再按名称排序分页，并限制枚举数量、核对声明版本。限制与错误行为见上述记录。全局列表查询、真实环境验收和实际 UI 接入仍待完成；C-03 与 WP-02 均未全部完成。

DLQ 依赖与当前 Queue 自身 Stream 成员分离。所有适配保留优先级省略与显式零；不能用普通声明 Subjects 代替生成优先级 Subjects。外部 Stream 的 Consumer 没有 Queue 归属可以是合法状态，不能据此判为无效资源。

### C-07 · 关联审计查询

在声明支持后扩展资源种类/名称、操作者、请求 ID 的精确筛选和 UTC 时间区间（含起点、不含终点），拟新增按事件 ID 精确查找。通过 **`intentId`** 配对结果与意图，不能只靠 request ID，因为调用方可复用关联 ID。新到旧浏览需要不透明游标/高水位边界、稳定序号排序，并定义保留清理造成空洞、新事件、无效游标的行为。筛选/查询必须有界或有索引，不能在浏览器扫描全部审计。如果精确筛选总量成本不可接受，应明确返回未知而非伪造总量。

当前意图 `revision` 为 `create` 或原 KV 版本；应用结果（含失败尝试）可能替换成期望内容版本；删除结果可能继续保留原前置版本。旧版本字段必须按阶段解释。提议新增独立 `expectedKvRevision`、`desiredContentRevision`，不能宣称它们就是最终提交版本。意图/结果缺失、禁止访问、存储不可用与无事件不同。审计 Stream 确实不存在时当前返回空页；空历史不证明保留期之外或存储缺失前从未发生修改。

## 5. 验收矩阵与实施顺序

下列编号是**必需用例**，不表示已全部实现。现有测试只能证明其实际覆盖的行为。

| 用例 | 必须验证 | 契约 |
| --- | --- | --- |
| API-01 | 受保护匿名读取拒绝，本地演示例外明确；探针/metrics 按独立策略执行 | C-01 |
| API-02 | operator/auditor/无效 Token/能力停用不同；401 不自动解释成过期 | C-01 |
| API-03 | 未知部署不根据可达节点数选择 R1/R3；支持值与资格值分开 | C-02 |
| API-04 | Queue 文档往返保留 labels、bindings、DLQ、省略/零优先级及精确大整数 | C-02/05 |
| API-05 | 201 个资源，第一页外也能搜到；总量代表完整查询且排序稳定 | C-03 |
| API-06 | 无效过滤器拒绝，旧服务不支持时不能伪装已筛选 | C-03 |
| API-07 | 末页条目删除、资源变化、取消和迟到响应不展示错误资源 | C-03/04 |
| API-08 | 零监控目标、来源不可用、实测零不能合并为健康/零 | C-04 |
| API-09 | 在线未同步副本、Leader 证据缺失、陈旧成功、混合失败保持区别 | C-04 |
| API-10 | 优先级 0–7 包含同一 Stream 全部八个 Consumer，预期缺失身份不带伪造计数 | C-06 |
| API-11 | 精确 Consumer 查询不依赖分页，其他 Stream 同名资源不能匹配 | C-06 |
| API-12 | 预览不改变 Broker 资源、元数据、审计意图数量 | C-05 |
| API-13 | 原始带引号 ETag 保留，noop 可不变版本，无 ETag 的 200 触发重读 | C-05 |
| API-14 | 创建冲突、版本变化、锁竞争、名称不符、安全阻断的恢复不同 | C-05 |
| API-15 | 重复提交派发只产生一次客户端请求，不把重复请求 ID 宣称为幂等 | C-05/07 |
| API-16 | 审计意图失败零后端写调用；审计结果失败可能发生在已完成写入后 | C-05/07 |
| API-17 | Broker 部分写或响应丢失保留不确定性，不自动重放或宣称回滚 | C-05 |
| API-18 | 冲突/权限恢复保留草稿并独立复核；清会话警告后删除草稿 | C-01/05 |
| API-19 | 删除影响预览后进入的新消息不受声明 ETag 保护 | C-05 |
| API-20 | 审计按 intent ID 配对，按阶段解释版本，结果缺失保持未决 | C-07 |
| API-21 | 审计插入/保留空洞下分页行为明确；先筛选再分页，总量不完整时标注 | C-07 |
| API-22 | 文档响应码、报文、响应头、Schema 与 handler 一致，不只比对路由 | 全部 |

实施顺序：（1）明确 D-05 暴露策略及部署信息来源；（2）版本化 Schema/能力及精确关联读取；（3）查询/观测契约；（4）不写入的预览及类型化写入错误；（5）有索引的关联审计；（6）真实 API UI 接入及新候选版回归。这不是部署或替换冻结候选版的授权。

当前 [OpenAPI 测试](../api/contract_test.go)验证嵌入、路由清单、operation ID、引用，不验证完整响应语义。OpenAPI 使用通用响应对象，遗漏部分实际 403/能力停用 404，没有完整描述 ETag/关联信息或旧 info 错误。应**随实现**更新发布 Schema 和 handler/Schema 等价测试，不能将提案接口先写成已存在能力。

## 6. 初始 v0.1 规格验证（历史记录）

阅读本地 handler、模型、协调、条件写入/锁、审计、监控、现有 UI 适配器及 OpenAPI 测试。执行本地非缓存测试：

```powershell
go test -count=1 ./management/internal/api ./management/internal/jetstream ./management/internal/monitoring ./internal/topology ./api
```

全部五个包通过，只验证现有测试，不证明未来扩展、部署安全或生产规模。现有测试明确覆盖 KV ETag、关联审计意图/结果、写前意图失败、写后结果失败、角色授权、接口路由文档。未使用 SSH，未改容器，未使用凭据、实时写入或新增公开接口。
