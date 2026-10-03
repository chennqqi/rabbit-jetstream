# Management Multi-Tenancy

[English](multi-tenancy.md) | [简体中文](multi-tenancy.zh-CN.md)

Management tenants are isolated NATS accounts. Each configured tenant has its own NATS connection, metadata bucket, audit stream, monitoring client, reconciliation controller, Consumer index, and diagnostics ownership scope. A local account may belong to one or more tenants. The Admin UI sends the selected tenant as `X-RJS-Tenant` on protected resource requests.

Set `RJS_TENANTS_FILE` to a root-readable secret file outside the source tree:

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

The document is strictly versioned, rejects unknown fields, duplicate/invalid IDs, mixed credentials methods, trailing JSON, and files over 1 MiB. Empty optional values inherit process defaults. Prefer NATS credentials/NKeys and TLS; user/password fields exist for compatibility but should be delivered only through a protected secret file.

All configured tenant connections must succeed before HTTP starts. Readiness without a selected tenant checks every backend. If any tenant fails during startup, all already-opened tenant connections are closed. Account memberships referencing an unknown tenant also prevent startup.

Static recovery tokens and OIDC identities without tenant claims are restricted to the lexicographically first configured tenant. Local accounts are the supported multi-tenant browser identity source in the current release. External OIDC/IdP tenant-claim mapping remains deferred.

The current Prometheus history/alerts backend has no tenant selector. To prevent cross-tenant metric disclosure, startup rejects `RJS_PROMETHEUS_URL` together with `RJS_TENANTS_FILE`; per-tenant Prometheus routing is future work.
