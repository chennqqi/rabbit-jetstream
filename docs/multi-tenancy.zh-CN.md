# 管理面多租户

[English](multi-tenancy.md) | [简体中文](multi-tenancy.zh-CN.md)

管理租户以独立 NATS Account 为隔离边界。每个租户使用独立的 NATS 连接、元数据 Bucket、审计 Stream、监控客户端、协调控制器、Consumer 索引和诊断任务所有权。一个本地账户可加入一个或多个租户。Admin UI 在受保护的资源请求中通过 `X-RJS-Tenant` 携带当前租户。

将 `RJS_TENANTS_FILE` 指向源码树之外、仅管理员可读的 Secret 文件：

```json
{
  "version": "rjs.tenants.v1",
  "tenants": [
    {
      "id": "production-a",
      "nats_url": "tls://nats-a.example:4222",
      "nats_creds": "/run/secrets/nats-a.creds",
      "nats_tls_ca": "/run/secrets/ca.pem",
      "monitor_urls": "https://nats-a-1.example:8222,https://nats-a-2.example:8222",
      "metadata_bucket": "RJS_META",
      "metadata_replicas": 3
    }
  ]
}
```

该文件采用严格版本控制，会拒绝未知字段、重复或非法 ID、混用认证方式、尾随 JSON 以及超过 1 MiB 的文件。可选字段为空时继承进程级默认值。生产环境应优先使用 NATS credentials/NKeys 和 TLS；用户名密码字段仅为兼容保留，并且只能通过受保护的 Secret 文件交付。

HTTP 启动前必须成功连接全部租户。未选择租户的就绪检查会检查所有后端。任一租户启动连接失败时，已打开的其他租户连接会全部关闭。本地账户若引用未配置租户，服务同样拒绝启动。

静态恢复 Token 以及不含租户声明的 OIDC 身份只允许访问按字典序排列的第一个租户。当前版本支持通过本地账户实现浏览器多租户身份；外部 OIDC/IdP 租户 Claim 映射仍暂缓开发。

当前 Prometheus 历史与告警后端没有租户选择能力。为避免跨租户指标泄露，服务会拒绝同时配置 `RJS_PROMETHEUS_URL` 和 `RJS_TENANTS_FILE`；按租户路由 Prometheus 属于后续工作。
