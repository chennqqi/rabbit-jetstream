# 变更日志

[English](CHANGELOG.md) | [简体中文](CHANGELOG.zh-CN.md)

## v0.1.0-rc.1

- 提供管理控制面、内嵌 Admin UI 和 `rjsctl` 运维 CLI。
- 支持多副本 Queue、路由、优先级 subject/consumer、审计和保持优先级的死信转移。
- 提供单机、三节点 Compose 和面向生产的 Helm 部署资源。
- 提供备份恢复、迁移、诊断、可观测性、认证和凭据轮换流程。
- Native Go SDK 在 Linux/AMD64、三节点、三副本和八个优先级（`0..7`）范围内通过生产资格验证。
- 通过 24 小时长稳、容量、节点故障、网络隔离、滚动重启、race、安全和恢复测试。

本版本使用 Native SDK，不兼容 RabbitMQ/AMQP 线协议。ARM64 和超过八个优先级的配置未在 v0.1 中取得生产资格。
