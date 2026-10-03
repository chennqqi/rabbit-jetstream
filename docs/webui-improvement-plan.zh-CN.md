# WebUI 管理控制台改进计划

状态说明：**已修复** — 已在当前工作树实现；**跟踪中** — 确认为待办并附原因。

对照文档：[webui-improvement-plan.md](webui-improvement-plan.md)。分析范围覆盖控制台前端（`admin-ui/src`）及其服务端 `management/internal/api`。

## 基线评估

该控制台是安全优先的控制平面：先预览后应用、`If-Match` 乐观并发（428）、能力漂移预条件（412）、fail-closed 审计（intent 先于执行落库）、错误响应携带变更证据。前端以显式写操作状态机、URL 编码状态、基于完成批次的刷新调度和真实无障碍基线（跳转导航、焦点管理、`role="status"/"alert"`、axe 门禁 live 套件）与之对应。因此以下改进项集中于一致性、规模与交互打磨——均不改变写操作安全语义。

## 一、功能设计

| # | 问题 | 状态 | 说明 |
|---|---|---|---|
| F-1 | 登录限流在反代后失效：`loginLimiter` 按 `RemoteAddr` 计数，同一代理后所有客户端共享失败桶（20 次/分钟），单个行为者可锁死所有人。 | **已修复** | 新增 `RJS_TRUSTED_PROXY_HOPS`（默认 0，宽松解析），按跳数从 `X-Forwarded-For` 右侧解析客户端 IP，用于限流与审计 `SourceIP` 归属；链长不足或非 IP 回退对端地址。见 `management/internal/api/handler.go`（`clientIP`）与 `docs/configuration.md`。 |
| F-2 | 过期 Token 与无效 Token 不可区分：`authorize` 吞掉了 verifier 错误，前端只能靠本地时钟猜测过期。 | **已修复** | 本地 verifier 返回 `identity.ErrTokenExpired`，API 映射为 401 `token_expired`；控制台仅对该明确码标注"过期"，通用 401 仍不推定过期。契约更新见 `docs/webui-api-contracts.md`。 |
| F-3 | 列表端点对全量后端枚举做内存子串过滤与排序；"10k Queue / 100k Consumer" 规模目标从未实测。 | **已完成（模拟范围）** | 查询、采集、鉴权 HTTP 和最大页面 DOM 门禁覆盖 1 万/10 万规模；Docker 另通过 100 万条三副本消息。保留已测量的代次绑定 offset 契约，不引入没有证据必要性的通用游标。真实精确候选种群属于发布验收。 |
| F-4 | 全局 Consumer 索引的归属架构决策未决；Queue 行级健康聚合存在 N+1 风险。 | **已完成** | JetStream/声明保持权威；management 只拥有按租户隔离、有界、可重建、单副本本地的投影。隔离、代次不匹配、陈旧保留及副本语义已有契约测试；多副本分页前须使用粘性路由或共享投影。 |
| F-5 | `handler.go` 是 god-file（路由+鉴权+审计+错误映射+分页）；严格 JSON 解码样板逐 handler 重复；响应混用 typed struct 与 `map[string]any`。 | **已完成（结构）** | 已拆分 `routes.go`、`authz.go`、`audit.go`、`errors.go` 和 `decode.go`。生成响应类型已从全局 Consumer 路径开始采用；其余通用响应属于增量清理，不是门禁。 |
| F-6 | 访问账户 handler 把校验失败与存储失败都压成 409 `account_*_rejected`。 | **已完成** | 存储返回 identity 哨兵（`ErrAccountExists/NotFound/Policy/Validation/StoreUnavailable`）；API 分别应答 404 `account_not_found`、409 `account_already_exists`、409 `account_policy_rejected`、400 `invalid_account_request`、503 `local_account_store_unavailable`，未识别失败保留历史 409 桶。见 `access_account_errors_test.go`。 |
| F-7 | 无推送通道：审计/告警页靠轮询。 | **已完成** | 带鉴权、按租户隔离的 fetch/SSE 失效提示驱动审计/告警的有界刷新，具备重放、心跳、退避、预算和慢客户端淘汰。代理、HTTP/2、Linux race 及真实 Docker/Chromium 路径通过。告警检测仍为有界轮询，不冒充精确通知。见 `docs/webui-sse.zh-CN.md`。 |
| F-8 | OpenAPI 已内嵌但未用于生成 Go/TS 类型。 | **已完成（生成基线）** | 固定版本、可复现的 Go/TS 生成、离线 Queue 打包、生成物入库、CI 漂移和 uint64/bigint 保护均完成。全局 Consumer 200 路径已使用生成模型并通过契约等价测试。其余端点采用属于增量类型化，不再是设施门禁。见 `docs/openapi-codegen.zh-CN.md`。 |

