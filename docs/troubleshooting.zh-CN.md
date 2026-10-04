# 故障排查指南

[English](troubleshooting.md) | [简体中文](troubleshooting.zh-CN.md)

控制台、API、CLI 与 SDK 中最常见错误的"现象 → 原因 → 处置"。API 错误码是稳定的，请按错误码匹配而非消息文本。

## HTTP / API 错误

| 现象 | 原因 | 处置 |
|---|---|---|
| `401 unauthorized`（API）/ 控制台回到登录页 | 会话过期（Token TTL 15 分钟）或 Token 错误 | 重新登录，草稿仍保留。自动化：检查 `RJS_ADMIN_TOKEN(S)` / `RJS_AUDIT_TOKENS`。 |
| `403 role-denied` | 角色缺少权限（auditor 不能写） | 请平台管理员经租户访问管理授予 operator 角色，或改用 operator 凭据。 |
| `404 read_api_disabled` | 服务端完全未配置认证 | 设置 `RJS_ADMIN_TOKEN(S)` / 本地账户 / OIDC 后重启。 |
| `404 alerts_api_disabled` / 控制台告警页"Prometheus 后端未配置" | 未设置 `RJS_PROMETHEUS_URL` | 配置 Prometheus（见[运维配套清单](operations-companion.zh-CN.md)）；告警通知还需 Alertmanager。 |
| `404 history_api_disabled` / "历史后端不可用" | 同上 | 同上；历史指标共用 Prometheus 后端。 |
| `428 precondition_required` | 缺少 `If-None-Match: *`（创建）或 `If-Match: <kvRevision>`（更新） | 重新读取资源取当前 KV 修订，带 header 重试。控制台自动完成。 |
| `409 conflict` / ETag 不匹配 | 声明或 KV 修订在你编辑期间变化 | 重新读取，在新修订上重新应用修改（控制台：重新打开编辑器）。绝不盲目重提交。 |
| `409 name_mismatch` | URL 名称与文档名称不一致 | 对齐两者。 |
| 删除返回 `409` 且 `requires_force` | Stream 仍含消息 | 确认影响、勾选 force、输入精确 Queue 名称。建议先停止消费并排空。 |
| 删除**结果不确定**（发出请求后报错，控制台锁定重试） | 后端调用后审计写入失败 | 不要盲目重试。按请求 ID 查审计，`nats stream info RJSQ_<名称>` 检查 Stream 是否存在；按[运维配套清单](operations-companion.zh-CN.md)场景 C 处理。 |
| `503 queue_unavailable` / 后端错误 | NATS 或元数据 quorum 故障 | 见 [Incident Triage](operations.zh-CN.md)：检查节点与控制器，绝不同时重启多个成员。 |

## 控制台现象

| 现象 | 原因 | 处置 |
|---|---|---|
| 刚创建的 Queue 徽章显示 缺失/Missing | 控制器尚未对账完成 | 等一个对账周期（约 5 秒）后刷新。持续存在则检查控制器指标 `rjs_controller_leader` 与 blocked 队列数。 |
| 刷新指示器显示过期横幅 | 标签页隐藏、刷新失败或数据超过 30 秒 | 把标签页置前；持续失败则检查管理服务 healthz 与 NATS。 |
| 仅在点击 SSO 后出现"SSO 登录不可用" | OIDC 配置错误或 IdP 拒绝 | 核对 `RJS_OIDC_*` 配置、回调 origin 精确一致、IdP 可达。本地账号登录始终可用。 |
| 告警页显示覆盖不完整 | Prometheus 缺少部分随货规则 | 检查 `deploy/observability/alerts.yml` 是否加载；promtool 校验。 |
| F5 后会话被清除 | 设计如此：Token 仅存页面内存 | 重新登录；草稿会丢失，证据需要会话——先记录再刷新。 |

## SDK（Native Go）

| 现象 | 原因 | 处置 |
|---|---|---|
| 发布失败提示优先级校验 | Priority > Queue `maxPriority` | 发布 ≤ max，或先预览再更新 Queue 声明。 |
| 发布成功但消费者收不到 | 消费的 subject/durable 不对 | 使用 SDK `Consumer`（绑定 `rjs.q.<queue>.p.*` durable）；不要手拼 subject。 |
| 消息重复投递 | at-least-once 设计使然；NAK 或 ack-wait 超时 | 处理器幂等化；保持稳定发布 ID 用于去重。 |
| `Close` 后消息重投递 | 缓冲消息在关闭时被 NAK | 预期行为；关闭前排空，或按重投递设计。 |
| 低优先级饿死 | 预取预算过小 | 调整 `ConsumerConfig.Prefetch`（默认把 256 条均分到各级）。 |

## CLI（rjsctl）

| 现象 | 原因 | 处置 |
|---|---|---|
| `audit list` 要求 `--token` | 审计读取需要显式凭据 | 传入 operator token。 |
| `queue apply` 返回 409 blocked | 计划为 recreate 类或所有权不匹配 | 先 `queue plan`/`diff` 检查；破坏性变更走备份路径。 |
| `backup restore` 拒绝执行 | 目标 Stream 已存在 / 缺 `--confirm RESTORE` | 恢复到空账户（停止写入者与控制器）并传入确认词。 |
| `diagnostics collect` 只含 200 条队列/流 | 设计边界（单响应 4 MiB） | 事故场景足够；完整清单走 API。 |

## 部署 / 本地开发

| 现象 | 原因 | 处置 |
|---|---|---|
| kind rootless 报 "Delegate=yes" 错误 | user@.service 缺委托 | `systemctl set-property user@<uid>.service Delegate=yes` 后重启用户服务。 |
| kind：镜像 ErrImageNeverPull | 镜像未加载进节点 | 逐镜像 `kind load image-archive`（多镜像归档会把共享基础镜像的镜像塌缩）。 |
| 管理服务 `connect to NATS: no servers available` | NATS 未启动或 `RJS_NATS_URL` 错误 | 先启动 NATS；检查 routes 与集群名（JetStream 集群必须 `--cluster_name`）。 |
| `subjects overlap with an existing stream` | 声明 subject 与其他 Stream 冲突 | 修正 Queue subjects 或移除冲突 Stream；控制面会拒绝重叠。 |
| npm test 出现 3 个 Windows 跳过 | Windows 上的 vite SSR 盘符缺陷 | 预期行为：断言由 Playwright e2e 覆盖。 |
| LF/CRLF 警告 | Git autocrlf | 仓库已通过 `.gitattributes` 固定 LF；本地设置 `core.autocrlf=false`。 |
| NATS jsz：stream 配置/副本详情缺失 | jsz 默认不含每流 config 详情；用 nats CLI 获取权威副本数 | `nats stream info <name>`（operator 镜像内），或 jsz 参数加 `streams=true&config=true`。 |

## 仍未解决

`rjsctl diagnostics collect --output bundle.zip`，按 [Escalation Evidence](operations.zh-CN.md) 要求附上（不含消息负载）。
