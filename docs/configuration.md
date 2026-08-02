# 配置

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| `RJS_NAME` | `rabbit-jetstream` | 实例名称 |
| `RJS_HTTP_ADDR` | `:8223` | 管理 HTTP 监听地址 |
| `RJS_NATS_URL` | `nats://127.0.0.1:4222` | NATS 地址，多个地址以逗号分隔 |
| `RJS_NATS_USER` | 空 | 用户名 |
| `RJS_NATS_PASSWORD` | 空 | 密码 |
| `RJS_NATS_CREDS` | 空 | NATS credentials 文件；设置后优先于用户名密码 |
| `RJS_NATS_TLS_CA` | 空 | PEM CA 文件；设置任一 TLS 选项即强制 TLS，生产必须配置 |
| `RJS_NATS_TLS_CERT` | 空 | PEM 客户端证书；必须与 `RJS_NATS_TLS_KEY` 同时设置 |
| `RJS_NATS_TLS_KEY` | 空 | 客户端私钥文件 |
| `RJS_NATS_TLS_SERVER_NAME` | 空 | 覆盖服务端证书名称校验目标 |
| `RJS_NATS_TLS_INSECURE_SKIP_VERIFY` | `false` | 跳过证书校验，仅限临时故障诊断，不得用于生产 |
| `RJS_NATS_MONITOR_URLS` | `http://127.0.0.1:8222` | NATS monitoring 基础地址，多个节点以逗号分隔 |
| `RJS_ADMIN_TOKEN` | 空 | 启用 apply/delete 写 API 的 Bearer Token；为空时写 API 关闭 |
| `RJS_ADMIN_TOKENS` | 空 | 逗号分隔的 operator Token；全部可 apply/delete 及读取审计，用于重叠轮换 |
| `RJS_AUDIT_TOKENS` | 空 | 逗号分隔的 auditor Token；只能读取审计 API，不能修改 Queue |
| `RJS_OIDC_ISSUER` | 空 | OIDC issuer；设置后启动时执行 discovery，默认要求 HTTPS |
| `RJS_OIDC_AUDIENCE` | 空 | 管理 API 的预期 audience；启用 OIDC 时必填 |
| `RJS_OIDC_ROLE_CLAIM` | `roles` | 包含角色的字符串或字符串数组 claim |
| `RJS_OIDC_OPERATOR_ROLE` | `rabbit-jetstream-operator` | 映射为 operator 的 IdP 角色 |
| `RJS_OIDC_AUDITOR_ROLE` | `rabbit-jetstream-auditor` | 映射为 auditor 的 IdP 角色 |
| `RJS_OIDC_ALLOW_INSECURE_ISSUER` | `false` | 仅本地测试允许 HTTP issuer；生产环境不得开启 |
| `RJS_OTEL_TRACES_ENDPOINT` | 空 | OTLP/HTTP traces 完整 URL；为空时 tracing 关闭 |
| `RJS_OTEL_METRICS_ENDPOINT` | 空 | OTLP/HTTP metrics 完整 URL；为空时 OTLP metrics 关闭 |
| `RJS_OTEL_METRIC_INTERVAL` | `30s` | OTLP metrics 周期导出间隔 |
| `RJS_OTEL_SAMPLE_RATIO` | `0.1` | 父级感知的采样率，范围 0–1 |
| `RJS_OTEL_ALLOW_INSECURE` | `false` | 仅隔离测试环境允许 HTTP collector |
| `RJS_METADATA_BUCKET` | `RJS_META` | Queue 声明使用的 JetStream KV bucket |
| `RJS_METADATA_REPLICAS` | `1` | KV 副本数，仅允许 1、3、5；生产三节点集群应设为 3 |
| `RJS_INSTANCE_ID` | 主机名与进程号 | 管理实例唯一标识；多副本部署必须唯一 |
| `RJS_CONTROLLER_ENABLED` | `true` | 是否参与选主并执行持续 reconcile |
| `RJS_CONTROLLER_INTERVAL` | `5s` | controller 检查周期及单次后端操作超时 |
| `RJS_CONTROLLER_LEASE_TTL` | `15s` | leader 租约；小于两倍检查周期时自动提升为三倍 |
| `RJS_LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |
| `RJS_CONNECT_TIMEOUT` | `5s` | 初次连接超时 |
| `RJS_SHUTDOWN_TIMEOUT` | `10s` | HTTP 优雅退出超时 |

命令行 `--http`、`--nats`、`--name` 会覆盖相应环境变量。HTTP 服务固定限制为 15 秒完整请求读取、30 秒响应写入、60 秒空闲连接和 64 KiB 请求头；上传 Queue 声明仍额外限制为 1 MiB。管理 API、健康检查及错误响应统一使用 `Cache-Control: no-store` 和浏览器安全头，OpenAPI 契约保留 5 分钟公共缓存，Admin UI 静态资源使用自身的严格 CSP 与缓存策略。Monitoring 地址可以包含 HTTP Basic Auth，但管理 API 会移除 URL 中的凭据后再返回；生产环境应使用独立监控网络或 HTTPS。NATS 客户端连接应使用 credentials/NKeys 与 TLS；Compose 配置只用于本地开发和架构演示。TLS 客户端最低使用 TLS 1.2；证书或私钥缺失、CA 无效时服务拒绝启动。上述 `RJS_NATS_*` 认证与 TLS 变量也适用于 `rjsctl` 的 JetStream 迁移命令、影子数据工具和性能测试工具。
