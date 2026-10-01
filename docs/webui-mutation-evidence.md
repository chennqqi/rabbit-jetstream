# Mutation receiving-attempt evidence

[English](webui-mutation-evidence.md) | [简体中文](webui-mutation-evidence.zh-CN.md)

The candidate implements the additive failure metadata from C-05 in [API contracts](webui-api-contracts.md). Existing HTTP status, error code and message remain unchanged. This is not a durable outcome ledger, an idempotency protocol, rollback proof or permission to retry.

PUT/DELETE failures after reaching audit-intent processing can include `error.mutation`:

```json
{"schemaVersion":"rjs.mutation-evidence.v1","scope":"receiving-attempt","phase":"audit_intent","resourceEffects":"none","intentId":"0123456789abcdef0123456789abcdef"}
```

| Phase | Resource effects | Meaning |
| --- | --- | --- |
| `audit_intent` | `none` | Audit support or intent persistence failed before invoking the mutation backend. |
| `backend` | `possible` | Backend returned an error; recording the failed outcome succeeded. Includes conflicts; does not assert rollback. |
| `audit_outcome` | `possible` | Backend was invoked, but recording its outcome failed; partial effects or completion are unresolved. |

Effects refer only to Queue/Stream/Consumer and declaration changes in this handler invocation. Audit/lock metadata and earlier transport replays are excluded. `intentId` is omitted when no intent was generated. Its presence does not prove persistence: a publish acknowledgment may be lost. Match an outcome's `intentId` to the intent's `id`; request IDs remain reusable correlation identifiers, not idempotency keys.

Authorization, parsing, precondition and read failures do not acquire invented phase evidence. Absence means no phase information. Unknown versions, scopes, phase/effect combinations or malformed identifiers are ignored by the client. Evidence is advisory display/download data and never changes the mutation state machine. After dispatch, an error remains uncertain even when the receiving attempt reports `none`; readback and audit inspection do not unlock retry. Evidence downloads allowlist these fields, exclude raw error text/credentials, and retain their existing configuration/audit-data sensitivity warning.

## Verification

Backend tests exercise PUT and DELETE with missing audit support, failed intent persistence, failed outcome persistence, backend errors, conflicts, and combined backend/outcome failure. They check actual backend invocation counts and intent/outcome correlation. Read failures have no mutation evidence. Frontend tests cover strict decoding, allowlisted downloads and no retry after `none` on both mutation paths.

Local browser scenario (build candidates locally first):

```powershell
$env:RJS_TEST_RESPONSE_FAULT='attempt-evidence'
npm.cmd --prefix admin-ui run test:live
```

The harness uses isolated local services and its own fixture resources. It substitutes a synthetic `audit_intent/none` error after forwarding a real successful PUT/DELETE, checks that the actual change occurred while the UI remains unknown/locked, and verifies downloads. This is deliberately a client replay-safety scenario, not proof that the real successful handler failed its audit intent. Backend phase mapping is tested independently. Without this environment variable, the original invalid-JSON scenario remains the default.

Durable original-request resolution, idempotent retry, release qualification and embedded-UI promotion remain separate unfinished work.
