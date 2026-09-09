# 剩余发布工作

[English](remaining-release-work.md) | [简体中文](remaining-release-work.zh-CN.md)

初始计划估算为 92%；按照已经明确的首版边界，当前工程完成度约为 **95%**，这不是发布审批得分。剩余主要是资格验收与发布闭环，而不是核心 Queue 功能缺失。下方保留最初 8% 的拆分以便追溯。

## 当前状态：2026-09-09

- 最终 rc.2 运行时冻结为 `2872e4819f5da873c752a1e06cf186d459a9b594`，SDK 为 `8c63313efc3f9cefb87744358795a633fa55acc2`。这一组合已通过全部 31 项本地 Release 门槛，最终本地证据为 `artifacts/rc2-local-release-final.json`；此结果更新了下文早先的打包校验和跟进状态。
- 负责人选择 `lsb112` 进行无容器验收，并要求全部在本机工作站构建。[真实裸机模式](baremetal-qualification.zh-CN.md)已实现；验证工具独立冻结为 `49f1dbfe43db727022eb2f53aeae7dd98f17a6e7`，运行时二进制与冻结候选版保持完全一致。本机 Go 测试、vet、Linux 工具 race 检查及覆盖率通过：总体 80.4%，关键包 92.3%/96.1%。
- 最终两分钟校准通过：600,001 条消息全部发布并消费，丢失/重复/损坏及重试均为零，吞吐约 5,000 条/秒，发布 P99 为 1.67 ms，资源审计通过。最初的启动日志与元数据收敛问题已保留记录并修复，失败尝试不计入 soak 时长。
- 新的 24 小时运行于 **2026-09-09 12:22:49（北京时间）**开始，预计 **2026-09-10 12:22:49**结束负载，随后完成采样和审计。服务为 `rjs-qual-rc2-20260909-soak02.service`。初始检查确认负载、三个 NATS 进程、管理服务及采样器由同一个非 root 动态账户运行，位于受限 cgroup。全部构建在本机完成，远端未使用容器或编译器；仅新建验收专用目录与测试进程。
- **运行中不等于通过。** 必须完成整个观察窗口并核验最终完整性、性能及资源证据后，才能关闭此门槛。原始报告仍保留在验收主机，下载到本地 artifacts 目录尚待明确的数据回传授权。其他合适主机上的原生 Kind、应用 Canary 观察、唯一负责人签署及公开发布条件仍是独立待办；不会仅因启动 soak 而增加完成度计分。

只读查看当前运行：

```bash
ssh lsb112 'systemctl status rjs-qual-rc2-20260909-soak02.service --no-pager'
```

## 剩余工作

| 领域 | 占比 | 必须达到的结果 |
| --- | ---: | --- |
| CI 全绿基线 | 1% | 修复 RabbitMQ/JetStream 影子迁移测试中的 Linux 文件属主问题，在不降低断言的前提下让全部 CI Job 通过。 |
| 最终冻结与回归 | 2% | 冻结 server 和 Native SDK revision，重新执行 Go、race、覆盖率、安全、Admin UI 浏览器、管理集成、Helm/Kind、故障、恢复、升级和迁移门禁。 |
| 最终资格证据 | 2% | 将资格证据绑定到精确的发布 commit 和不可变镜像 digest。已有 24 小时结果对其原冻结 revision 仍然有效，但不能自动覆盖后续运行时或打包变更。 |
| 发布制品与供应链 | 1% | 生成 Linux/AMD64 二进制、不可变容器镜像、Helm Chart、SBOM、attestation、许可证记录和经过校验的 `SHA256SUMS`，并验证从零安装与回滚。 |
| Canary 与审批 | 2% | 完成文档规定的 1%、10%、25%、50% 和 100% Canary，保留健康与回滚证据，取得负责人/on-call 审批，并通过 `make verify-release-approval`。 |

## 发布顺序

### 2026-09-08 推进记录

- 已为 Linux 影子迁移容器实现宿主 UID/GID 映射。证据仍保持生产代码的 `0600` 权限，输出目录不再需要所有用户可写权限。
- 增加包含 `/bin/cp` 和生产 CLI 的专用切流测试镜像。生产镜像仍为 distroless。
- 将定义迁移和影子迁移加入本地 `Release` 回归入口。
- server/SDK 的测试、vet、构建通过（`artifacts/remaining-release-quick.json` 标记为工作区未冻结的开发验证），两个仓库的 Linux 容器 race 测试也通过。覆盖率门禁通过：总体 80.2%、topology 92.3%、controller 96.1%。
- 使用已有本地测试镜像和宿主 Go 模块缓存，通过完整影子流程：双写/恢复、两次三消息对账、幂等切流和回滚。Linux Docker 卷检查确认实际 CLI 输出保持 `0600`、属主为 UID 1000，且该 UID 可读。这不能证明当前 revision 的 CI 已全绿。

后续进展与剩余条件：

