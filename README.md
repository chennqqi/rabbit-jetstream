# rabbit-jetstream

[English](README.md) | [简体中文](README.zh-CN.md)

A production-oriented RabbitMQ replacement distribution built around the official NATS Server and JetStream.

The pinned upstream server is maintained as a Git subtree in `upstream/nats-server/` and is not modified by default. This repository owns distribution packaging, deployment, the management control plane, embedded Admin UI, operator tooling, and production qualification. The companion Go SDK is developed separately; `outlink/rabbit-jetstream-go` is only a development symlink.

> The first release provides RabbitMQ-style priority queues through the native `rabbit-jetstream-go` SDK. It does not implement the AMQP wire protocol, and existing RabbitMQ clients cannot connect unchanged. Full RabbitMQ/AMQP compatibility remains a future roadmap item.

## Quick Start

Requirements: Go 1.25 (toolchain `go1.25.13`) and Docker Desktop with its Linux engine.

```bash
docker compose -f deploy/compose/standalone.yml up -d --build --wait
go run ./tools/rjsctl status
```

The management endpoint is `http://127.0.0.1:8223`. Admin UI is at `/admin/`, Prometheus metrics at `/metrics`, and health/readiness at `/healthz` and `/readyz`.

## Common Commands

```bash
go test ./...
make build
docker compose -f deploy/compose/cluster.yml up -d
go run ./tools/rjsctl queue validate examples/queues/orders.yaml
go run ./tools/rjsctl queue plan examples/queues/orders.yaml
RJS_ADMIN_TOKEN=secret go run ./tools/rjsctl queue apply --url http://127.0.0.1:8223 tests/fixtures/queue-basic.yaml
go run ./tools/rjsctl diagnostics collect --url http://127.0.0.1:8223 --output diagnostics.zip
helm upgrade --install rabbit-jetstream deploy/helm/rabbit-jetstream --namespace messaging --create-namespace
```

Configuration uses `RJS_*` environment variables or flags. Start with the [Deployment Guide](docs/deployment.md) for standalone, clustered, and Kubernetes installations and the [Operations Runbook](docs/operations.md) for routine checks and incidents. See also [Architecture](docs/architecture.md), [Configuration](docs/configuration.md), [Queue Schema](docs/queue-schema.md), [Native SDK Contract](docs/native-sdk-contract.md), [Testing](docs/testing.md), [Release Process](docs/releasing.md), and [Roadmap](docs/roadmap.md).

## Repository Layout

```text
upstream/nats-server/   pinned official NATS Server subtree
management/             Go management API and control plane
admin-ui/               embedded web console
tools/rjsctl/           operator CLI
deploy/                 standalone and clustered deployment assets
packaging/              release image definitions
tests/                  integration, fault, security, and performance suites
docs/                   architecture and operational documentation
outlink/                untracked links to external SDK repositories
```
