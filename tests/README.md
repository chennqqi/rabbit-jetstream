# Test Suites

[English](README.md) | [简体中文](README.zh-CN.md)

- `integration/`: black-box tests against real single-node and three-node JetStream deployments.
- `fault/`: node loss, network interruption, process crash, disk pressure, and recovery scenarios.
- `compatibility/`: RabbitMQ-style semantic contracts shared with client SDK repositories.
- `performance/`: reproducible throughput, latency, resource, and soak benchmarks.

Unit tests remain next to Go packages. Fixtures must be deterministic and contain no credentials.
