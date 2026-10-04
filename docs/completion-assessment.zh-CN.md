# 项目完成度评估

[English](completion-assessment.md) | [简体中文](completion-assessment.zh-CN.md)

评估日期：2026-08-28。本评估明确区分已实现测试、已保留资格证据、当前提交 CI 状态和最终发布审批。

## 结论

Rabbit JetStream 是一个**在限定配置内已通过生产资格验证的发布候选版本**，并非仅完成约 75% 的原型。首版功能范围已经基本完成：Native Go SDK 在不修改 NATS JetStream 源码的前提下提供 RabbitMQ 风格优先级 Queue，并配套管理控制面、CLI、Admin UI、部署资源、迁移工具和运维文档。

已取得资格的生产配置为 Linux/AMD64、三个 JetStream 节点、三副本和最多八个优先级（`0..7`）。剩余工作主要是发布闭环，而不是产品核心功能缺失。

| 维度 | 完成度 | 判断 |
| --- | ---: | --- |
| Queue 拓扑与管理 | 95% | apply/update/delete、路由、DLQ、reconcile、审计、认证、诊断和 controller 均已实现并测试。 |
| Native SDK 与消息语义 | 95% | 严格优先级、有界公平、PubAck、Ack/Nak/Term、背压、重投、重连和故障恢复均已验证。 |
| Admin UI | 90% | 内嵌控制台及 Docker 化 Chromium/Firefox E2E 覆盖生命周期、错误、响应式布局、凭据处理和可访问性；仍应为最终版本重新生成证据。 |
| 部署与运维 | 95% | 单机/集群 Compose、生产 Helm、Kind 安装、备份恢复、滚动升级、故障、安全门禁和运维手册均已具备并通过测试。 |
| 发布闭环 | 70% | 已有 `rc.1`，但 `rc.2` 仍需最终提交全绿、提交绑定证据/制品、分阶段 Canary 和审批。 |
| 首版总体范围 | **约 92%** | 在既定边界内功能完整且通过生产资格验证，剩余为正式发布控制。 |

## 已完成的验证

- 仓库覆盖率、race、vet、build、API 兼容、安全和镜像门禁已在发布测试中通过。
- Docker 管理矩阵覆盖 standalone、API、reconcile、apply、delete、audit、auth、routing、DLQ、metrics、diagnostics、controller 和 fault。
- Helm fail-closed 校验及三 worker Kind 安装验证了 Pod、PVC、readiness、Helm test 和 Admin UI 可用性。
- 备份恢复和三节点滚动升级/回滚门禁已通过。
- RabbitMQ definitions 转换、校验和对账测试已经实现；真实 RabbitMQ/JetStream 影子流程在当前 CI 可移植性问题发生前已成功完成双写。
- 原生 Linux 资格测试在 Rocky Linux 10.2/AMD64、三节点和 rootless Podman 环境执行。
- 服务端 24 小时测试：432,000,001 条、5,000 msg/s，丢失、重复和损坏均为 0。
- 最终 Native SDK 24 小时测试：432,000,000 条、八优先级、三副本，完整性异常为 0，发布 P99 为 0.915 ms。
- 3/5/8 优先级、每组 100 万条 1 KiB 消息均通过，并完成 16 KiB 和 256 KiB 配置测试。
- 单节点停机、网络隔离、滚动重启、SDK 进程/重连和 race 测试均通过；预期重投验证了 at-least-once 语义。

证据索引见[原生 Linux 资格测试报告](native-linux-qualification-report-v0.1.0-rc.1.md)，原始证据保存在本地忽略目录 `artifacts/highhost/` 和 `artifacts/qualification-v2/`。

## 当前基线与剩余阻塞项

旧评估中的 CRLF 问题已经过时且不准确。production values 契约测试已经通过。`tools/upstreamcheck` 现已分离成功命令的 stdout 与仅警告 stderr；本次受管 Windows 会话中的完整 `go test ./...` 和 `go vet ./...` 均已通过，且未弱化子树检查。

当前记录中，revision `a85b839f` 的最新 CI 除 `rabbitmq-migration` 外全部通过。Definitions 转换已经通过，RabbitMQ 和 NATS 均正常 ready，两次三消息 dual-write 均成功。剩余失败是非 root distroless 容器生成 `dualwrite.ndjson` 后，宿主 PowerShell 读取该文件时遇到 Linux 文件属主权限问题。这属于测试夹具可移植性缺陷，不是 Queue 语义或迁移正确性失败，但正式发布前仍必须恢复全绿 CI。

修复后，发布闭环仍需：

1. 冻结最终 server 和 SDK revision，并重新生成与这些 revision 精确绑定的发布证据。
2. 生成并验证不可变 bundle、镜像 digest、SBOM、attestation、许可证和 `SHA256SUMS`。
3. 执行文档规定的 1%、10%、25%、50% 和 100% Canary，并保存回滚证据。
4. 获得服务负责人、应用负责人和 on-call 审批，并通过 `make verify-release-approval`。
5. 发布 `v0.1.0-rc.2`；只有 RC 观察期和审批标准通过后才晋级 GA。

此前两轮 24 小时结果对各自冻结 revision 仍是有效证据，但发布工作流会有意拒绝将其作为后续运行时或打包变更的“精确 revision 证据”。

## 产品边界

- 首版使用 Native Go SDK，不提供 RabbitMQ/AMQP 线协议兼容。
- Linux/AMD64 已取得生产资格；ARM64 仅交叉构建，尚未完成原生资格验证。
- 生产支持边界为八个优先级（`MaxPriority <= 7`），更高值属于实验配置。
- 交付语义为 at-least-once，业务必须实现幂等消费。
- 吞吐数据是本次硬件和消息大小的资格基线，不是通用 SLA。

## 最终评估

项目已经完成首个明确边界版本绝大部分工程和生产资格工作。准确描述应为：**高成熟度、在限定配置内已通过生产资格验证、等待发布流程闭环的 RC**。它尚不是无限制 RabbitMQ 替代品，也不是 GA；但剩余缺口约占首版范围的 8%，主要集中在 CI 可移植性、最终制品绑定、Canary 和审批，而不是核心 Queue 功能缺失或生产测试未执行。
