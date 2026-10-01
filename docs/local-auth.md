# Local Account Authentication

[English](local-auth.md) | [简体中文](local-auth.zh-CN.md)

Local accounts are an optional browser-login mechanism. `POST /api/v1/auth/login` returns a short-lived bearer access token. The Admin UI keeps it only in page memory and sends it in `Authorization`; reload and explicit sign-out remove it. Passwords and tokens are never written to `localStorage`, cookies, URLs, or logs.

## Account file

Generate a password hash without placing plaintext in process arguments:

```sh
printf '%s\n' 'use-a-password-manager-value' | rjs-management hash-password
```

Create a protected file outside the source tree:

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

Usernames and tenant IDs accept 1–128 ASCII letters, digits, `.`, `_`, and `-`. Membership roles are `operator` or `auditor`. Passwords use Argon2id (`m=65536,t=3,p=2`); plaintext passwords are rejected. The strict file format rejects unknown fields, invalid or duplicate memberships, trailing JSON, and files over 1 MiB. Version 1 files remain readable and are atomically migrated to version 2 on the first administrative write.

```sh
RJS_LOCAL_ACCOUNTS_FILE=/run/secrets/rjs-local-accounts.json
RJS_LOCAL_AUTH_SIGNING_KEY='at-least-32-random-secret-bytes'
RJS_LOCAL_AUTH_TTL=15m
```

The signing key must be random, at least 32 bytes, identical across replicas, and rotated through a controlled re-login window. TTL must be greater than zero and no more than 24 hours. Token verification checks the current account record, so password, membership role, membership set, enabled state, and platform-administrator changes invalidate older tokens immediately.

At least one enabled account needs `platform_admin: true` for **Tenant access** in the Admin UI. Platform administrators manage local accounts and tenant memberships. Password hashes are never returned. The last enabled platform administrator cannot be disabled, demoted, or deleted.

The account file must be writable for WebUI changes. Mount its containing directory because updates use atomic same-directory replacement. Tenant backends and NATS credentials remain protected startup configuration and are never returned or modified by this API.

Static operator/auditor tokens remain supported for recovery and automation, not as the primary browser login. See [Management multi-tenancy](multi-tenancy.md).
