# 评审改进计划

[English](review-improvement-plan.md) | [简体中文](review-improvement-plan.zh-CN.md)

状态：待 owner 批准 · 制定日期：2026-10-01
输入：[运维能力评审（技术经理视角）](ops-review.zh-CN.md) + [管理控制台产品评审（PM 视角）](../admin-ui/design-review.zh-CN.md)
范围：合并两份评审的全部行动项，去重、排期、补验收标准。不改变既有架构决策（D-04 嵌入式资产、D-05 访问策略、管理面不进消息数据路径）。

工作量单位：S ≤ 1 天 · M = 2–5 天 · L = 1–2 周（按单人全时估算，未含评审等待）。
条目编号 A1–A26；来源列引用两份评审的发现编号（P0-1/P1-x/P2-x 为产品评审，D1–D6/场景 x 为运维评审）。

---

## 1. 目标与原则

**目标**：把"3.0/5 的合格声明式控制面"提升为"值班团队敢用、爱用的运维工作台"，且不牺牲两份评审确认过的优势（声明式工作流、语义诚实、双语与无障碍、token 纪律）。

原则：
1. **先修信任，再修观感**：崩溃（P0）与会话半死态先于任何视觉打磨；
2. **架构边界不动摇**：管理面不进消息数据路径；消息级工具走独立受控 CLI/SDK 路线（A23）；
3. **视觉变更全部走 token 层**：新色、新组件必须经视觉 QA 进入 `shell.css` token 层，并通过 S-4 视觉矩阵回归（1440/1280/1024/375 × 双语 × 明暗）；
4. **不改 NATS subtree**；
5. **每项有验收标准**，完成即回填状态（吸取 backlog 未回填的教训，见 D6）。

---

## 2. 里程碑总览

| 里程碑 | 主题 | 条目 | 出口标准 |
|---|---|---|---|
| M0（本周） | 止血 | A1–A4 | 管理进程在无 Prometheus 场景 1 小时存活；会话过期可一键重登；值班 runbook 可用 |
| M1（2–4 周） | 信任与效率 | A5–A15 | 状态徽章上线；免责声明退出页面主体；rc.3 重新冻结 |
| M2（1–2 月） | 驾驶舱与闭环 | A16–A22 | 总览 30 秒回答核心问题；告警从触发到人 < 5 分钟 |
| M3（季度内） | 架构决策项 | A23–A26 | 消息级工具路线拍板并交付首个受控工具 |

依赖要点：A15（rc.3 冻结）必须在 M1 全部条目合入后执行；A19（告警闭环）依赖 A12（per-Queue DLQ 指标）；A23 依赖架构决策文档先行。

---

## 3. M0：紧急修复（本周）

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A1 | `AlertRules` 接收者 nil guard：未配置 Prometheus 时返回"不支持"错误而非 panic；补失败路径测试（无 Prometheus 配置 + 告警 watcher + SSE 订阅，断言进程存活） | P0-1 / D1 | 后端 | S | `go test` 新增用例通过；本地无 `RJS_PROMETHEUS_URL` 启动 + 登录浏览 1 小时进程存活 |
| A2 | 会话过期态修复：token 过期后保留主导航、显示"重新登录"主按钮、隐藏编辑器草稿 JSON；过期提示文案缩短为一句 + 详情折叠 | D4 / 场景 5 | 前端 | S | 实测：token 过期后页面可导航、可一键回登录页；页面不再渲染 `{"name":...}` 草稿串 |
| A3 | 发布《运营配套清单》：Alertmanager 基线配置（webhook/邮件示例）、消息级排障旁路脚本样例（nats CLI/SDK：查消息、死信重放）、`rjsctl backup` 定时任务示例、值班 runbook（积压/死信/删除三场景，含删除不确定结局的人工排查步骤） | D2/D3/场景 1–3 | 文档+SRE | M | runbook 三场景各演练一遍可照做；Alertmanager 配置在本地 Compose 栈验证路由可达 |
| A4 | 部署安全提示：在 A1 合并前，部署文档与 compose 注释标注"必须显式配置 `RJS_PROMETHEUS_URL`" | D1 | 文档 | S | standalone.yml / cluster.yml / 部署文档三处一致 |

---

## 4. M1：快速胜利（2–4 周）

