# Management Credential Rotation

The management API has two static roles:

- `operator`: Queue apply/delete and audit reads. Configure comma-separated credentials in `RJS_ADMIN_TOKENS`; the legacy `RJS_ADMIN_TOKEN` is also an operator credential.
- `auditor`: audit reads only. Configure `RJS_AUDIT_TOKENS`.

Tokens are compared in constant time and are never written to audit events. Use distinct, randomly generated values from a secret manager. Environment variables are a deployment transport, not a secret store.

## Zero-downtime rotation

1. Add the new token alongside the old token in `RJS_ADMIN_TOKENS` or `RJS_AUDIT_TOKENS`.
2. Roll all management replicas and verify both credentials against `GET /api/v1/audit`; verify the new operator with a non-destructive idempotent Queue apply.
3. Update clients to use the new credential.
4. Remove the old credential, roll all replicas again, and verify it returns `401` while the new credential still succeeds.

For Helm with `auth.existingSecret`, use optional comma-separated Secret keys `admin-tokens` and `audit-tokens`. Updating a Secret does not update process environments; run a rolling restart after each overlap/removal change. Keep `admin-token` during compatibility migration because it remains a required chart key.

Static roles are the production baseline, not identity federation. Restrict management network access and use TLS at the ingress. OIDC integration remains a separate roadmap capability.
