# OIDC Federation

管理 API 可同时接受静态 Token 和 OIDC ID Token，便于平滑迁移及紧急回退。服务启动时通过 issuer discovery 获取 JWKS；每次请求校验签名、issuer、audience、有效期，并在遇到未知 `kid` 时刷新密钥缓存。

最小配置：

```sh
export RJS_OIDC_ISSUER=https://id.example.com/realms/platform
export RJS_OIDC_AUDIENCE=rabbit-jetstream-management
export RJS_OIDC_ROLE_CLAIM=roles
export RJS_OIDC_OPERATOR_ROLE=rabbit-jetstream-operator
export RJS_OIDC_AUDITOR_ROLE=rabbit-jetstream-auditor
```

`operator` 可 apply/delete Queue 并读取审计；`auditor` 只能读取审计。合法 Token 但缺少所需角色返回 403，签名或声明无效返回 401。审计记录使用 `oidc:<issuer>#<sub>`，不保存原始 Token 或个人资料 claim。

生产 issuer 必须使用 HTTPS，并限制管理 Pod 仅访问受信任 IdP。`RJS_OIDC_ALLOW_INSECURE_ISSUER=true` 只用于隔离的本地集成测试。轮换 IdP 签名密钥时应先发布新 JWKS，再签发新 `kid`，最后等待旧 Token 过期后移除旧密钥。
