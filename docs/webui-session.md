# Verified WebUI identity

[English](webui-session.md) | [简体中文](webui-session.zh-CN.md)

Status: implemented in source, unreleased; partial C-01 delivery. The owner has confirmed D-05; see [resource-read access policy](webui-access.md).

`GET /api/v1/session` verifies the supplied bearer using the same static-token/OIDC authorization as mutation/audit routes. It returns `actor`, `role`, `permissions`, `expires_at` and `resource_read_policy`.

| Verified role | Permission strings |
| --- | --- |
| operator | `resources:read`, `audit:read`, `queue:preview`, `queue:apply`, `queue:delete` |
| auditor | `resources:read`, `audit:read` |

Permissions describe role authorization, not resource existence, availability, ownership, audit persistence, preview success or guaranteed future writes. Every operation still enforces authorization and its own conditions. Static-token actors use the existing audit identity hash; raw tokens are never returned. OIDC actors and expiry come from the verified token, not browser-decoded claims. `expires_at=null` means no verified expiry is available, not infinite validity or an assurance against token rotation/revocation.

Missing/invalid credentials return 401 with a Bearer challenge; an authenticated identity lacking an operator/auditor role returns 403; no authentication configuration returns 404 `session_api_disabled`. A generic 401 must not automatically be labeled “session expired.” OIDC verification errors are not exposed verbatim. A three-second context bounds this identity request. Responses are no-store. Optional browser Authorization Code + PKCE uses public bootstrap/exchange endpoints but creates no cookie or server-side session and exposes no refresh token or revocation endpoint.

`resource_read_policy` reports `authenticated` by default, or `anonymous` in explicit loopback-only demo mode. This identity endpoint always requires authentication. The UI must distinguish an unavailable/disabled identity capability from a verified identity, keep credentials in memory, and erase credentials/drafts explicitly on session clear. Browser clear is not server credential revocation. A browser SSO flow remains separate work.

Verification:

```sh
go test -count=1 ./management/internal/api ./management/internal/auth ./api
go vet ./...
```

Passed locally. Tests cover static roles, missing/wrong credentials, disabled auth, OIDC identity/expiry, absent role, verifier errors, challenge/no-store/no-cookie, no secret disclosure and no mutation/audit calls. The real local OIDC test provider confirms verified expiry propagation along with existing issuer/audience/role/key-rotation tests. Schema property/required fields are checked against the response model. No production UI integration, external IdP qualification, deployment or remote operation is claimed.
