# Admin UI

[English](README.md) | [简体中文](README.zh-CN.md)

## Embedded React console

The approved React/Vite implementation lives in `src/`, independently of the synthetic design prototype. Run `npm ci --ignore-scripts`, `npm test`, then `npm run build` to compile a static candidate into `build-candidate/` with `/admin/` asset URLs. On PowerShell with script restrictions, use `npm.cmd`. The lockfile is included; a clean offline installation passed locally. No runtime frontend server or CDN is required for the final embedded deployment.

The candidate integrates bearer identity, Queue/Stream/Consumer reads, node evidence, audit queries, reviewed Queue mutations, Overview refresh preferences and compatibility metadata. Queue Configuration includes [read-only DLQ diagnosis](../docs/webui-dlq-diagnostics.md); process-wide counters are not per-Queue transfer proof. Credentials and drafts remain in memory. This is not a completed management console: consult the requirement scope and evidence in the development ledger. Run it behind the same origin as the management API; Vite preview alone has no backend. `npm run dev` binds loopback port 18225; `npm run preview` uses 18226. Neither starts or modifies the management server.

The qualified candidate is copied into `dist/`, embedded into `rjs-management`, and served at `/admin/`. The embedded console consumes only the versioned management HTTP API and never connects to NATS directly. Tests parse the built entrypoint and require every hashed script and stylesheet it references to be present and served by the Go handler. Do not hand-edit generated `dist/` assets; rebuild and requalify `build-candidate/`, then promote that exact file set.

The 2026-09-11 promotion was exercised through the actual locally built Go binary in Chromium, including 127 real-service checks and 23 axe snapshots. Remaining production pages, approved architecture items and release qualification are still tracked in the [development ledger](../docs/webui-development.md); embedded promotion alone does not complete the console or approve a release. Default authenticated resource reads are defined by the [access policy](../docs/webui-access.md).

Run `make test-admin-ui` for the current React standalone Compose browser gate. It runs authentication, reviewed Queue create/update/delete, session, mobile failure-state and axe checks in both Chromium and Firefox. Run `pwsh -NoProfile -File tests/admin-ui/run.ps1 -DeploymentProfile cluster` for the same gate against the official three-node cluster manifest; each mode asserts the matching server-declared profile. The local isolated harness can verify the Go-served assets with `RJS_TEST_EMBEDDED_UI=1` and an explicitly selected freshly built management binary.