### 4.1 前端批次（依赖 token 层扩展 + 视觉 QA，见 6.1）

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A5 | 状态徽章体系：观测状态、读取状态、告警三档色（token 层新增色阶，经视觉 QA）；待投递超阈值警示色 + 加粗 | P1-2 | 前端 | M | Queue 列表/详情、节点、告警页使用徽章；500 条积压与 0 条在截图中肉眼可区分；S-4 矩阵回归通过 |
| A6 | 数据呈现治理：32 位 hash 截断前 8 位 + 复制按钮；字节数人性化（64.7 KB）；消除 1440px 表格横向溢出（英文 Queue 列表、中文 Stream 列表实测场景） | P2-1 | 前端 | S | 英文 1440px 无横向滚动条；hash 可一键复制全值 |
| A7 | 文案治理：各页顶部免责声明降级为折叠 tooltip/"关于这些数字"入口；"自动刷新 10 秒"改为状态指示器（刷新于 hh:mm:ss · 自动） | P1-5 | 前端 | M | 页面主体不再出现否定句式说明；刷新指示器在断网/恢复时状态正确 |
| A8 | 登录页修复：未配置 SSO 时不渲染红色"SSO 登录不可用"错误（最多折叠说明）；登录按钮升级为主按钮样式 | P2-2/P2-3 | 前端 | S | 无 SSO 部署登录页无红色错误；axe 回归通过 |
| A9 | 导航重组：分组（资源/运维/治理）；"创建 Queue"移入 Queue 列表页头主按钮；"兼容性"降级到设置页或页脚 | P1-3 | 前端 | M | 导航 ≤ 3 组；创建入口在列表页头可达；S-4 移动端矩阵回归通过 |
| A10 | 顶栏权重修正：身份（用户名+租户）平铺展示，"清除本机会话"收进身份菜单；顶栏品牌与侧栏去重 | P2-5/P2-10 | 前端 | S | 顶栏高度下降；身份信息无需展开即可见 |
| A11 | 表单与控件治理：创建表单控件宽度上限 + 双列布局；消费者/审计筛选区收敛为单行工具栏；原生文件上传/select 统一皮肤 | P2-4/P2-7/P2-8 | 前端 | M | 创建 Queue 页一屏可见主 CTA；筛选区高度 ≤ 120px |

### 4.2 后端批次

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A12 | per-Queue DLQ 指标：`rjs_dlq_failed_total` 等增加 queue 标签或新增按队列计数；告警规则同步改写 | D3/场景 2 | 后端 | M | Prometheus 中可按 queue 查询 DLQ 失败；`RabbitJetStreamDeadLetterFailures` 告警能定位队列 |
| A13 | 删除预检补 DLQ 依赖方检查：预检响应与 UI 均列出"哪些队列的 deadLetter 指向它" | D5/场景 3 | 后端+前端 | M | 删除被引用队列时预检显示依赖方并要求显式确认 |
| A14 | 文档卫生：backlog（WEB-017/022/024/026 已实现）状态回填；`management-api.md` 移除"read-only"过时表述；`remaining-release-work.md` 标题日期更新 | D6 | 文档 | S | 三处文档与实现一致；CI 中的文档检查（若有）通过 |

### 4.3 发布

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A15 | rc.3 重新冻结：M1 合入后重建 exact-candidate、跑全量回归（含 S-4 视觉矩阵、axe、playwright 双浏览器门禁）、新 24h soak 证据绑定新 revision | D6 | 发布 | M | 新冻结资产指纹入库；全部门禁绿灯；证据 revision 一致 |

---

## 5. M2：驾驶舱与运维闭环（1–2 月）

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A16 | 总览驾驶舱：健康摘要条（端点/Queue/Consumer + 状态色）、待投递 Top 5 列表（链接详情）、存储趋势图（Prometheus 数据）；总览免责全数收纳 | P1-1 | 前端+后端 | L | 三类用户各 3 人的可用性测试："系统健康吗/哪里积压最重/要不要行动"30 秒内可答 |
| A17 | 声明 vs 观测并排对比：Queue 摘要页两列展示声明值/观测值，不一致标色；原始 JSON 收入折叠区 | P1-6 | 前端 | M | 不一致场景（人工制造 ETag 冲突）可视且可点击到详情 |
| A18 | 会话刷新恢复（安全评审后）：刷新后提供页内一次性重新验证（重输密码静默恢复），token 仍不落盘；或提供可关闭的"记住用户名（仅本机）" | P1-4 | 前端+后端 | M/L | F5 后一次交互内恢复会话；安全评审记录入库；凭据仍不入 localStorage |
| A19 | 告警闭环验证：随货 Alertmanager 配置 + 本地 Compose 栈演练"规则触发 → 人收到通知" | D3 | SRE | S | 演练记录：触发到通知 < 5 分钟 |
| A20 | 积压阈值文档化：`QueueBacklogHigh` 默认 10 万条标注"需按负载调优"，运营配套清单给出按队列调优示例 | 场景 1 | 文档 | S | runbook 包含阈值调优步骤 |
| A21 | 移动端治理：375px 顶栏压缩为单行、横幅可关闭、列表卡片化（替代横向滚动表格） | P2-9 | 前端 | M | 375px 首屏即见数据内容；S-4 移动矩阵回归通过 |
| A22 | 审计体验：时间快捷范围（近 1h/24h/7d）、筛选区单行化 | 场景 6/P2-8 | 前端 | S | 快捷范围生成正确的 from/until；S-4 回归通过 |

