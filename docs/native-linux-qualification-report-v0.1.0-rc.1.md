# Rabbit JetStream v0.1.0-rc.1 原生 Linux 测试报告

## 1. 报告摘要

| 项目 | 结果 |
| --- | --- |
| 测试版本 | `v0.1.0-rc.1` |
| 服务端提交 | `a703f1d849b98e4c986c44c511e70116d4109bf0` |
| Native SDK 提交 | `ff54e4a8c135c3f477617d4c3768b81eec98f7ae` |
| 测试平台 | Rocky Linux 10.2，Linux 6.12，AMD64，32 CPU，约 123 GiB 内存 |
| 测试时间 | 2026-08-23 至 2026-08-24（UTC） |
| 综合结论 | **有限制通过（qualified with limitations）** |
| 发布建议 | 不允许以“不限制优先级数量”的方式进入生产；完成 SDK 调度优化和容量复测前，仅可按已验证配置进行受控试点 |

候选版本通过了三节点 JetStream、管理面、Native SDK 核心语义、单节点故障、网络隔离、滚动重启及连续 24 小时稳定性验证。主要阻塞项是 8 个优先级、100 万消息场景未能在 30 分钟硬时限内完成。首个正式版本不包含 RabbitMQ/AMQP 协议兼容，本报告也不对该能力作出结论。

## 2. 范围与方法

本次验证覆盖：

- 三节点 NATS JetStream 集群及管理服务部署；
- Queue 创建、幂等重放、删除、鉴权和审计；
- Native SDK 严格优先级、公平性、PubAck、背压、Nak 重投和 durable 重连；
- 数据完整性、持续负载、资源趋势和三类故障恢复；
- Linux/AMD64 原生运行。

不在范围内：AMQP 协议兼容、ARM64 生产资格、跨仓库 CI。测试二进制和镜像均在本机构建，仅将冻结制品及验证输入传至远端；未复制源代码。实际执行主机为用户指定的 `rabbitjetstream`（`VM-90-6-rockylinux`）。后续验证主机使用规则以 `AGENTS.md` 为准。

## 3. 测试环境

- NATS Server：2.14.1；镜像 ID `sha256:7e68c4f9cc6e0cbad82688c01a90636c3965e622b6802b2f15587cc242a19ff7`
- 容器运行时：rootless Podman 5.8.2，overlay 存储
- 集群：3 个 JetStream 节点，消息流副本数 3
- 文件系统：500 GB，测试结束剩余约 487 GB
- Native Linux 预检：通过

## 4. 结果汇总

| 测试项 | 结果 | 关键结果 |
| --- | --- | --- |
| 24 小时持续负载 | 通过 | 4.32 亿条发布并消费；丢失、重复、损坏均为 0 |
| 资源与健康审计 | 通过 | 4,323 个样本；无不健康、慢消费者、客户端停滞或元数据积压 |
| 单节点停机 | 通过 | 90 万条；数据异常 0；发布重试 1 |
| 网络隔离 | 通过 | 90 万条；数据异常 0；发布重试 8，消费重试 76 |
| 逐节点滚动重启 | 通过 | 240 万条；丢失/损坏 0；发生 4 次预期的 at-least-once 重投 |
| 管理面功能与鉴权 | 通过 | 未认证写入返回 401；apply、noop、delete、audit 均符合预期 |
| Native SDK 功能 | 通过 | 严格优先级、公平性、确认、背压、重投和 durable 场景通过 |
| 3 优先级、100 万条容量 | 通过 | 发布 27,756.67 msg/s；消费 733.05 msg/s；P99 0.498 ms |
| 8 优先级、100 万条容量 | **未通过** | 30 分钟超时，未生成完整结果报告 |

## 5. 24 小时稳定性结果

测试从 `2026-08-23T10:07:06.692039159Z` 运行至 `2026-08-24T10:07:06.692936651Z`，目标速率 5,000 msg/s，1 KiB 消息，8 个发布协程，批次 256，副本数 3。

- 请求、发布、消费：均为 432,000,001 条；
- 丢失、重复、内容损坏：均为 0；
- 发布/消费重试：均为 0；
- 实际发布/消费速率：约 5,000 msg/s；
- 延迟：P50 0.25 ms，P95 0.37 ms，P99 0.95 ms，最大 71.20 ms；
- 峰值 backlog 80，结束时 3，排空耗时 0.0003 秒。

