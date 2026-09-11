# OIDC Federation

[English](oidc.md) | [简体中文](oidc.zh-CN.md)

The management API accepts static bearer tokens and verified OIDC ID tokens. At startup it discovers the provider and its JWKS, then validates signature, issuer, audience, expiry, and the configured role claim on every request. Unknown signing key IDs cause the provider key cache to refresh.

## API bearer validation

```sh
export RJS_OIDC_ISSUER=https://id.example.com/realms/platform
export RJS_OIDC_AUDIENCE=rabbit-jetstream-management
export RJS_OIDC_ROLE_CLAIM=roles
export RJS_OIDC_OPERATOR_ROLE=rabbit-jetstream-operator
export RJS_OIDC_AUDITOR_ROLE=rabbit-jetstream-auditor
```

`operator` can preview/apply/delete Queues and read audit records; `auditor` has read-only operational and audit access. A valid token without either mapped role returns 403. Invalid signatures or claims return 401. Audit actors use `oidc:<issuer>#<sub>`; raw tokens and personal claims are not persisted.

## Browser SSO

The optional Admin UI login uses Authorization Code with PKCE S256:

```sh
export RJS_OIDC_BROWSER_CLIENT_ID=rabbit-jetstream-management
export RJS_OIDC_BROWSER_REDIRECT_ORIGIN=https://console.example.com
```

Register the exact callback `https://console.example.com/admin/oidc/callback` as a public-client redirect URI. The browser client ID must equal `RJS_OIDC_AUDIENCE`. Both browser variables must be set together. The UI requests only `openid profile`; it stores the one-time state and PKCE verifier in `sessionStorage`, deletes them at callback, and removes the authorization code from the URL before exchange. The management service exchanges the code against the discovered fixed token endpoint, verifies the returned ID token, and returns only that verified token. It does not create cookies, a server session, or expose a refresh token or token endpoint.

The verified bearer remains in page memory. Reloading or clearing the page requires a new login. If the provider advertises `end_session_endpoint`, the UI offers provider logout after the existing draft/in-flight evidence safety checks; provider-specific logout parameters are not inferred.

Production issuers, redirect origins, and discovered endpoints must use HTTPS. `RJS_OIDC_ALLOW_INSECURE_ISSUER=true` permits HTTP only for isolated local testing. Restrict management egress to the trusted IdP. During signing-key rotation publish the new JWKS key first, issue tokens with the new `kid`, then remove the old key after old tokens expire.
