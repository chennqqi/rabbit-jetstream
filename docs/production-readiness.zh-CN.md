# 生产就绪与 Canary 操作手册

[English](production-readiness.md) | [简体中文](production-readiness.zh-CN.md)

本手册用于晋级配套 server 与 Native SDK，不证明 AMQP 兼容性。

## 冻结候选版本

记录干净的 server/SDK 40 位 revision、`SDKVersion`、`ContractVersion`、NATS 子树 tag 和预期镜像 digest。运行 `make test-local-release`，保留 `artifacts/local-rc.json` 及其带哈希的性能报告。源码、依赖、Chart 或构建参数变更后，原证据失效，必须重新运行。

## 原生 Linux Soak

使用与生产架构、内核、文件系统、存储类别、CPU/内存限制和容器运行时匹配的独立原生 Linux 主机。Docker Desktop 和 WSL 不可替代。仅传输冻结制品，不得复制源码；以非特权用户运行：

```bash
/srv/rabbit-jetstream/v0.1.0-rc.1/bin/linux-amd64/nativequal \
  -bundle /srv/rabbit-jetstream/v0.1.0-rc.1 \
  -source-revision <40-character-server-revision> \
  -output native-linux-preflight.json
```

预检拒绝非原生 Linux、WSL/Docker Desktop、revision 不符、校验和错误、OCI 平台缺失及当前架构 CLI/management 版本不符。保留独占创建的 JSON 及 soak 证据。首版仅 `linux/amd64` 具备原生执行资格，arm64 交叉构建不代表取得资格。

首轮没有匹配且获准的基线时，明确指定绝对服务阈值：

```powershell
pwsh tests/performance/jetstream.ps1 -Mode soak -Duration 24:00:00 `
  -InauguralBaseline -MinPublishMessagesPerSecond 4900 `
  -MinConsumeMessagesPerSecond 4900 -MaxPublishLatencyP99Millis 10 `
  -Output performance-soak.json
```

保留获准基线，后续冻结版本使用该基线验证：

```powershell
pwsh tests/performance/jetstream.ps1 -Mode soak -Duration 24:00:00 `
  -Baseline performance-baseline.json -Output performance-soak.json
make verify-soak
```

保留报告、资源样本、复制的基线和证据清单四类文件。要求丢失/损坏为零、无节点退出/重启、持续采样三个节点、发布和消费吞吐不低于基线 80%、发布 P99 不超过基线 130%。人工检查 CPU、RSS、磁盘延迟/IOPS、网络、副本延迟和存储趋势；即使数值通过，存在资源耗尽趋势仍应拒绝。

## Canary 顺序

部署不可变镜像 digest 和配套 SDK。保留旧部署；RabbitMQ 迁移还须保留源拓扑和回滚日志。

1. 部署隔离低风险 Queue 群组，检查就绪、controller 主节点、三个当前副本、Admin UI/API、指标、审计和诊断。
2. 合成流量覆盖全部优先级、重复 ID、Ack/Nak/重投、背压和保留优先级的 DLQ；丢失/损坏必须为零。
3. 将所选群组流量依次扩大至约 1%、10%、25%、50%、100%；每个重要阶段至少覆盖一次声明的峰值/积压恢复窗口，不能仅因时间经过而晋级。
4. 不晚于 10% 阶段停止一个 NATS 节点，确认 PubAck/消费持续、副本收敛，再恢复节点。
5. 应用负责人确认载荷保真、幂等、顺序假设及全部使用中的 `partial` 兼容项后，按 Queue 群组扩展。

每阶段要求 JetStream 可用、controller 活跃、预期节点全部可用、DLQ 转移无失败、management 5xx 低于 5%、存储低于 70%、积压低于负载告警阈值、P99 满足 SLO 且不超过基线 130%、吞吐至少为基线 80%。重复率不得超过声明的 at-least-once 预算。

## 停止与回滚

消息丢失/损坏、重复无界、失去 quorum、副本不收敛、PubAck 失败、持续 SLO/错误/积压超限、DLQ 失败、存储超过 85%、元数据不符或存在未批准兼容依赖时，立即停止晋级。停止新增候选路由，保留诊断/审计/性能证据，对已确认消息排空或对账，按演练逆序先回滚 management 再回滚 NATS。不得用备份覆盖仍在运行且数据分歧的集群。

## 单负责人审批

正式 `v0.1.0` 审批须审查配套 revision、本地 Release、原生预检与 soak、Canary 观察、回滚和已知限制。一个人承担服务、应用和值班责任时，仅使用一条 `sole_owner` 签署；否则保留 `service_owner`、`application_owner`、`on_call_operator` 三条角色签署，两种模式不得混用。

复制 [审批模板](release-approval.template.json)，替换全部占位符，为各阶段、节点故障及回滚导出不可变观察文件并计算 SHA-256。模板默认单负责人且尚未签署，必须列出应用使用的全部 `partial` 依赖，其中 `priority-queue` 必填。单负责人在 100% 观察窗口结束后统一审阅完整证据，一次签署承担三类责任。自动化可采集五个阶段，但不可伪造观察或代替最终人工决定。

完成最终观察及适用签署后运行：

```bash
make verify-release-approval RELEASE_APPROVAL=/srv/rabbit-jetstream/release-approval.json
```

校验器检查证据完整性、哈希、版本绑定、阈值、晋级顺序、AMQP/at-least-once 限制，并拒绝提前、重复或缺失签署。它不能证明手工导出指标的真实性，负责人仍须检查原始观察文件。
