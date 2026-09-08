# Remaining Release Work

[English](remaining-release-work.md) | [简体中文](remaining-release-work.zh-CN.md)

The project is approximately **92% complete** for its explicitly bounded first release. The remaining 8% is primarily release closure, not missing core Queue functionality.

## Remaining Work

| Area | Share | Required outcome |
| --- | ---: | --- |
| Green CI baseline | 1% | Fix the Linux ownership issue in the RabbitMQ/JetStream shadow-migration harness and pass every CI job without weakening assertions. |
| Final freeze and regression | 2% | Freeze the server and Native SDK revisions, then rerun Go, race, coverage, security, Admin UI browser, management integration, Helm/Kind, fault, recovery, upgrade, and migration gates. |
| Final qualification evidence | 2% | Bind qualification evidence to the exact release commits and immutable image digests. Existing 24-hour results remain valid for their original frozen revisions but do not automatically qualify later runtime or packaging changes. |
| Release artifacts and supply chain | 1% | Produce Linux/AMD64 binaries, immutable container images, the Helm Chart, SBOMs, attestations, license records, and verified `SHA256SUMS`; verify clean installation and rollback. |
| Canary and approval | 2% | Complete the documented 1%, 10%, 25%, 50%, and 100% canary stages, retain health and rollback evidence, obtain owner/on-call approvals, and pass `make verify-release-approval`. |

## Release Sequence

### Progress on 2026-09-08

- Implemented host UID/GID mapping for Linux shadow-migration containers. Evidence keeps production `0600` permissions; the output directory no longer needs world-write access.
- Added a test-only cutover image containing `/bin/cp` and the production CLI. Production remains distroless.
- Added definition migration and shadow migration to the local `Release` regression entry point.
- Passed server/SDK tests, vet, and builds (`artifacts/remaining-release-quick.json`, marked as a dirty development run), plus Linux container race tests for both repositories. Coverage passed: 80.2% overall, 92.3% topology, 96.1% controller.
- Passed the full shadow workflow with existing local test images and the host Go module cache: dual-write/resume, two three-message reconciliations, idempotent cutover, and rollback. A Linux Docker-volume check verified that actual CLI output remains mode `0600`, belongs to UID 1000, and is readable by that UID. This does not establish green current-revision CI.

Follow-up status and remaining conditions:

- Retrying restored fresh NATS/operator builds. Full-build shadow migration (with the host module cache), definition migration, all eight Chromium/Firefox Admin UI tests, and Helm gates passed before the subsequent security dependency update. Current-revision CI still needs to run.
- The refreshed image scan identified [CVE-2026-56854](https://pkg.go.dev/vuln/GO-2026-6303) in `golang.org/x/crypto v0.53.0` and [CVE-2026-84304](https://github.com/grpc/grpc-go/security/advisories/GHSA-vp52-pcj8-j9qc) in `google.golang.org/grpc v1.82.1`. Updated crypto to `v0.55.0` in the server module and image build overrides, its required dependencies, and gRPC to `v1.83.1`. Go tests, vet, Linux race, and the complete security gate passed after both updates. All three rebuilt production images have zero findings under the committed fixed HIGH/CRITICAL vulnerability gate. Coverage (80.2%), eight browser tests, and full-build shadow migration also passed after the crypto update, before the final gRPC update. Frozen-revision full regression remains pending. NATS and management builds now use the operator's module cache and proxy/checksum-mirror defaults without disabling checksum verification.
- `ssh jdcloudremote` succeeded on retry. Use `runuser -l sandbox` with `XDG_RUNTIME_DIR=/run/user/1000` so rootless Podman does not inherit root's runtime/working directories. The existing frozen rc.1 bundle passed native preflight again (13 checksum entries, three OCI archives, and both executable version checks); evidence is `artifacts/native-preflight-20260908.json`, SHA-256 `8ec7878e3eeeafd7c84094b323a46ef40ac241c389ed4d7a8b1d86387f9a6bcc`. It binds server `a703f1d849b98e4c986c44c511e70116d4109bf0` and SDK `ff54e4a8c135c3f477617d4c3768b81eec98f7ae`, not rc.2. Retry intermittent SSH/SCP failures; do not transfer source.
- SDK remains `0.1.0-rc.1` at `b7acd5285656b52756756b8b62e4d9a1e4d68bbc`. Final server/SDK freeze, exact-revision full regression, qualification, and supply-chain artifacts remain pending.
- No application canary target, workload, or responsible signers have been supplied. Actual staged observations and owner/on-call sign-offs are required; synthetic tests cannot replace them.

No `rc.2` or GA release was published. The overall estimate remains unchanged until mandatory gates close.

### Required order

### rc.2 freeze and sole-owner workflow

The rc.2 SDK candidate is frozen on the local `release/v0.1.0-rc.2` branch at `8c63313efc3f9cefb87744358795a633fa55acc2`, in the separate `rabbit-jetstream-go-rc2` checkout. The original SDK checkout is preserved. Server regression must pass `-SDKPath` explicitly so integration and race tests bind this same SDK.

The owner has confirmed that one person covers service, application and on-call responsibilities. The approval template therefore uses a single unsigned `sole_owner` entry. Review all stage/fault/rollback evidence once and sign once after the final observation window. The five automated observation stages and their integrity, health and rollback gates remain applicable; no final approval is inferred from permission to prepare a candidate.

The local rc.2 package embeds BuildKit SBOM/provenance attestations and includes license records. All binaries are built locally in Docker; only frozen artifacts and evidence inputs may be transferred to `jdcloudremote`. Public publisher signing and redistribution terms remain separate from local artifact qualification.

1. Restore a fully green CI baseline.
2. Freeze the final server and SDK revisions.
3. Generate revision-bound qualification and release artifacts.
4. Publish `v0.1.0-rc.2` and execute the staged canary.
5. Promote to `v0.1.0` GA only after the observation period and approval criteria pass.

## Product Boundaries

The first release uses the Native Go SDK and does not provide RabbitMQ/AMQP wire compatibility. Production qualification is limited to Linux/AMD64, three JetStream nodes, three replicas, and eight priority levels (`MaxPriority <= 7`). Delivery is at least once, so applications must consume idempotently. ARM64 and higher priority counts remain outside the qualified production profile.

Completion therefore means that the first-release functionality and principal production tests are substantially done. The remaining work is small in implementation volume but mandatory for a trustworthy production release.
