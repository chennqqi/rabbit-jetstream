# 本地账户认证

[English](local-auth.md) | [简体中文](local-auth.zh-CN.md)

本地账户是可选的浏览器登录机制。`POST /api/v1/auth/login` 返回短时效 Bearer Access Token。Admin UI 仅在页面内存中保存 Token，并通过 `Authorization` Header 携带；刷新页面或明确退出都会清除它。密码和 Token 不会写入 `localStorage`、Cookie、URL 或日志。

## 账户文件

通过标准输入生成密码哈希，避免把明文密码放入进程参数：

```sh
printf '%s\n' 'use-a-password-manager-value' | rjs-management hash-password
```

在源代码树之外创建受保护的文件：

```json
{
  "version": "rjs.local-accounts.v2",
  "accounts": [{
    "username": "admin",
    "password_hash": "$argon2id$v=19$m=65536,t=3,p=2$REPLACE_SALT$REPLACE_HASH",
    "memberships": [
      {"tenant": "local", "role": "operator"},
      {"tenant": "audit-zone", "role": "auditor"}
    ],
    "platform_admin": true
  }]
}
```

用户名和租户 ID 接受 1–128 个 ASCII 字母、数字、`.`、`_` 和 `-`。成员角色为 `operator` 或 `auditor`。密码使用 Argon2id（`m=65536,t=3,p=2`）；拒绝明文密码。严格文件格式会拒绝未知字段、无效或重复的成员关系、尾随 JSON，以及超过 1 MiB 的文件。版本 1 文件仍可读取，并在首次管理写入时原子迁移为版本 2。

```sh
RJS_LOCAL_ACCOUNTS_FILE=/run/secrets/rjs-local-accounts.json
RJS_LOCAL_AUTH_SIGNING_KEY='at-least-32-random-secret-bytes'
RJS_LOCAL_AUTH_TTL=15m
```

签名密钥必须随机生成、至少 32 字节、在所有副本间保持一致，并通过受控的重新登录窗口轮换。TTL 必须大于零且不超过 24 小时。Token 验证会检查当前账户记录，因此密码、成员角色、成员集合、启用状态或平台管理员标记变化后，旧 Token 会立即失效。

至少一个已启用账户需要设置 `platform_admin: true`，Admin UI 才会显示“租户访问管理”。平台管理员负责本地账户和租户成员关系。API 永不返回密码哈希。最后一个已启用平台管理员不能被禁用、降权或删除。

若要通过 WebUI 修改账户，账户文件必须允许管理进程写入。更新采用同目录原子替换，因此应挂载文件所在目录。租户后端和 NATS 凭据仍是受保护的启动配置，本 API 不会返回或修改它们。

静态 operator/auditor Token 继续支持恢复和自动化，但不作为主要浏览器登录方式。参见[管理面多租户](multi-tenancy.zh-CN.md)。
