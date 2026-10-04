# Repository Guidelines

## Project Structure & Module Organization

`upstream/nats-server/` is the pinned Git subtree and server core. Do not edit it for management features. Go management code lives under `management/`, shared Queue contracts under `internal/topology/`, the operator CLI under `tools/rjsctl/`, and the console under `admin-ui/`. Cross-component suites belong in `tests/`; unit tests stay beside packages as `*_test.go`.

Deployment assets live in `deploy/compose/` and `deploy/nats/`. Architecture, configuration, and milestone decisions belong in `docs/`. `outlink/rabbit-jetstream-go` is a development-only symbolic link to the separate SDK repository; do not import it into this module or commit its contents.

## Build, Test, and Development Commands

- `make build`: build `rjs-management` and `rjsctl` into `bin/`.
- `make test`: run Go tests; `make test-race` enables the race detector.
- `make fmt`: format repository Go files.
- `make run`: start the management server against `nats://127.0.0.1:4222`.
- `docker compose -f deploy/compose/standalone.yml up -d`: start a local JetStream node.
- `docker compose -f deploy/compose/cluster.yml config`: validate the three-node deployment definition.
- `go vet ./...`: run static checks before submitting changes.
- `make test-linux-smoke`: build and test the Linux standalone distribution.
- `make test-linux-fault`: verify writes and recovery during a one-node outage.
- `make test-kubernetes`: on native Linux, install the chart into a pinned three-worker kind cluster and verify Pods, PVCs, Helm tests, readiness, and Admin UI.

## Coding Style & Naming Conventions

Use standard Go style and `gofmt`; indentation is tabs as produced by the formatter. Package names should be short, lowercase nouns. Wrap errors with operational context, for example `fmt.Errorf("connect to NATS: %w", err)`. Keep HTTP handlers thin and JetStream behavior behind `management/internal/jetstream`. Configuration variables use the `RJS_` prefix.

## Testing Guidelines

Use Go's `testing` package and table-driven tests. Name tests `TestFeature` or `TestFeature_Scenario`. Unit tests must not require NATS. Put integration, fault, compatibility, and performance suites under `tests/`. New code must not reduce coverage; critical reconciliation and message-semantics code requires exhaustive failure-path tests. See `docs/testing.md`.

## Commit & Pull Request Guidelines

The repository has no established commit history yet. Use concise, imperative subjects, preferably Conventional Commit prefixes such as `feat:`, `fix:`, `docs:`, and `test:`. Keep commits focused.

Pull requests should explain motivation, behavioral changes, validation commands, and operational impact. Link relevant issues or roadmap milestones. Include API examples for endpoint changes and screenshots for future WebUI changes. Call out configuration, compatibility, migration, or security implications explicitly.

## Architecture & Security

Do not modify the NATS subtree for management features. The management service is a control plane and must not enter the message data path. Never commit credentials, generated data, or `.env` files. Production examples should use credentials/NKeys and TLS.

## Documentation Languages

English is the default documentation language. User-facing README, changelog, testing, release, and operational documents must provide a corresponding Simplified Chinese `.zh-CN.md` version with reciprocal language links. Keep commands, version boundaries, safety requirements, and acceptance thresholds semantically identical; resolve ambiguity in favor of the English version.

## Native Linux Qualification Host

Use `ssh jdcloudremote` for native Linux release qualification. SSH logs in as `root`, but run ordinary validation as `sandbox`; use root only for necessary host-level installation or configuration. The `sandbox` account has rootless Podman. Prefer release binaries for simple checks, and use rootless Podman when isolated networking or container behavior is needed.

Never copy the source tree to the remote host. Build all binaries, images, charts, and verification helpers locally with Docker Desktop, then transfer only frozen release artifacts and evidence inputs. Prefer configured Chinese package and container mirrors for any unavoidable remote installation or download. The first release currently requires native execution qualification only on `linux/amd64`; arm64 artifacts may be cross-built but are not production-qualified until an arm64 host is available.


## Agent Skills

Repo-shipped skills for autonomous operation live in `skills/` (SKILL.md each): `rjsctl-operations` (management-plane CLI/API operations and safety rules), `rjs-incident-response` (health/triage/backlog/DLQ/unknown-outcome procedures), `rjs-release-drills` (qualification gates: packaging, soak, cluster smoke, canary, node-failure, rollback), `rjs-console-automation` (admin-UI Playwright/agent automation with page map and traps). Consult the matching skill before operating the system as an agent. The documentation map for humans is `docs/README.md`.
