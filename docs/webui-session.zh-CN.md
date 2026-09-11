# WebUI 已验证身份

[English](webui-session.md) | [简体中文](webui-session.zh-CN.md)

状态：源码已实现，尚未发布；C-01 部分交付。负责人已确认 D-05，见[资源读取访问策略](webui-access.zh-CN.md)。

`GET /api/v1/session` 使用与写入/审计路由相同的静态 token/OIDC 授权验证所提供的 bearer，返回 `actor`、`role`、`permissions`、`expires_at` 和 `resource_read_policy`。

| 已验证角色 | 权限字符串 |
| --- | --- |
| operator | `resources:read`、`audit:read`、`queue:preview`、`queue:apply`、`queue:delete` |
| auditor | `resources:read`、`audit:read` |

权限表示角色授权，不代表资源存在、可用性、归属、审计持久化、预览成功或未来写入保证。每次操作仍校验权限及自身前置条件。静态 token 的 actor 沿用审计身份哈希，绝不返回原 token。OIDC actor 和过期时间来自验证后的 token，不来自浏览器解码声明。`expires_at=null` 表示没有可用的已验证过期时间，不代表永久有效或不会因轮换/撤销而失效。

凭据缺失/无效返回 401 和 Bearer challenge；已认证但无 operator/auditor 角色返回 403；未配置认证返回 404 `session_api_disabled`。通用 401 不能自动标为“会话过期”。OIDC 验证内部错误不原样暴露。身份请求上下文限制三秒，响应为 no-store。可选浏览器 Authorization Code + PKCE 使用公开引导／交换端点，但不创建 Cookie 或服务端会话，也不暴露 refresh token 或撤销端点。

`resource_read_policy` 默认返回 `authenticated`，仅显式回环演示模式返回 `anonymous`。身份接口始终要求认证。UI 必须区分身份能力不可用/禁用和身份已验证，凭据仅存内存，清除会话时明确清除凭据/草稿。浏览器清除不等于服务端凭据撤销。浏览器 SSO 流程仍是独立工作。

验证：

```sh
go test -count=1 ./management/internal/api ./management/internal/auth ./api
go vet ./...
```

本地通过。测试覆盖静态角色、缺失/错误凭据、禁用认证、OIDC 身份/过期时间、无角色、验证器错误、challenge/no-store/no-cookie、无凭据泄漏和零修改/审计调用。本地真实 OIDC 测试提供方验证过期时间传播，并沿用签发方/受众/角色/密钥轮换测试。Schema 属性/必填字段与响应模型对照检查。不声称生产 UI 接入、外部 IdP 验收、部署或远程操作已完成。
