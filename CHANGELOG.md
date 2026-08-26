# Changelog

[English](CHANGELOG.md) | [简体中文](CHANGELOG.zh-CN.md)

## Unreleased

- Fix Linux CI portability for writable offline Go module metadata, non-root test output/TLS fixtures, and `pipefail`-safe HTTP assertions.
- Add a Dockerized Chromium/Firefox Admin UI E2E release gate covering Queue lifecycle, authentication and revision errors, partial API failure, narrow viewports, credential non-persistence, and automated accessibility checks.
- Fix nested management dialogs and allow an operator token to be entered safely inside the Queue editor.
- Add discoverable bilingual Roadmap, standalone/cluster/Kubernetes deployment guidance, and an operations runbook.

## v0.1.0-rc.1

- Ship a management control plane, embedded Admin UI and `rjsctl` operator CLI around the pinned NATS JetStream `v2.14.1` subtree.
- Provision replicated queues, routing, priority subjects/consumers, audit records and priority-preserving dead-letter handling.
- Provide standalone, three-node Compose and production-oriented Helm deployment assets.
- Add backup/restore, migration, diagnostics, observability, authentication and credential-rotation workflows.
- Qualify the paired Native Go SDK on Linux/AMD64 with three nodes, three replicas and up to eight priorities (`0..7`).
- Verify 24-hour stability, capacity, node failure, network isolation, rolling restart, race, security and recovery gates.

The release uses the Native SDK and is not RabbitMQ/AMQP wire compatible. ARM64 and more than eight priorities are not production-qualified in v0.1.
