# WebUI 历史指标

[English](webui-metric-history.md) | [简体中文](webui-metric-history.zh-CN.md)

状态：当前开发候选已实现 WEB-017。它不属于此前冻结的 rc.2 验收范围，也不能单独构成发布批准。

Overview 和 Queue 详情页通过需认证的 `GET /api/v1/history` 读取真实范围数据。浏览器只能选择已声明的指标和时间窗标识，不能提交 PromQL、Prometheus URL、凭据、任意标签、起止时间或步长。管理服务将请求映射为固定 PromQL 白名单，以及固定的 15 分钟、1 小时、24 小时时间范围和 15、60、300 秒步长。

Prometheus 仅通过管理服务的 `RJS_PROMETHEUS_URL` 配置；需要认证时使用 `RJS_PROMETHEUS_TOKEN`。Token 绝不返回浏览器。默认必须使用 HTTPS；`RJS_PROMETHEUS_ALLOW_INSECURE=true` 仅限隔离开发环境或 Compose 内部网络，Helm 生产配置校验会拒绝它。重定向、URL 凭据、非预期 origin 路径、超限响应、过多序列／样本、投影后重复身份、未批准标签，以及无效数值或时间顺序都会被拒绝。

曲线在可访问的精确样本表中保留源值。Prometheus `NaN` 样本会显示为缺口，不会伪造成零或插值连线。管理服务运行时间下降会开始新线段，并标注为进程重置。空序列只表示“没有样本／状态未知”，绝不表示零或健康。后端未配置、访问拒绝、不可用／响应不兼容互相区分，并会清除旧证据。

2026-09-11 本机验证结果：

- 前端 375 项测试通过；1 项 Windows 符号链接测试按设计跳过。
- `go test ./...` 与 `go vet ./...` 通过。
- 晋升后的四文件嵌入式 UI 通过制品一致性检查。
- 嵌入式 Chromium 的 134 项真实服务检查全部通过。报告包含固定查询、真实时间范围、缺口和时间窗断言，以及 32 次服务端 Prometheus 请求记录：`artifacts/webui-live-7YEWqc/report.json`。
- 本机冻结的管理服务候选 SHA-256：`3b16a67f94d1bc49d56ef2d184238b5131a1520242a59f369c9b6f44614b79c5`。

Docker Compose 配置校验已通过。本机没有 Helm CLI，因此 Chart 渲染仍需由 Helm／发布门禁验证。Prometheus 可用性、保留期和抓取健康仍由运维负责；本功能不是告警投递服务。
