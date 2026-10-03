# 诊断包

[English](diagnostics.md) | [简体中文](diagnostics.zh-CN.md)

`rjsctl diagnostics collect` 从管理 API 采集某一时段的支持诊断包，不访问 JetStream 数据面的消息。

```bash
rjsctl diagnostics collect \
  --url http://127.0.0.1:8223 \
  --output rabbit-jetstream-diagnostics.zip
```

ZIP 包含健康、就绪、服务／账号、集群、节点、Queue、Stream、控制器及 Prometheus 快照和 `manifest.json`。清单记录采集时间，以及每个已包含文件的 HTTP 状态、字节长度和 SHA-256 摘要。有效且已脱敏的非 2xx JSON 响应会保留，便于分析故障。单个端点失败不影响其余端点采集。Queue 和 Stream 快照只请求第一页，上限为 200；诊断包不是完整账号导出或原子快照。

## 安全与运维限制

- 不覆盖已有输出文件。
- 最终发布在同一目录中为已完成的临时归档原子创建硬链接，采集期间新建的目标文件也会保留。输出文件系统须支持硬链接，否则发布失败，不会降级为可能覆盖文件的重命名。
- 每个响应限制为 4 MiB，以限制本地内存和包大小。
- JSON 字段名包含 authorization、credential、password、secret 或 token 时，字段值替换为 `[REDACTED]`。
- JSON 数字保留精度，包括 uint64／int64 计数及超过 JavaScript 安全整数范围的修订号；脱敏不经过浮点数转换。
- 已知 JSON 来源即使报告错误的内容类型也会解析。无效 JSON 被省略，不会以未脱敏原文归档；清单保留其 HTTP 状态和静态错误，不记录文件、长度或摘要。其余来源继续采集。
- API 响应、清单和请求错误中的绝对 URL 移除用户信息。
- 管理 API 本身也会从报告的 NATS URL 中去除凭据。

向运维信任边界之外分享前，请审阅诊断包。即使凭据已脱敏，Queue 和 Stream 名称、集群拓扑、资源使用和错误消息仍属于敏感运维信息。诊断包不包含消息载荷、NATS 凭据文件或环境变量。

字段名及绝对 URL 脱敏不是任意文本、嵌入 URL 或查询参数的通用秘密检测器。Prometheus 文本不按 JSON 处理。不能仅凭 CLI 采集器就认定浏览器下载已获授权：WebUI 端点仍需有界任务／存储生命周期、访问策略、有效期和访问审计。此次修改不提供该端点。
