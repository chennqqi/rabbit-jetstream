# Changelog

[English](CHANGELOG.md) | [简体中文](CHANGELOG.zh-CN.md)

## v0.1.0-rc.1

- Ship a management control plane, embedded Admin UI and `rjsctl` operator CLI around the pinned NATS JetStream `v2.14.1` subtree.
- Provision replicated queues, routing, priority subjects/consumers, audit records and priority-preserving dead-letter handling.
- Provide standalone, three-node Compose and production-oriented Helm deployment assets.
- Add backup/restore, migration, diagnostics, observability, authentication and credential-rotation workflows.
- Qualify the paired Native Go SDK on Linux/AMD64 with three nodes, three replicas and up to eight priorities (`0..7`).
- Verify 24-hour stability, capacity, node failure, network isolation, rolling restart, race, security and recovery gates.

The release uses the Native SDK and is not RabbitMQ/AMQP wire compatible. ARM64 and more than eight priorities are not production-qualified in v0.1.