资源审计覆盖每节点 1,441 个样本。NATS 节点峰值内存约为 560/423/414 MB；CPU 峰值 269%/173%/134%（多核计量）；元数据 pending 最大值为 0。持续负载结束后集群保持健康。

## 6. 故障与语义验证

单节点停机和网络隔离期间均保持完整交付。滚动重启场景出现 4 次重复投递，符合 JetStream 的 at-least-once 语义，不视为数据损坏；生产应用仍必须基于业务消息 ID 实现幂等消费。

早期生成的 `fault-restarts.json` 因 systemd transient unit 的 `KillMode` 同时终止了被重启容器，属于测试编排缺陷，已作废且未纳入结论。报告采用修正后的手工逐节点滚动重启结果。

## 7. 已知限制与发布门槛

8 优先级容量场景失败的直接表现是 `context deadline exceeded`。现有 SDK 会顺序探测空的高优先级 consumer，默认每级探测超时 1 ms；优先级数量增长时，空队列探测开销显著压低消费能力。3 优先级场景虽通过，但该工作负载的端到端消费能力仅约 733 msg/s，不能外推为任意消息大小、优先级数量或积压规模下的生产容量。

正式 GA 前必须完成：

1. 优化 SDK 多优先级调度及空 consumer 探测机制；
2. 对计划支持的最大优先级数执行 3/5/8 级及不同 backlog 的容量矩阵；
3. 明确版本支持的最大优先级数、吞吐基线和资源规格；
4. 增加逐优先级 pending、探测延迟和饥饿监控；
5. 在修复后重跑 24 小时稳定性与全部故障测试。

在上述门槛完成前，可考虑仅对 3 个优先级、负载不超过已验证消费能力且具备降级和回滚方案的业务进行受控试点。ARM64 需获得原生主机后单独取得生产资格；RabbitMQ/AMQP 完全兼容仅保留在 Roadmap。

## 8. 证据与可追溯性

| 证据 | SHA-256 |
| --- | --- |
| [资格汇总](../artifacts/highhost/qualification-summary.json) | `9477e52c227ec1cba204b14c91aeffa6d265ac1b0c996bac3dc5699ea4572e4c` |
| [24 小时结果](../artifacts/highhost/soak-24h.json) | `911ac9e1373fee2e6b16f442fc16a01de4bce95e66907315863f67e0b017937f` |
| [24 小时证据索引](../artifacts/highhost/soak-evidence.json) | `19055d78c0db33951905eaf8eec687aa5de42aa6c1dc9028819c196a018d3028` |
| [资源审计汇总](../artifacts/highhost/soak-resources.summary.json) | `ba793a5012e4f3a48401da538fc034211f53575d9f131e2826cf74569013e19f` |
| [单节点停机](../artifacts/highhost/fault-smoke-unique.json) | `e579a30e3bc6226340adfd4754e6fe0afdf5e3c73b3521badd05228eaaba5e38` |
| [网络隔离](../artifacts/highhost/fault-network.json) | `cfc8bc5881ae6209f1a4032522b3d96bc0a508b9cd5334f7635483e4275bb56b` |
| [滚动重启](../artifacts/highhost/rolling-manual.json) | `56efa497f1135f2a784195c9629519607c89fb9c37ac8fa11334a423f7c55aa0` |
| [3 优先级容量](../artifacts/highhost/sdk-priority-3levels-1m.json) | `2be1ea45233d6ce2938559d41d0dfd58191ecf3f6cdfd09bdbe084551cb5dcce` |
| [8 优先级失败日志](../artifacts/highhost/sdk-priority-8levels-failure.log) | `b47f4462a5cfa9540bc18f37247a0fd41de45def3863ab87a07b912151157025` |

测试结束后，临时 Queue、管理实例和辅助二进制均已清理。为避免误删非本轮数据，`RJS_AUDIT_EVENTS` 与 `KV_RJS_META` 被保留。最终三个 NATS 节点及管理服务均为 healthy/ready，工作区已通过 `go test ./...`。
