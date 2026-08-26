# 测试套件

[English](README.md) | [简体中文](README.zh-CN.md)

- `integration/`：真实单节点和三节点 JetStream 部署的黑盒测试。
- `fault/`：节点丢失、网络中断、进程崩溃、磁盘压力和恢复场景。
- `compatibility/`：与客户端 SDK 仓库共享的 RabbitMQ 风格语义契约。
- `performance/`：可重复的吞吐、延迟、资源和长稳基准。

单元测试与 Go package 放在一起。Fixture 必须确定且不得包含凭据。
