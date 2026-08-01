# Repository Guidelines

## Project Structure & Module Organization

`upstream/nats-server/` is the pinned Git subtree and server core. Do not edit it for management features. Go management code lives under `management/`, the operator CLI under `tools/rjsctl/`, and the console under `admin-ui/`. Cross-component suites belong in `tests/`; unit tests stay beside packages as `*_test.go`.

Deployment assets live in `deploy/compose/` and `deploy/nats/`. Architecture, configuration, and milestone decisions belong in `docs/`. `outlink/rabbit-jetstream-go` is a development-only symbolic link to the separate SDK repository; do not import it into this module or commit its contents.

## Build, Test, and Development Commands

- `make build`: build `rjs-management` and `rjsctl` into `bin/`.
- `make test`: run Go tests; `make test-race` enables the race detector.
- `make fmt`: format repository Go files.
- `make run`: start the management server against `nats://127.0.0.1:4222`.
- `docker compose -f deploy/compose/standalone.yml up -d`: start a local JetStream node.
- `docker compose -f deploy/compose/cluster.yml config`: validate the three-node deployment definition.
- `go vet ./...`: run static checks before submitting changes.

## Coding Style & Naming Conventions

Use standard Go style and `gofmt`; indentation is tabs as produced by the formatter. Package names should be short, lowercase nouns. Wrap errors with operational context, for example `fmt.Errorf("connect to NATS: %w", err)`. Keep HTTP handlers thin and JetStream behavior behind `management/internal/jetstream`. Configuration variables use the `RJS_` prefix.

## Testing Guidelines

Use Go's `testing` package and table-driven tests. Name tests `TestFeature` or `TestFeature_Scenario`. Unit tests must not require NATS. Put integration, fault, compatibility, and performance suites under `tests/`. New code must not reduce coverage; critical reconciliation and message-semantics code requires exhaustive failure-path tests. See `docs/testing.md`.

## Commit & Pull Request Guidelines

The repository has no established commit history yet. Use concise, imperative subjects, preferably Conventional Commit prefixes such as `feat:`, `fix:`, `docs:`, and `test:`. Keep commits focused.

Pull requests should explain motivation, behavioral changes, validation commands, and operational impact. Link relevant issues or roadmap milestones. Include API examples for endpoint changes and screenshots for future WebUI changes. Call out configuration, compatibility, migration, or security implications explicitly.

## Architecture & Security

Do not modify the NATS subtree for management features. The management service is a control plane and must not enter the message data path. Never commit credentials, generated data, or `.env` files. Production examples should use credentials/NKeys and TLS.
