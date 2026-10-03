# 裸机资格验收

[English](baremetal-qualification.md) | [简体中文](baremetal-qualification.zh-CN.md)

这是原生 Linux/AMD64、单物理机稳定性配置，不是容器配置，也不证明跨主机高可用。三个带认证的 JetStream 进程采用 R3 存储，同时监测管理进程，并显式传入 `RJS_DEPLOYMENT_PROFILE=cluster`；这是验收拓扑声明，不是根据可达节点进行推断，也不代表验收结论。Kubernetes 验收与应用 Canary 审批仍是独立门槛。

## 构建与来源

**只在本机工作站构建**。禁止向验收主机传源码、编译、安装软件包或运行容器。`scripts/release/package-baremetal.ps1` 校验冻结的运行时制品包，复制原管理服务/CLI，在本机从冻结的 AMD64 OCI 层提取完全相同的 NATS 可执行文件，并使用固定 Go 镜像、禁用网络在本机构建验证工具。

打包前提交验证工具变更。清单分别记录运行时与验证工具 revision，工具变更不会重新标记已有运行时回归证据。全部部署文件由 `SHA256SUMS` 覆盖，保留原运行时清单与 NATS OCI 描述符。裸机预检记录 `deployment_mode: bare-metal`、二进制 SHA-256、清单 SHA-256、文件系统和非特权 UID，不伪造 Docker 证据。

```powershell
./scripts/release/package-baremetal.ps1 -RuntimeBundle dist/v0.1.0-rc.2
```

仅将生成的归档传入 `/opt/rjs-qualification/` 下**全新**的 root 所有目录。解包前校验归档 SHA-256，之后校验 `SHA256SUMS`。目录必须允许动态服务账户访问，可执行文件必须允许其读取/执行，但所有制品均禁止非 root 写入。不得覆盖已有制品或状态目录。

## 安全与执行

提供的 `scripts/baremetal-systemd.sh` 仅用 root 启动临时服务。测试由 systemd `DynamicUser` 运行，不创建永久账户、不启用开机服务、不变更已有服务、防火墙、内核设置或应用数据。服务只能写入新建的 `StateDirectory` 和私有临时目录。网络仅允许回环，十个端口全部绑定 `127.0.0.1`。密码随机生成，仅保留在私有运行文件/环境中。

整个服务共用受限 cgroup：CPU 配额 400%（四个逻辑核），内存软/硬上限 3/4 GiB，禁止 swap，最多 128 个任务，低 CPU/IO 优先级，设备读/写限速 40/20 MB/s（十进制），单文件上限 256 MiB，总运行时限 26 小时。每个 NATS 节点内存存储上限 128 MiB、文件存储上限 2 GiB。主机可用内存低于 16 GiB、磁盘剩余低于 30 GiB、就绪检查失败、节点身份改变或子进程退出时，保护机制仅停止本次测试。不自动重启。资源限制降低干扰风险，但不承诺对共享主机性能完全零影响。

启动管理服务或负载前，三个节点必须对元数据 leader 达成一致，leader 必须连续五次确认另外两个副本均在线且追平。仅进程健康不足以判定集群就绪；此预热时间不计入负载持续时间。

先使用唯一运行 ID 启动 `calibration`（2 分钟，端口 24220–24229），检查完成状态、资源汇总和实际 systemd 限制。通过后另用全新 ID 启动 `soak`（24 小时，端口 24240–24249）：

```bash
bash /opt/rjs-qualification/BUNDLE/scripts/baremetal-systemd.sh /opt/rjs-qualification/BUNDLE UNIQUE-ID RUNTIME-40-CHAR-REVISION calibration
# 校准成功后，使用不同的 UNIQUE-ID：
bash /opt/rjs-qualification/BUNDLE/scripts/baremetal-systemd.sh /opt/rjs-qualification/BUNDLE UNIQUE-ID RUNTIME-40-CHAR-REVISION soak
systemctl status rjs-qual-UNIQUE-ID.service
# 仅在需要停止本次运行时执行：
systemctl stop rjs-qual-UNIQUE-ID.service
```

不得按进程名称批量杀进程或删除状态目录。失败证据保留；重试必须使用新状态目录，重新计算完整观察窗口。监督进程启动错误记录在本服务的 journal 中（`journalctl -u rjs-qual-UNIQUE-ID.service`）；子进程日志与证据仍在私有运行目录。使用 journal 避免在 systemd 创建动态状态目录之前依赖该目录。

## 验收标准

固定负载为每秒 5,000 条消息、1,024 字节 payload、八个发布者、batch 256、R3。要求连续发布至少 24 小时，每 10 秒采样且数据完整，节点不重启，丢失/损坏/重复及发布/消费重试均为零，发布和消费吞吐均不低于 4,900 条/秒，发布 P99 不超过 10 ms。校准采用相同速率及完整性/性能门槛，但绝不作为 24 小时证据。

`/var/lib/rjs-qual-UNIQUE-ID/run/started.json` 记录开始和预计结束时间。`completion.json` 必须为 `completed`，启动健康并不等于通过。完成后生成 `report.json`、`resources.ndjson`、`resource-summary.json` 和校验通过的 `soak-evidence.json`。将这些文件及 `preflight.json`/`command.json` 回传，独立执行 `perfevidence -require-soak -source-revision RUNTIME-REVISION -evidence soak-evidence.json`；同时保留 systemd 限制及状态。

首次裸机配置使用上述明确的绝对门槛。旧容器/其他主机的基线对其原始配置仍有效，但不能作为可比较的性能基线；后续比较必须采用相同硬件、资源限制和负载配置。完成本次验收不授权 GA 发布，也不代替唯一负责人的应用 Canary 决策。