- 重试后 NATS/operator 全新构建已恢复。完整构建路径的影子迁移（使用宿主模块缓存）、定义迁移、全部八项 Chromium/Firefox Admin UI 测试及 Helm 门禁均通过，但这些结果早于后续安全依赖升级。当前 revision 的 CI 仍需执行。
- 更新后的镜像扫描在 `golang.org/x/crypto v0.53.0` 中检出 [CVE-2026-56854](https://pkg.go.dev/vuln/GO-2026-6303)，在 `google.golang.org/grpc v1.82.1` 中检出 [CVE-2026-84304](https://github.com/grpc/grpc-go/security/advisories/GHSA-vp52-pcj8-j9qc)。已将 server 模块及镜像构建覆盖版本中的 crypto 升级为 `v0.55.0`，同步所需依赖，并将 gRPC 升级为 `v1.83.1`。两项升级后的 Go 测试、vet、Linux race 和完整安全门禁通过。三个重新构建的生产镜像在仓库既定的已有修复版本 HIGH/CRITICAL 漏洞门禁下均为零检出。80.2% 覆盖率、八项浏览器测试及完整构建影子迁移也在 crypto 升级后通过，但早于最终 gRPC 升级。冻结 revision 的完整回归仍待完成。NATS 和 management 构建现采用 operator 的模块缓存及代理/校验镜像默认配置，不关闭校验。
- 重试后 `ssh jdcloudremote` 已连接成功。使用 `runuser -l sandbox` 并设置 `XDG_RUNTIME_DIR=/run/user/1000`，避免 rootless Podman 继承 root 的运行时目录和工作目录。已有冻结 rc.1 制品再次通过原生预检（13 条校验和、三个 OCI 归档及两个二进制版本检查）；证据为 `artifacts/native-preflight-20260908.json`，SHA-256 为 `8ec7878e3eeeafd7c84094b323a46ef40ac241c389ed4d7a8b1d86387f9a6bcc`。它绑定 server `a703f1d849b98e4c986c44c511e70116d4109bf0` 和 SDK `ff54e4a8c135c3f477617d4c3768b81eec98f7ae`，不代表 rc.2 资格。SSH/SCP 偶发失败时重试，不得传输源码。
- SDK 仍为 `0.1.0-rc.1`，revision 为 `b7acd5285656b52756756b8b62e4d9a1e4d68bbc`。最终 server/SDK 冻结、精确 revision 的完整回归、资格验收及供应链制品仍待完成。
- 尚未提供应用 Canary 目标、负载和签署负责人。必须具备真实分阶段观察及负责人/on-call 签署，合成测试不能替代。

本次未发布 `rc.2` 或 GA。在强制门禁闭环前，总体完成度估计保持不变。

### 冻结回归与资格主机决定

server `6d437e2403bc75b866ba394805491584e3d19586` 与 SDK `8c63313efc3f9cefb87744358795a633fa55acc2` 均在干净状态下通过全部 31 项本地 Release 门禁。证据为 `artifacts/rc2-local-release.json`，SHA-256 为 `f974ec636c31d01078111105df04e37e5d678f93be9f9cda4f7801ba9a5f9098`。覆盖率为总体 80.3%、topology 92.3%、controller 96.1%。20,000 条消息的本地性能冒烟通过，但不代表生产容量。原生 Kind 是独立门禁，本地入口未执行该项。

随后双架构打包发现安全依赖覆盖后 ARM64 NATS 构建缺少 `x/sys` 校验和。构建现于编译前显式下载该模块。这项打包变更要求重新冻结 server 并回归；此前证据仍仅绑定 `6d437e24`。

负责人将另行提供正式资格主机。当前 2 核/4 GB 的 `jdcloudremote` 不启动也不宣称通过正式 24 小时 soak，仅用于隔离的安装、故障和回滚功能检查。原生 Kind 及精确候选版本的长期资格验收仍待合适主机。旧 32 核主机报告不能证明新版合格。

### rc.2 冻结与单负责人流程

rc.2 SDK 候选版本已在独立 `rabbit-jetstream-go-rc2` 副本的本地 `release/v0.1.0-rc.2` 分支冻结为 `8c63313efc3f9cefb87744358795a633fa55acc2`。原 SDK 副本保留。server 回归必须显式传入 `-SDKPath`，确保集成和 race 使用同一 SDK。

负责人已确认一人承担服务、应用和值班责任，因此审批模板使用一条尚未签署的 `sole_owner` 记录。最终观察窗口结束后，统一审阅全部阶段/故障/回滚证据，一次签署。五个自动观察阶段及其完整性、健康、回滚门禁仍适用；准备候选版本的授权不视为最终批准。

本地 rc.2 包嵌入 BuildKit SBOM/provenance attestation 并附许可证记录。所有二进制均在本地 Docker 中构建；仅可向 `jdcloudremote` 传输冻结制品和证据输入。公开发布者签名及再分发条款与本地制品资格验收分别处理。

### 必须遵循的顺序

1. 恢复全部 CI 全绿基线。
2. 冻结最终 server 和 SDK revision。
3. 生成与 revision 绑定的资格证据和发布制品。
4. 发布 `v0.1.0-rc.2` 并执行分阶段 Canary。
5. 仅在观察期和审批标准通过后晋级 `v0.1.0` GA。

## 产品边界

首版使用 Native Go SDK，不提供 RabbitMQ/AMQP 线协议兼容。生产资格范围限定为 Linux/AMD64、三个 JetStream 节点、三副本和八个优先级（`MaxPriority <= 7`）。交付语义为 at-least-once，业务必须实现幂等消费。ARM64 和更高优先级数量仍不属于已取得资格的生产配置。

因此，92% 表示首版功能和主要生产测试已经基本完成。剩余工作实现量不大，但都是可信生产发布不可省略的门禁。
