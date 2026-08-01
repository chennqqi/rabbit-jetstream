# 配置

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| `RJS_NAME` | `rabbit-jetstream` | 实例名称 |
| `RJS_HTTP_ADDR` | `:8223` | 管理 HTTP 监听地址 |
| `RJS_NATS_URL` | `nats://127.0.0.1:4222` | NATS 地址，多个地址以逗号分隔 |
| `RJS_NATS_USER` | 空 | 用户名 |
| `RJS_NATS_PASSWORD` | 空 | 密码 |
| `RJS_NATS_CREDS` | 空 | NATS credentials 文件；设置后优先于用户名密码 |
| `RJS_LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |
| `RJS_CONNECT_TIMEOUT` | `5s` | 初次连接超时 |
| `RJS_SHUTDOWN_TIMEOUT` | `10s` | HTTP 优雅退出超时 |

命令行 `server --http`、`--nats`、`--name` 会覆盖相应环境变量。生产环境应使用 credentials/NKeys 与 TLS；Compose 配置只用于本地开发和架构演示。

