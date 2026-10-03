# WebUI 鉴权事件流

[English](webui-sse.md) | [简体中文](webui-sse.zh-CN.md)

`GET /api/v1/events` 是只读、按租户隔离的 SSE 失效提示流。WebUI 使用 `fetch`，因此页面内存中的 Bearer 凭据和 `X-RJS-Tenant` Header 沿用普通 API 策略。凭据不会进入 URL、浏览器持久化存储、Cookie 或原生 `EventSource` 状态。

事件只要求客户端执行有界权威刷新，不代表变更结果或精确通知。审计 intent/outcome 持久化后发送 `audit`。单进程共享的 Prometheus 监视器仅在固定规则投影变化后发送 `alerts`；第一次成功观察只建立基线。告警采集仍是 15 秒一次的有界轮询，不是精确投递。

每个租户拥有独立单调 ID 和 256 条实例内重放窗口。首次连接从当前头部开始。重连携带最后一个已完整处理的 ID；无法重放时返回 409，客户端刷新两类资源后不带 ID 重连。退避从 1 秒增长到 30 秒，服务端每 15 秒发送注释心跳。

连接预算为每 actor 2 条、每租户 32 条、每实例 256 条。超出准入预算返回带 `Retry-After` 的 429；填满 16 条事件缓冲区的慢客户端会被关闭。management 仍不进入消息数据路径。

## 验收证据

2026-09-13，Go 测试通过鉴权、非法输入、租户隔离、首次从头部开始、重放/过期、连接预算、慢客户端淘汰、关闭、反向代理 flush 及 HTTP/2。相同重点套件在 Docker Desktop 一次性 Linux Go 容器中通过 race detector。Admin UI 测试覆盖分片解析、64 KiB 上限、非法数据、内存 Header、重放、重连和退避。

完整 Go 测试及 `go vet ./...` 通过。全部 404 项 Admin UI 测试中 403 项通过，1 项因 Windows 符号链接能力跳过，0 失败。生产构建、候选提升和嵌入 dist 校验通过。隔离 Docker Desktop 的真实变更产生了租户事件及匹配的持久审计证据。无头 Chromium 加载嵌入式审计页且 URL 无凭据，真实 intent/outcome 事件使审计读取次数由 1 增至 3。

反向代理必须关闭响应缓冲，并允许超过心跳周期的空闲时间。真实部署代理配置和容量仍属于精确候选环境验收；实现和可复现的本地设施已完成。