## 二、交互设计

| # | 问题 | 状态 | 说明 |
|---|---|---|---|
| I-1 | 破坏性流程使用原生 `window.confirm/alert/prompt`：无法承载证据安全指引、阻塞渲染器、不可样式化——控制台最重要的文案活在最弱的表达介质里。 | **已修复** | `dialog.jsx` 提供基于 Promise 的页内 `alertdialog`（confirm / alert / prompt 三模式、危险确认按钮着色、Escape 取消、焦点还原、单宿主单对话框语义）。8 处原生调用全部替换（清除会话、切换租户、SSO 登出、创建恢复/归档/导入、编辑器交接、路由模式切换、标签增改删、账户删除）。live 套件同步从 `page.once("dialog")` 改为页内对话框交互。 |
| I-2 | 全局写锁（`submitting`/`uncertain`/`inspecting`）在用户离开对应 Queue 页后不可见。 | **已修复** | `evidence-indicator.jsx`：顶栏徽标列出保留的草稿、删除与证据（含阶段标签与深链）；写锁期间徽标转为危险色。 |
| I-3 | URL 驱动的租户切换在导航中途用阻塞式原生对话框确认。 | **已修复** | 路由 effect 现在立即回退 URL 并改为页内确认；干净控制台仍同步切换、不弹框。 |
| I-4 | Queue 详情缺面包屑、资源头排布不一致（design-qa P1）。 | **已完成** | 共享 `resource-breadcrumb.jsx` 覆盖 Queue 详情/编辑/删除三页、Stream 详情、Consumer 详情与连接详情；Queue 详情的编辑/删除链接与页签改用共享 `RouteLink`。完整资源头构成（刷新位置、密度）仍属 S-4 视觉遍。 |
| I-5 | 表格缺 `<caption>`；刷新总数已经由 `role="status"` 播报。 | **已完成** | Queue/Stream 列表、节点列表、Stream Consumer 集合、Queue Consumer 集合、全局 Consumer、审计窗口、连接、租户账户各表均已补 caption。 |
| I-6 | 复制便利性：请求 ID 与证据 JSON 需手动选中文本。 | **已完成（请求 ID）** | `copy-value.jsx` 接入编辑器预览/提交的请求 ID 与删除请求 ID。证据 JSON 导出随证据指示器后续**跟踪**。 |
| I-7 | 刷新接线、旧数据判定、路由链接点击处理在多页复制粘贴。 | **已完成** | `use-refresh-loop.mjs` 与 `stale-evidence.jsx` 覆盖全部轮询页面。共享 `RouteLink` 覆盖路由所属的面包屑、页签、集合行与摘要链接；其余 anchor 是有意保留的原生/外部链接。 |

## 三、样式风格

| # | 问题 | 状态 | 说明 |
|---|---|---|---|
| S-1 | 仅 4 个 CSS 自定义属性；30+ 硬编码色值、近重复的危险/警告色、旧调色板残留。 | **已修复（token 化）** | `shell.css` 现定义语义 token 层（表面、墨色、导航、线条、状态、焦点），所有规则引用 token。颜色与已审计构建保持字节级一致——近重复色的归并有意留给视觉 QA 遍，以保持验收基线稳定。 |
| S-2 | 字体栈把 `Arial` 排在 `system-ui` 前，且缺 macOS/Linux 中文回退（仅微软雅黑）。 | **已修复** | `system-ui` 优先，补 PingFang SC / Noto Sans CJK SC；声明 `color-scheme: light`。 |
| S-3 | 死规则：`#172b4d` 按钮色被后续覆盖；蓝色焦点环始终被绿色覆盖。 | **已修复** | 已删除；保留单一 `:focus-visible` token 规则。 |
| S-4 | `docs/webui-selected-design.md` 的间距刻度（4/8/12/16/24/32）与密度目标未强制；无共享 Table/Pagination 组件。 | **已完成** | Queue/Stream 与全局 Consumer 集合共用原生表格和分页基础组件，具备紧凑单元格、带 scope 的表头、caption、范围实时播报及可聚焦溢出区。修复 1280px 筛选断点后，96 项 Chromium 矩阵及 Firefox 溢出/焦点检查通过。见 `docs/webui-s4-visual-qa.zh-CN.md`。 |
| S-5 | 图标使用不一致（仅导航用 Tabler）。 | **已完成（决策已定）** | 决策已写入 `admin-ui/README.zh-CN.md`：图标仅用于导航以保证双语清晰；扩展到操作按钮属于显式设计决策。 |
| S-6 | 无 favicon（持续 404）。 | **已完成** | 内联 SVG data-URI；不新增资产文件，嵌入资产白名单不变。 |
| S-7 | 无暗色模式。 | **已完成** | `prefers-color-scheme: dark` 重指向 token 层（含专用 `--brand` token）；在暗色偏好下用 axe 色彩对比度探针验证登录、总览、队列、节点、设置——零违规。新增色值必须留在 token 层。 |