---

## 6. M3：架构决策项（季度内）

| # | 条目 | 来源 | 层级 | 工作量 | 验收标准 |
|---|---|---|---|---|---|
| A23 | 消息级工具路线拍板：写决策文档（管理面之外、受控 CLI/SDK 形态：脱敏、审计钩子、只读 peek 优先）；若批准，交付首个工具（只读消息浏览） | D2 | 架构+后端 | 决策 M / 交付 L | 决策文档入库；首个工具支持"按 subject 浏览消息（脱敏）"并有审计事件 |
| A24 | 租户生命周期 API/UI：创建/修改/停用租户不再依赖文件编辑重启 | §2.2 矩阵 | 后端+前端 | L | UI 创建租户→该租户可登录→删除租户（空租户）全程不重启 |
| A25 | 细粒度角色：至少把 `access:manage` 从 operator 拆出（新角色或权限组合） | 安全观察 2 | 后端 | M/L | 新角色无法访问账户管理但可完成队列变更；授权矩阵测试更新 |
| A26 | 集成长尾（按需排序）：事件 webhook 外送、Terraform provider、备份恢复编排（WEB-033） | §6 | 后端 | L | 各自单独立项，本文档不承诺日期 |

---

## 7. 度量与验收总表

| 度量 | 基线（本次评审） | 目标 | 对应条目 |
|---|---|---|---|
| 管理进程稳定性 | 无 Prometheus 场景数分钟内 panic | 7×24 存活（含该场景回归） | A1 |
| 会话过期恢复 | 半死态，需盲点"清除本机会话" | 一键重登 ≤ 2 次点击 | A2/A18 |
| 告警到人 | 无闭环（仅页面投影） | 演练 < 5 分钟 | A3/A19 |
| 总览决策支持 | 三屏文本、零图形 | 30 秒回答三类用户核心问题 | A16 |
| 状态可辨识 | 500 积压与 0 等价 | 徽章三档色，截图可辨 | A5 |
| 免责声明 | 每页主体 1–3 条 | 主体 0 条，tooltip 收纳 | A7 |
| 表格溢出 | 英文 1440px 横向滚动 | 无溢出 | A6 |
| 删除安全 | 不查 DLQ 依赖方 | 预检列出依赖方 | A13 |
| DLQ 可定位 | 全局计数 | 按队列查询 | A12 |
| 发布一致性 | 代码漂移出 rc.2 | rc.3 冻结 + 证据 revision 一致 | A15 |

---

## 8. 风险与边界

1. **视觉回归成本**：A5–A11 触碰大量页面，必须分两批合入（先 token/组件层，后页面应用），每批跑 S-4 矩阵 + axe + 双浏览器门禁，避免一次性大爆炸 diff；
2. **语义诚实不回退**：免责声明收纳不等于删除——"观测非声明""unknown 不造假"的表达必须保留在 tooltip 层，验收时逐条核对；
3. **A18 的安全边界**：任何"刷新恢复"方案不得把凭据写入 localStorage/cookie（C-01 红线），实现前先出安全评审记录；
4. **A23 的架构红线**：管理面不进消息数据路径（不借管理 API 伪装 peek）；受控工具是独立进程/独立授权面；
5. **容量现实**：A16/A23/A24 均为 L 级，若并行只有一个开发，M3 只能二选一，建议优先 A23（运维痛点最大）。

## 9. 建议执行顺序（单开发视角）

```
第 1 周    A1 → A2 → A4 →（A3 与开发并行，由 SRE 承担）
第 2–3 周  A6 → A8 → A5（token 层先行）→ A7 → A10
第 4 周    A9 → A11 → A12 → A13 → A14 → A15（冻结）
第 5–8 周  A16（后端指标先行）‖ A19 → A17 → A22 → A20 → A21
第 9–12 周 A18（安全评审）→ A23 决策 → A24/A25 择一启动
```

## 10. 完成后的回填义务

每个条目完成时：在本文件状态列标注 Done + 日期 + 证据链接（截图/测试/演练记录），与 `docs/webui-development.md` 台账联动；两份评审文档中的对应发现不修改原文（评审是快照）。

### 回填记录

