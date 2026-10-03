# OIDC 联邦认证

[English](oidc.md) | [简体中文](oidc.zh-CN.md)

管理 API 同时接受静态 Bearer Token 和经过验证的 OIDC ID Token。服务启动时发现提供方及其 JWKS；每次请求都会校验签名、issuer、audience、有效期及配置的角色声明。遇到未知签名 `kid` 时会刷新提供方密钥缓存。

## API Bearer 校验

```sh
export RJS_OIDC_ISSUER=https://id.example.com/realms/platform
export RJS_OIDC_AUDIENCE=rabbit-jetstream-management
export RJS_OIDC_ROLE_CLAIM=roles
export RJS_OIDC_OPERATOR_ROLE=rabbit-jetstream-operator
export RJS_OIDC_AUDITOR_ROLE=rabbit-jetstream-auditor
```

`operator` 可预览、应用、删除 Queue 并读取审计记录；`auditor` 只有运维信息和审计只读权限。有效 Token 未映射到任一角色时返回 403，签名或声明无效时返回 401。审计身份使用 `oidc:<issuer>#<sub>`；不会持久化原始 Token 或个人声明。

## 浏览器 SSO

可选的 Admin UI 登录采用 Authorization Code + PKCE S256：

```sh
export RJS_OIDC_BROWSER_CLIENT_ID=rabbit-jetstream-management
export RJS_OIDC_BROWSER_REDIRECT_ORIGIN=https://console.example.com
```

应在公共客户端中精确注册回调 `https://console.example.com/admin/oidc/callback`。浏览器 client ID 必须等于 `RJS_OIDC_AUDIENCE`，两个浏览器变量必须同时配置。UI 只请求 `openid profile`；一次性 state 与 PKCE verifier 临时保存在 `sessionStorage`，回调时立即删除，并在交换前从地址栏移除授权码。管理服务只向发现得到的固定 token endpoint 交换授权码，验证返回的 ID Token，并仅把已验证 Token 返回浏览器；不会创建 Cookie、服务端会话，也不会暴露 refresh token 或 token endpoint。

已验证的 Bearer 只保留在页面内存中；刷新或清除页面后需要重新登录。如果提供方声明 `end_session_endpoint`，UI 会在通过已有草稿、提交中和结果证据安全检查后提供签发方退出；不会猜测提供方专用的退出参数。

生产 issuer、回调 origin 和发现得到的端点必须使用 HTTPS。`RJS_OIDC_ALLOW_INSECURE_ISSUER=true` 只允许隔离的本地 HTTP 测试。管理服务的出站访问应限制到受信 IdP。轮换签名密钥时，应先发布新 JWKS 密钥，再用新 `kid` 签发 Token，最后等旧 Token 过期后移除旧密钥。
