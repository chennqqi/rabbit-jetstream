# WebUI authentication, tenancy, and desktop review

[English](webui-auth-tenancy-pc-review.md) | [简体中文](webui-auth-tenancy-pc-review.zh-CN.md)

Date: 2026-09-11. This is a combined UX/security architecture review of the locally running Docker Desktop instance. It does not approve a new authentication or tenancy architecture.

## Captured flow

1. **Desktop login — needs redesign.** [Screenshot](../artifacts/webui-design-audit-current/01-login-desktop.png). The centered width is readable, but asking a person for a long-lived Bearer Token is an operator recovery workflow, not a normal account login. The green input fill has no explained meaning. A newer UI against the running older backend also turns an unrecognized OIDC-config 404 into a persistent red SSO error.
2. **Desktop Queue list — structurally usable, poor desktop density.** [Screenshot](../artifacts/webui-design-audit-current/02-overview-desktop.png). Navigation and current location are clear, but search, sort, page size, and refresh each occupy a full row across a 1,190-pixel content area. The controls dominate the empty result and make scanning slow. Identity and session actions consume excessive top-bar width.
3. **375-pixel Queue list — healthy reflow with excessive vertical cost.** [Screenshot](../artifacts/webui-design-audit-current/03-current-mobile.png). There is no horizontal page overflow, controls remain reachable, and navigation collapses. The same one-control-per-row layout is appropriate here, but it produces a long page. This confirms the defect: mobile defaults were reused on desktop without a desktop toolbar composition.

Screenshot evidence cannot prove keyboard order, screen-reader announcements, contrast ratios, session fixation resistance, CSRF protection, or tenant isolation. Those require executable tests and code review.

## Authentication boundary

`Authorization: Bearer` is a transport mechanism, not inherently forbidden. The current safety rule forbids credentials in URLs, logs, committed files, and persistent browser-readable storage. Manual static-token entry should remain only as an explicitly labeled recovery/bootstrap path.

The commonly seen “password login returns JWT to localStorage” design is not recommended for this management plane: any successful script injection can read and retain that token beyond the page lifetime. The recommended normal login is:

- `POST /api/v1/auth/login` accepts username/password over HTTPS and always returns a generic failure;
- password hashes use Argon2id with bounded parameters and no plaintext recovery;
- successful login sets a short-lived `HttpOnly; Secure; SameSite=Strict` host-only cookie;
- state-changing requests require an origin check plus a session-bound CSRF token/header;
- login, failure throttling, logout, session expiry, and administrative account changes are audited without credentials;
- static Bearer tokens remain a separate, visibly labeled emergency/API credential path.

This introduces a server-recognized session even if the cookie is cryptographically signed and no per-session database row is stored. Key rotation, revocation, timeout, multi-replica consistency, brute-force limits, and recovery must therefore be designed before implementation.

## Multi-tenant boundary

A tenant is an authorization and data-source boundary, not a UI filter. Every request, cache key, background refresh, diagnostic job, audit event, idempotency key, and JetStream client must be tenant-scoped. A user needs an explicit tenant membership and tenant-local role. Cross-tenant negative tests must cover direct API paths, stale browser models, concurrent tabs, downloads, audit queries, and mutation replay.

Two materially different architectures remain:

1. **Single management service, multiple tenant connections.** URLs and APIs carry an immutable tenant identifier; the backend resolves a tenant-scoped NATS credential and service bundle. This provides one console and real tenant switching, but substantially expands credential storage, connection pooling, cache isolation, audit, rate limiting, and failure containment.
2. **One isolated management deployment per tenant.** The reverse proxy selects the tenant deployment; each instance keeps one NATS account and its own credentials. This gives the strongest and simplest isolation but does not provide in-app tenant switching and increases deployment count.

The product request for in-app multi-tenancy implies option 1. It requires explicit approval of the tenant source, user/membership store, credential source, URL/API compatibility strategy, and migration behavior before code changes.

## Desktop remediation

- At widths above 1,100px, compose search, sort, page size, and refresh into a compact grid/toolbar; keep labels visible and allow search to take the flexible column.
- Limit ordinary controls to useful widths instead of stretching every input/select/button to the panel edge.
- Put result count, last-success time, and pagination in a compact table header/footer, with empty state closer to the filtering controls.
- Reduce top-bar identity/session actions to one account menu; keep destructive “clear session” inside it.
- Keep the current mobile stack below the breakpoint. Do not shrink touch targets to achieve desktop density.
- Treat an unsupported OIDC bootstrap endpoint as “SSO unavailable” during mixed-version deployment; show an error only for a recognized configured contract that fails.

## Required decision before implementation

Approve the normal login/session model and the multi-tenant deployment model together. Recommended baseline: built-in username/password with an HttpOnly cookie, static Bearer only for recovery/API use, and single-service tenant-scoped URLs/APIs if in-app tenant switching is required.

## Owner decision and implementation

The owner subsequently approved a different normal-login transport: built-in local username/password authentication returns a short-lived bearer access token, the UI keeps it only in page memory, and every API request carries it in `Authorization`. It is never written to `localStorage`, cookies, or URLs. Static bearer tokens remain a separate recovery/automation path. This later decision supersedes the cookie recommendation above.

The approved tenancy model is the single-service/multiple-connection option. The implementation now uses tenant-explicit browser URLs and `X-RJS-Tenant`, validates membership before backend access, and isolates NATS connections, monitoring, controllers, Consumer indexes, diagnostics ownership, and audit storage. See [Local account authentication](local-auth.md) and [Management multi-tenancy](multi-tenancy.md).