| 条目 | 状态 | 日期 | 证据 |
|---|---|---|---|
| A1 AlertRules nil guard + 回归测试 | Done | 2026-10-01 | `management/internal/prometheus/nil_client_test.go`、`management/internal/api/alerts_nil_backend_test.go`；3 包测试绿 |
| A2 会话过期态修复 | Done | 2026-10-01 | `admin-ui/src/main.jsx` expired 分支重写；浏览器复验：一键重登回到登录页、草稿 JSON 不再渲染 |
| A3 运营配套清单 | Done | 2026-10-01 | `docs/operations-companion.{zh-CN,}.md`、`deploy/observability/alertmanager.yml`、`deploy/observability/prometheus.yml` alertmanager 挂接、`deploy/compose/standalone.yml` alertmanager 服务 |
| A4 部署安全提示 | Done | 2026-10-01 | `docs/configuration.md` Prometheus URL 行更新；`deploy/compose/standalone.yml` + `cluster.yml` 注释；`docker compose config` 校验通过 |
| A5 状态徽章体系 | Done | 2026-10-01 | `admin-ui/src/status-badge.jsx` + `shell.css` token 派生徽章；Queue 列表/详情、Nodes、Alerts 四处应用；浏览器复验 Consistent 徽章可见 |
| A6 hash 截断 + 字节人性化 | Done | 2026-10-01 | `admin-ui/src/format.mjs` + `tests/format.test.mjs`；QueueList/QueueDeclaration 用 CopyValue short；Overview/QueuePanels/StreamDetail 字节人性化；浏览器复验 `955f19ac…Copy` |
| A7 免责声明降级 + 刷新指示器 | Done | 2026-10-01 | `admin-ui/src/DisplayValues.jsx` PageNotes/RefreshStatus 组件；QueueList/Overview/Nodes/StreamDetail 四页应用；浏览器复验 "About these numbers" 折叠 + "Refreshed ... auto every 10s" |
| A8 登录页 SSO 修复 + 主按钮 | Done | 2026-10-01 | `management/internal/api/read_auth.go` oidc/config 公开路径 + 测试；`admin-ui/src/main.jsx` SSO 静默加载 + `.primary-action` 登录按钮；浏览器复验无红色 SSO 错误 |
| A9 导航分组 + 创建移入列表页头 + 兼容性降级 | Done | 2026-10-01 | `admin-ui/src/ConsoleNavigation.jsx` 三组（RESOURCES/OPERATIONS/GOVERNANCE）；QueueList 页头 Create Queue 主按钮；Settings 加兼容性入口；浏览器复验分组结构 |
| A10 顶栏权重修正 | Done | 2026-10-01 | `admin-ui/src/SessionIdentity.jsx` 身份平铺 + 清除会话收进面板；`shell.css` `.identity-flat` + 桌面品牌去重；浏览器复验顶栏显示 `local:pm-admin / operator · local` |
| A11 表单控件治理 | Done | 2026-10-01 | `shell.css` `form.filter-toolbar` 单行布局 + `form.creation-form` 宽度上限；Audit/QueueConsumers/QueueCreate 标记 |
| A12 per-Queue DLQ 指标 | Done | 2026-10-01 | `internal/topology/plan.go` PerQueue 字段 + Bump 方法；`management/internal/jetstream/dlq.go` 三处归因；`management/internal/controller/controller.go` DLQByQueue 累加；`management/internal/api/metrics.go` per-queue 输出；`deploy/observability/alerts.yml` DeadLetterFailures 改写为 `sum by (queue)`；jetstream 测试扩展 |
| A13 删除预检补 DLQ 依赖方检查 | Done | 2026-10-01 | `management/internal/jetstream/delete_preview.go` DeadLetterDependents 字段 + ListDeclarations 扫描；`management/internal/jetstream/delete_preview_test.go` TestDeletePreviewReportsDeadLetterDependents；`admin-ui/src/QueueDelete.jsx` 依赖方行；API 实测 `dependents: ['payments-dlq-source']` |
| A14 文档卫生 | Done | 2026-10-01 | `docs/management-api.md` 移除 read-only；`docs/remaining-release-work.{zh-CN,}.md` 状态日期更新至 2026-10-01；`docs/webui-rabbitmq-requirements.{zh-CN,}.md` backlog 状态回填 |
| A15 rc.3 重新冻结 | Done (soak gate) | 2026-10-03 | 本地门禁全绿（S-4 96 张、Playwright 8/8 含 axe 零违规、Go 契约测试修复）；冻结制品 linux/amd64 传输校验通过（revision 033010ee）；24h 裸金属 soak 实际观测 42 小时：5053 周期 health/queues_api 全 200、修复后零崩溃、管理日志零 error、453,505 条消息持续增长——证据见 `docs/releases/v0.1.0-rc.3-soak-evidence.zh-CN.md`。实际发布 rc.3 仍需 owner 冻结决策 + helm/Kind 集群资格（见 remaining-release-work） |