## 四、其他问题

| # | 问题 | 状态 | 说明 |
|---|---|---|---|
| O-1 | `/metrics` 无鉴权（有意设计）。 | **沿用既有文档** | 已有文档；部署检查清单须保持网络隔离提醒。无代码变更。 |
| O-2 | `build-candidate/` 与 `dist/` 的 promote 混淆。 | **沿用既有文档** | `admin-ui/README.md` 已说明 promote 流程；`verify:dist` 会明确报错。无需变更。 |
| O-3 | i18n：四套并存的字符串写法；中英漂移风险。 | **已完成** | 用户可见功能文案现均来自功能级目录或带参数的目录格式器。`i18n-inline-debt.test.mjs` 要求内联双语选择为零；`labels-parity.test.mjs` 校验中英结构、非空叶节点、功能归属及未使用的顶层键。正常、空、加载、陈旧、错误、确认和无障碍状态均具备双语文案，并由现有模型、DOM 与浏览器测试保护。 |
| O-4 | 无障碍门禁只覆盖静态 axe 快照；动态播报与焦点还原只有隐式断言。 | **已完成（对话框）** | live 套件现断言确认对话框打开时把焦点移入自身、关闭时把焦点还给触发控件（L-08 对话框行为）；动态错误播报继续由既有 `role="alert"` 断言覆盖。 |
| O-5 | `main.jsx` 路由/租户胶水层回归风险最高且无直接单元覆盖。 | **已完成（设施+首批关键路径）** | 在现有 Node 运行器上新增固定版本的 jsdom、Testing Library 与 user-event。`main-dom.test.mjs` 通过 Vite 加载真实 `main.jsx`，覆盖密码登录、干净租户切换与权限替换、脏路由回退、取消/焦点恢复、确认、证据丢弃及规范租户 URL。Playwright 继续作为浏览器/视觉权威。 |

## 已执行的验证

- 全部受影响 Go 包（`api`、`auth`、`config`、`identity`、`app`）`go test` 通过，含新增测试：`trusted_proxy_test.go`（跳数解析、代理后按客户端分桶、过期 Token 401 码）、`list_query_benchmark_test.go`、config 解析、本地认证过期测试更新。
- 仓库 `go vet ./...` 与 `gofmt` 干净。
- `node --test tests/*.test.mjs`：390 通过（原 389；+labels 对齐、+token_expired 映射；1 个既有跳过）。
- `vite build` + `dist-sync` promote 与 `--check`，嵌入资产与源码一致。
- live 套件 `tests/admin-ui/candidate-live.mjs`（Chromium，真实 `nats-server` + management 二进制）全量通过，证据见 `artifacts/webui-live-*/report.json`。页内对话框另以一次性 60 轮 open/cancel 循环固化验证（0 次点击丢失、0 页面异常）——首次 live 运行暴露的 `settle` 接线缺陷因此被根因修复，这正是该套件存在的意义。

## 完成顺序

已按 `webui-remaining-gates-decisions.zh-CN.md` 的决策顺序执行：F-4/F-3 不变量、O-5、F-8、O-3、S-4、规模证据、F-7。本计划中可由代码和 Docker Desktop 模拟完成的门禁均已关闭。裸机 24 小时验收、精确候选 Canary 及外部发布签署仍是发布门禁，明确不计为 WebUI 实现工作。
