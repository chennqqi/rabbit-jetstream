# WebUI 运维告警

[English](webui-operational-alerts.md) | [简体中文](webui-operational-alerts.zh-CN.md)

状态：当前开发候选已实现 WEB-026。它不验收此前冻结的 rc.2 运行时，也不构成发布批准。

已认证的 operator 和 auditor 可以打开 `/admin/alerts`。管理服务读取固定的 Prometheus `/api/v1/rules?type=alert`，且只投影 `deploy/observability/alerts.yml` 中提交的六个告警名称。浏览器不能提交规则名、PromQL、上游 URL 或凭据。响应展示来源、阈值表达式、配置持续时间、严重级别、当前 firing／pending／inactive 状态及来源观测时间。

inactive 只表示“当前未触发”。首次观测绝不会被改写为已恢复。管理进程只为这六条规则保留有界状态，并且仅在观察到同一规则从 firing 转为 inactive 后报告 recovered。恢复证据仅属于当前进程，管理服务重启后会清除；它不是持久化告警历史。缺失的仓库定义规则会被明确返回并展示，因此部分 Prometheus 规则集不会看起来像完整覆盖。

本控制台不发送通知。可通过 `RJS_PROMETHEUS_PUBLIC_URL` 单独声明浏览器可访问的 HTTPS Prometheus origin；配置后，UI 只提供其 `/alerts` 页面外链。绝不从内部抓取地址推断外链。带凭据、查询串、fragment 或任意路径的地址会被拒绝；回环 HTTP 仅能配合现有不安全开发开关使用。

2026-09-11 验证：前端 379 项通过，1 项 Windows 符号链接测试按设计跳过；全仓 Go 测试与 vet 已通过，覆盖完整性细化后受影响包再次通过。嵌入式 Chromium 的 135 项真实服务检查全部通过，覆盖固定规则投影、部分覆盖警告、阈值、持续时间、触发、观测恢复、显式外链及“不发送通知”声明。证据：`artifacts/webui-live-r3J5uN/report.json`。本机冻结管理候选 SHA-256 为 `3a913f922b38d589e367e03ae8de5026aec4e565f46f2d8ab70a77180dea9238`。

Docker Compose 配置校验通过。本机没有 Helm，因此 Chart 渲染仍由发布门禁完成。
