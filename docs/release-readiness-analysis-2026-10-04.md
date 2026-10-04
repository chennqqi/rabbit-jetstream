# 发布就绪度分析（2026-10-04）

> 状态：即时分析快照，未提交。分析基线：HEAD `cc22a49f77b029a7eb9aec65b5ecf7be9f31e0e9`（与 origin/main 同步，工作树干净）。
> 分析方法：实际运行校验工具、核对证据包内容与修订绑定，非文档自述。

## 总体结论

**功能完成度约 95%，重量级资格认证已全部执行，但当前不具备发布标准。**

项目自己的最终强制门禁 `make verify-release-approval` 在当前代码树上实测失败，且五份关键证据分别绑定在 5 个不同的修订号上，批准包与校验工具 schema 脱节。产品本体和资格认证实质上已经完成，卡在最后一步"批准包的可验证性"上。

## 一、已完成的部分（真实有效）

| 领域 | 状态 | 证据 |
|---|---|---|
| 核心功能（队列拓扑/管理/SDK 语义/控制面） | ~95% | 管理矩阵 11 场景、契约测试、覆盖率 80.4%（关键包 92.3%/96.1%） |
| 42h 裸机 soak（要求 24h） | 通过 | 5053 个证据周期全绿、零崩溃、零错误日志、453,505 条消息（`docs/releases/v0.1.0-rc.3-soak-evidence.md`） |
| helm/Kind 三 worker 集群资格认证 | 通过 | R3+mTLS+PDB+NetworkPolicy 正反向验证、PVC Bound、干净卸载（`docs/releases/v0.1.0-rc.3-cluster-evidence.md`） |
| 五阶段金丝雀 + 节点故障 + 回滚演练 | 已执行 | `artifacts/rc3/canary-drill-evidence.tar.gz`（2026-10-04） |
| 唯一所有者签署 | 已签署 | `approved_at: 2026-10-04T07:20:00Z`（`docs/releases/v0.1.0-rc.3-release-approval.json`） |
| 发布基础设施 | 已定义 | `.github/workflows/ci.yml`（14 类任务）+ `release.yml`（SBOM/provenance/Helm/SHA256SUMS） |
| 双语文档 | 齐全 | 用户/运维/SDK/架构全套 + zh-CN 对照 |

关键有利事实：**运行时代码在 soak 修订（033010ee）与批准修订（f08af85）之间 0 差异**。实测：

```
git diff --name-only 033010ee f08af85 -- management internal api tools admin-ui/src admin-ui/package.json go.mod
（输出为空，0 个文件）
```

差异仅为文档、演练脚本（`tests/soak/*.sh`）、`.gitattributes` 和 admin-ui dist。因此现有 soak/集群/金丝雀证据可以合法地重新绑定到最终修订，无需重跑 42 小时。

## 二、阻塞发布的问题（实测发现）

### A. 批准包无法通过自家校验工具（最严重）

实测命令与结果：

```
$ go run ./tools/releaseapproval -evidence docs/releases/v0.1.0-rc.3-release-approval.json \
    -source-revision cc22a49f77b029a7eb9aec65b5ecf7be9f31e0e9
decode approval evidence: json: unknown field "cluster_qualification_evidence"
exit status 1
```

逐条对照 `tools/releaseapproval/main.go` 的校验逻辑，即使修掉该字段还会连环失败：

1. **版本一致性约束不满足**：工具要求 `release_version` 去掉 `v` 前缀后等于 `sdk_version`（main.go:163）。当前 `0.1.0-rc.3` vs SDK `0.1.0-rc.1` 不匹配。
2. **金丝雀阶段缺必需字段**：批准 JSON 的 5 个阶段只有 `traffic_percent` + `evidence`；工具的 `stage` 结构要求 `started_at/ended_at`、`storage_percent`、`backlog_messages/backlog_limit_messages`、`publish_p99_slo_millis`、`baseline_publish_p99_millis`、`duplicate_rate_percent/duplicate_budget_percent` 等。演练脚本产出的是另一套 schema（如 `management_5xx_count`、`received_messages`）。
3. **stage-100 证据 `current_nodes: 0`**：违反工具的 `current_nodes == expected_nodes(3)` 门禁；演练文档未解释此异常，疑似演练脚本采集缺陷。
4. **node-failure 证据字段不匹配**：有 `recovery_cycles: 1`，缺工具要求的 `puback_continued/consume_continued/recovery_seconds/recovery_slo_seconds`。
5. **rollback.json 是非法 JSON**：`"canary_stream_messages": ,`（语法错误），且 `recovery_seconds: 0`（工具要求 > 0）。
6. **证据路径全部无法解析**：批准 JSON 位于 `docs/releases/`，相对路径却指向仓库根的 `artifacts/`；`docs/releases/artifacts/` 不存在（实测 glob 无结果）。soak 文件字段还带括号注释 `(42h bare-metal soak, 5053 cycles)`，不是合法路径。
7. **local_release_evidence 绑定错误**：当前 `artifacts/local-rc.json` 是 2026-10-04 生成的 **Quick 模式**运行、绑定 `28adba8`；工具要求 `mode == "release"` 且 `server.revision == 批准修订`。

### B. 修订绑定混乱（5 个 SHA 并存）

| 证据 | 绑定修订 |
|---|---|
| soak/集群/金丝雀 tarball（`artifacts/rc3/revision.txt`） | `033010ee90cf93b5c469bef16e60d673b5e2048c` |
| 批准 JSON `server_revision` | `f08af851590d5940f160c4151b1f023e1f2b210a` |
| `artifacts/local-rc.json` | `28adba81c9d4ec7a7065339dfbed4eec9119bf97` |
| 标签 `v0.1.0-rc.3` | `ee63964759f0266f71d7a06701e80a77d6673386`（落后 HEAD 4 个 docs 提交） |
| HEAD / origin/main | `cc22a49f77b029a7eb9aec65b5ecf7be9f31e0e9` |

运行时代码虽一致，但"资格绑定到精确树"是项目自己的铁律（`skills/rjs-release-drills/SKILL.md`：re-run gates after ANY merge; record the revision in every evidence file），当前状态经不起审计。

### C. soak 证据格式与门禁链路不匹配

校验工具和 `release.yml` 都期望 `rabbit-jetstream.io/performance-evidence/v1alpha1` JSON（由 `tools/perfevidence` 产出，`mode=soak`，`source_revision` 绑定批准修订）；实际证据是日志 tarball（`soak-evidence.log` 等四个日志文件，无 JSON）。发布工作流中的 `perfevidence -require-soak -source-revision $GITHUB_SHA` 检查同样会失败。

### D. 发布动作未执行

- 无 GitHub release；仓库为私有（web 访问 404），本环境无法确认最终修订的 GitHub Actions 是否全绿（历史上 `rabbitmq-migration` 曾是 flaky 项）。
- 标签 `v0.1.0-rc.3` 落后 HEAD 4 个 docs 提交；本环境 SSH 无密钥，无法确认标签是否已推送远端。
- 公开工件签名/再分发条款在 `docs/remaining-release-work.md` 中被明确标注为独立未关闭门禁。

### E. 文档滞后

`docs/remaining-release-work.md` 仍停留在 rc.2 时代叙述（"No `rc.2` or GA release was published"），其 8% 分解表大部分已被 rc.3 证据关闭但未回填——违反项目自己的 A14 回填义务（`docs/review-improvement-plan.md`）。

## 三、建议行动清单

### 第一优先：让门禁真实变绿（约 1–2 天工作量）

1. 确定单一最终冻结修订（建议以 HEAD 为准），把"运行时代码自 033010ee 起 0 差异"写入证据说明，统一所有绑定。
2. 修复 `rollback.json` 语法错误；按批准工具 schema 重新生成演练证据（补齐缺失字段）；排查 stage-100 `current_nodes=0` 采集缺陷，必要时重跑该阶段。
3. 决策 `cluster_qualification_evidence`：推荐给工具**增加**该字段（这是 rc.3 新增的重型门禁，工具落后于流程），而非从 JSON 删除。
4. 在最终修订以 **Release 模式**重跑 `make test-local-release`，替换 `local_release_evidence`。
5. 用 `tools/perfevidence` 将 soak 证据转为标准格式并绑定最终修订（或同步修改工具 + release.yml 接受 tarball 格式，二选一保持一致）。
6. SDK 版本对齐决策：出 `0.1.0-rc.3` SDK 标签，或修改工具的版本一致性约束。
7. 修正证据路径解析方式（移动 JSON 或改写路径），重跑 `make verify-release-approval` 至绿色，提交并按需重打标签。

### 第二优先：发布动作

8. 在 GitHub 确认最终修订 CI 全绿（重点 `rabbitmq-migration`、`kubernetes-smoke`）。
9. 推送最终标签 → 准备证据 release（`performance-soak.json` 资产）→ 触发 `release.yml`：镜像发布 + attestation、Helm 打包、SHA256SUMS、草稿转正式。
10. 完成公开签名（GPG/cosign）与再分发条款决策。
11. 回填 `remaining-release-work.md`、`completion-assessment.md` 状态。

### 第三优先：GA 条件澄清

12. 金丝雀目前是资格主机上的演练证据；若坚持"真实部署金丝雀"，需在真实环境执行，或在批准记录中明确演练替代的边界。
13. ARM64 维持"未生产认证"的限制声明。

## 一句话总结

产品本体和资格认证实质上已经完成，卡在最后一步"批准包的可验证性"上——证据是真实的，但打包方式与自家门禁工具脱节。需要一次以 `make verify-release-approval` 变绿为目标的证据重整，然后即可走 `release.yml` 发布。

---

# 附录 A：修复验证（2026-10-04 晚，HEAD `616b66d`）

针对上文第二节问题的修复情况实测核对：

| 原问题 | 状态 | 说明 |
|---|---|---|
| A1 未知字段 `cluster_qualification_evidence` | ✅ 已修复 | 工具新增该字段（`omitempty`），批准 JSON 保留 |
| A2 版本一致性（rc.3 vs SDK rc.1） | ✅ 已修复 | SDK 升至 `0.1.0-rc.3`（`53f612b`），批准 JSON 同步 |
| A3 金丝雀阶段缺必需字段 | ✅ 已修复 | 批准 JSON 补齐 started_at/ended_at/全部指标字段 |
| A4 stage-100 `current_nodes: 0` | ✅ 根因已修 | `/jsz` 不含 per-stream 配置 → 改用 nats CLI（616b66d），并写入 troubleshooting 文档 |
| A5 node-failure 字段不匹配 | ✅ 已修复 | 补齐 puback_continued/consume_continued/recovery_seconds/SLO |
| A6 rollback.json 非法 JSON | ✅ 已修复 | 重新生成，recovery_seconds=1（SLO 30s） |
| A7 证据路径无法解析 | ✅ 已修复 | 证据移至 `docs/releases/evidence/`，相对路径可解析 |
| A8 local-rc 绑定错误 | ⚠️ 部分修复 | 绑定修订正确（`6a985c5`），但仍是 **quick 模式**，且工具被改为接受 quick（见下） |
| B 修订绑定混乱 | ⚠️ 好转但再次失绑 | 见下 |

实测校验结果：

```
$ go run ./tools/releaseapproval -evidence docs/releases/v0.1.0-rc.3-release-approval.json \
    -source-revision 6a985c529e112557767b476dca314b61eb6c0154   # 批准绑定的修订
release approval evidence verified        # 通过

$ go run ./tools/releaseapproval ... -source-revision $(git rev-parse HEAD)   # HEAD = 616b66d
release identity is incomplete or mismatched   # 再次失败
```

## 遗留问题（3 项）

1. **绑定再次失效**：批准 JSON 绑定 `6a985c5`，但其后又有提交 `616b66d`（soak 脚本修复 + 文档）。`make verify-release-approval` 使用 `git rev-parse HEAD`，当前**再次变红**。运行时代码在两者间仍为 0 差异（616b66d 只改了 `tests/soak/rc3-canary-drill.sh` 和文档），重新绑定是合理的，但需把批准提交放在分支最后，或每次提交后重新绑定并记录。
2. **门禁放宽**：工具被改为接受 `mode=quick` 的 local-rc 证据。Release 模式比 Quick 多跑 race/coverage/backup/rolling/migration/helm/security/performance 门禁——接受 quick 意味着批准包不再证明这些门禁在冻结修订上运行过，与仓库自身"不放宽断言"的规则冲突。建议改为在最终修订跑一次 Release 模式（`make test-local-release`），恢复工具的严格断言。
3. **证据文件卫生**：五份 `canary-XXX-stage.json` 实为字节完全相同的 gzip 压缩包（同一 sha256 `9ff6d737…`），以 `.json` 后缀提交。工具只做哈希校验所以通过，但命名有误导性；建议每阶段存纯 JSON 或内容各异的真实归档。

---

# 附录 B：性能分析（2026-10-04）

## B.1 新鲜数据点：CI 模式性能烟雾（当前修订 `616b66d`）

在本机 Docker Desktop 手动复刻标准编排执行（相同镜像摘要、相同三节点拓扑、相同 bench 参数；基础镜像经 daocloud 镜像加速站按精确摘要拉取，未改动任何仓库文件）。报告存于 `artifacts/perf-ci-20261004.json`。

| 指标 | 数值 |
|---|---|
| 工作负载 | 20,000 × 1 KiB，3 副本，4 发布者，batch 256，8 vCPU |
| 完整性 | published = consumed = 20,000；missing/duplicates/corrupt/retries = **0**（通过 harness 断言门禁） |
| 发布吞吐 | **2,271 msg/s** |
| 消费吞吐 | **2,271 msg/s** |
| 发布延迟 | P50 1.57 ms / P95 3.14 ms / **P99 4.83 ms** / max 11.8 ms |
| 积压 | 峰值 32 条；排空 0.002 s |
| 总时长 | 8.81 s |

注意：**本机为 8 vCPU 的低配开发环境（Windows + Docker Desktop），与历史容量锚点所在的高配机器（32 核裸机等）不是同一硬件等级，吞吐数字不可横向对比**。此外 8.8 秒的短运行由建流/建消费者/预热主导，吞吐是下界；且按 `docs/performance-testing.md` 的明确要求，Docker Desktop 结果**不得**作为 Linux 生产硬件容量证据。此数据点的价值是：当前修订在低配环境的标准 CI 信号负载下完整性零缺陷、延迟在 SLO 内——是正确性与延迟信号，不是容量信号。

## B.2 金丝雀演练性能（绑定 `6a985c5`，裸机 jdcloudremote，2026-10-04）

| 阶段 | 设定速率 | 实测吞吐 | P99 | 完整性 |
|---|---:|---:|---:|---|
| 1% | 1/s | 1/s | 1.00 ms | 0 丢失/损坏/重复 |
| 10% | 10/s | 9.95/s | 1.42 ms | 同上 |
| 25% | 25/s | 24.72/s | 1.02 ms | 同上 |
| 50% | 50/s | 48.83/s | 1.04 ms | 同上 |
| 100% | 100/s | 94.75/s | 0.98 ms | 同上 |

- **延迟对流量份额完全不敏感**（1%→100% P99 稳定在 ~1 ms），远低于 10 ms SLO 与 5 ms 基线。
- 100% 阶段吞吐为设定值的 94.8%——计时器/PubAck 余量所致，在 80% 门禁内，但值得知晓。
- 节点故障演练：597 发布 / 0 错误 / 1194 接收（at-least-once 语义下约 2× 重投），副本收敛，恢复 1 s（SLO 60 s）。回滚演练：1 s（SLO 30 s）。
- **证据质量疑点**：`canary-100-sub` 记录 received 5250 / expected 5681 但 missing=0——原因是 sub 固定 45 s 窗口 < publish 60 s，快照先于排空（在途消息不计缺失）。建议演练脚本改为"发布结束后排空再计数"，使原始证据直接满足 `received == expected`。

## B.3 历史容量锚点对比（按硬件等级分组）

**高配机器（容量锚点，不可与低配环境数字对比）：**

| 数据点 | 硬件等级 | 吞吐 | P99 | 绑定修订 | 有效性 |
|---|---|---:|---:|---|---|
| rc.2 时代 24h 裸机 soak | 32 核裸机 | ~5,000/s | 0.915 ms | 旧修订 | 容量级，但**不绑定 rc.3**，且 remaining-release-work 明言旧主机报告不能认证新候选 |
| Docker Desktop 1M 规模（2026-09-12） | 高配机器 | 5,453/s | 2.28 ms | 当时开发树 | 模拟证据，非修订绑定 |

**低配/资格主机（稳定性与功能演练）：**

| 数据点 | 硬件等级 | 负载 | 有效性 |
|---|---|---|---|
| rc.3 42h 裸机 soak（jdcloudremote） | 2 核 / 4 GB | ~3/s（3 主题 × 1 msg/s） | 稳定性 soak，5053 周期全绿；**非容量证据** |
| 金丝雀演练（同主机） | 2 核 / 4 GB | ≤100/s | 功能与故障语义演练；**非容量证据** |
| 本次 CI 烟雾（本机） | 8 vCPU Docker Desktop | 20k × 1 KiB | 完整性 + 延迟信号；**非容量证据** |

**发布 soak 验收标准（performance-testing.md）：** 吞吐 ≥4,900/s、P99 ≤10 ms——该绝对门禁是在高配主机等级上标定的。

## B.4 性能结论与发布缺口

1. **延迟**：全谱系优秀且稳定——裸机金丝雀 ~1 ms、Docker CI 4.83 ms、历史 0.915–2.28 ms，全部远低于 10 ms SLO。无回归迹象。
2. **完整性**：所有负载形态下零丢失/零损坏/零重复/零重试，at-least-once 语义在节点故障下按预期重投。
3. **稳定性**：42h 低速率 soak 5053 周期全绿，零崩溃零错误。
4. **核心缺口——容量证据未绑定 rc.3，且存在主机等级错配**：仓库中不存在绑定 rc.3 修订的 ≥4,900/s × 24h 容量级 soak（`performance-baseline.json` 不存在；旧 24h 证据属旧修订 + 高配 32 核主机）。按 `docs/performance-testing.md`，发布 soak 要求容量工作负载 + 基线对比，且基线必须与候选**同主机等级**（"same dedicated hosts … as the candidate"）。而 ≥4,900/s 绝对门禁是在高配主机等级上标定的——当前资格主机 jdcloudremote 为 2 核 / 4 GB（remaining-release-work 明言其仅用于安装/故障/回滚功能检查），在该等级主机上直接套用 4,900/s 门禁要么不可达、要么产生不可比对的容量声明。**建议二选一**：(a) 提供与生产目标/历史锚点同级的裸机主机，对最终冻结修订执行 24h 容量 soak（`-Mode soak -InauguralBaseline -MinPublishMessagesPerSecond 4900 …`）；(b) 若以低配主机等级作为受支持的最低部署等级，则以标定数据推导该等级的绝对门禁（先在该主机跑一次标定运行），在 known_limitations 中记录容量证据的主机绑定，并在 qualified profile 中声明对应包络。这是发布前唯一实质性的性能缺口。
5. 次要建议：金丝雀演练脚本修复排空计数；100% 阶段吞吐缺口的成因（计时器 vs PubAck 反压）可在下一次演练中用更长窗口确认。

---

# 附录 C：资格主机最小配置实测验证（2026-10-04，jdcloudremote）

## C.1 验证方法

在实际资格主机 jdcloudremote（**2 vCPU / 3.7 GB 内存 / 60G ext4（ROTA=1）**，Kind-in-Podman 集群同期在跑、占 ~1.5 GB——即带背景干扰的悲观测量）上，用标准 `jetstream-bench`（与发布 harness 同参数形态：1 KiB、batch 256、R3、PubAck 逐条确认）对 drill 遗留的三节点 NATS 集群（冻结修订 `033010ee` 二进制）做三档发布者并行度探针。bench 在本地交叉编译（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`，sha256 `06ed70f4…`），按"只传冻结二进制+校验和"规则传输，主机端校验一致后以 sandbox 身份执行。报告保留在主机 `/home/sandbox/rc3/calib/report-*.json`。

## C.2 实测结果

| 探针 | 吞吐（发布=消费） | P99 | 负载（2核） | 完整性 |
|---|---:|---:|---:|---|
| 4 发布者，300k | 3,124/s | 5.16 ms | — | 0 丢失/重复/损坏/重试 |
| 8 发布者，200k | 4,334/s | 6.67 ms | 5.63（饱和） | 同上 |
| 16 发布者，200k | **6,171/s** | **9.17 ms** | 5.19（饱和） | 同上 |

磁盘 fsync 探针（4k oflag=dsync）：**~1.9 ms/次**——对观测速率（最高 ~6 MB/s 顺序写）不构成瓶颈；瓶颈是 CPU（负载 5+ / 2 核 = 完全饱和无余量）。

## C.3 结论：修正"最小配置"判断

1. **官方最小配置（4 GB/小型 CPU）确实"能跑"**——且实测远超预期：2 核主机在 16 发布者下达到 **6,171 msg/s，超过 4,900/s 门禁（126%）**，P99 9.17 ms 仍在 10 ms SLO 内。
2. **但该点不可作为认证工作点**：CPU 饱和（负载 ~5.2/2 核）意味着零余量——官方指南要求稳态之上留 20–30% CPU 余量供节点重平衡；capacity-planning.md 要求峰值 < 实测的 70%。按此规则，**该主机等级的安全可持续工作点约 3,000–4,300 msg/s**。
3. **语义健全性与主机等级无关**：三档探针全部零丢失/零重复/零损坏/零重试——2C/4G 等级主机上的 at-least-once 语义与完整性门禁全部成立。
4. **对发布 soak 的直接推论**：4,900/s 绝对门禁在该主机等级**技术可达但无工程余量**。两条合规路径不变：(a) 同级或更高级主机跑 4,900/s 门禁；(b) 就用 2C/4G 主机，但先以本次标定为依据推导该等级门禁（建议 min publish/consume ≈ 3,000/s、max P99 10 ms），以 inaugural baseline 模式跑 24h soak，并在 known_limitations 记录容量证据的主机绑定。
5. **对中间件的正确框架——能力包络，而非业务峰值**：本项目是基础设施/中间件，面向未知的多消费方，发布认证应声明"主机等级 → 能力包络"（sustained 吞吐、延迟 SLO、完整性、故障行为），由消费方按包络选型；资格主机由"想声明的包络 + 工程余量"决定，而非某个业务的预测负载。据此：2C/4G 实测天花板 6,171/s → 可支撑声明 **~3,000 msg/s 包络**（含余量）；若要声明 5,000/s 包络则需 4C/8G 级主机。**当前 qualified profile 缺吞吐/延迟包络声明**（v0.1.0-rc.3.md 只声明了拓扑/副本/优先级/投递语义）——这是发布前的文档缺口。24h soak 应在声明速率（稳态+突发形态）下跑，而非饱和点——饱和点只用于测天花板。

## C.4 本次验证的边界

- 探针为 46–96 秒短运行，建立的是主机等级天花板，用于门禁推导；24h soak 证据仍需单独执行。
- 测量带 Kind 集群背景干扰（悲观方向）；干净主机数字会略高，不影响"安全工作点 ~3,000–4,300/s"的量级结论。
- 复用的是 drill 遗留三节点集群（冻结 `033010ee` 二进制）；正式 soak 应对最终冻结修订重建集群。

---

# 附录 D：v0.1.0 双档能力包络（已确认，2026-10-04）

决策：qualified profile 同时声明两档包络（最低受支持档 + 推荐档），消费方按部署主机等级选型。

## D.1 包络声明提案（写入 qualified profile）

**EN（v0.1.0-rc.3.md "Qualified Profile" 增补）：**

> **Performance envelope** (1 KiB payloads, three replicas, batch 256, PubAck per message, at-least-once):
> - **Minimum supported host class** (2 vCPU / 4 GB RAM / local disk): sustained 3,000 msg/s concurrent publish+consume, publish P99 ≤ 10 ms, burst headroom to ~4,300 msg/s. Measured ceiling 6,171 msg/s (2026-10-04 calibration, `docs/release-readiness-analysis-2026-10-04.md` §C.2).
> - **Recommended host class** (4 vCPU / 8 GB RAM / local SSD): sustained 5,000 msg/s, publish P99 ≤ 10 ms. Anchored by the 32-core 24-hour soak (432M messages at a fixed 5,000 msg/s, P99 0.915 ms); a 4-core-class 24-hour soak formalizes this tier.
> - Capacity sizing per Queue follows `capacity-planning.md`; peak ingress must stay below 70% of the tier's sustained envelope.

**ZH（v0.1.0-rc.3.zh-CN.md 对应段落语义一致）：**

> **性能包络**（1 KiB 载荷、三副本、batch 256、逐条 PubAck、at-least-once）：
> - **最低受支持主机等级**（2 vCPU / 4 GB 内存 / 本地磁盘）：持续 3,000 msg/s 并发发布+消费，发布 P99 ≤ 10 ms，突发余量至 ~4,300 msg/s。实测天花板 6,171 msg/s（2026-10-04 标定，见分析文档 §C.2）。
> - **推荐主机等级**（4 vCPU / 8 GB 内存 / 本地 SSD）：持续 5,000 msg/s，发布 P99 ≤ 10 ms。以 32 核 24h soak（4.32 亿条 @ 固定 5,000 msg/s，P99 0.915 ms）为锚；该档由 4 核级 24h soak 正式化。
> - 单 Queue 容量核算遵循 `capacity-planning.md`；峰值入口流量须低于所在档位持续包络的 70%。

## D.2 门禁推导

| 档位 | 声明包络 | 24h soak 门禁（inaugural 模式） | 推导依据 |
|---|---|---|---|
| 2C/4G | 3,000/s，P99 ≤ 10 ms | `-MinPublishMessagesPerSecond 3000 -MinConsumeMessagesPerSecond 3000 -MaxPublishLatencyP99Millis 10` | 天花板 6,171/s 的 49%（< 70% 规则），突发余量至 ~4,300/s |
| 4C/8G | 5,000/s，P99 ≤ 10 ms | `-MinPublishMessagesPerSecond 5000 -MinConsumeMessagesPerSecond 5000 -MaxPublishLatencyP99Millis 10` | 2 核天花板 6,171/s 外推 4 核 ≥ 6,171/s（5,000/s 占比 ≤ 81%，有真实余量）；32 核 24h 锚点证明软件可 5,000/s 持续 |

## D.3 24h soak 命令序列

**档位 1（2C/4G，jdcloudremote，可立即执行）：**

```bash
# 0) 本地：以最终冻结修订重建 linux/amd64 二进制并传输（校验和核验，不传源码）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/rjs-management ./management/cmd/rjs-management
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/rjsctl ./tools/rjsctl
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/nats-server ./upstream/nats-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/jetstream-bench ./tests/helpers/jetstream-bench
# sha256sum + scp + 主机端校验（同 §C.1 流程）

# 1) 主机（sandbox）：以最终修订二进制重建三节点集群（端口 4222-4224/6222-6224/8222-8224，
#    数据目录 nats-data/calib-<rev>-{1,2,3}，先停 drill 遗留集群）

# 2) 24h 基准 + 健康采样（systemd-run 瞬态单元，非 root 动态用户）
runuser -u sandbox -- systemd-run --uid=sandbox --unit=rjs-soak-tier1 \
  /home/sandbox/rc3/calib/jetstream-bench \
  --server nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224 \
  --output /home/sandbox/rc3/calib/soak-tier1.json \
  --payload-bytes 1024 --publishers 8 --batch 256 --replicas 3 \
  --messages 0 --duration 24h --timeout 30m
# 期间按 tests/soak/rc3-bare-metal.sh 的监督器模式采集 health/queues_api 周期证据

# 3) 验收：soak-tier1.json 满足 integrity=0 缺陷、吞吐 ≥ 3000/s、P99 ≤ 10 ms；
#    perfevidence -create-inaugural 产出标准证据清单（绑定最终修订）
```

**档位 2（4C/8G，待主机）：** 同上序列，主机换 4C/8G，门禁换 5,000/s。主机就绪前该档以"32 核 24h 锚点 + 2 核标定外推"标注为 *pending formalization*，不作为已认证声明。

## D.4 证据状态

| 档位 | 声明 | 现有证据 | 缺口 |
|---|---|---|---|
| 2C/4G | 3,000/s，P99 ≤ 10 ms | 三档标定探针（46–96s）+ 42h 稳定性 soak | **24h 吞吐 soak 已启动（见 D.5）** |
| 4C/8G | 5,000/s，P99 ≤ 10 ms | 32 核 24h 锚点（旧修订）+ 2 核标定外推 | 4C 级主机 + 24h soak |

## D.5 档位 1 soak 启动记录（2026-10-04 21:40 CST）

- **单元**：`rjs-soak-tier1.service`（systemd 瞬态单元，sandbox 身份），启动 2026-10-04T21:40:26 CST，预计完成 **2026-10-05 21:40 CST**。
- **候选绑定**：全部组件以最终修订 `616b66d` 干净树本地交叉构建（nats-server 按 qualified 配方含 crypto v0.55.0 覆盖，构建后子树还原），SHA256SUMS 传输后主机端核验一致；`revision.txt` 记录于 `/home/sandbox/rc3/soak-tier1/bin/`。
- **拓扑**：三节点 JetStream 集群 `rjs-tier1`（4222-4224 / 6222-6224 / 8222-8224，数据目录 soak-tier1/nats-{a,b,c}-data）+ 管理面（cluster profile，127.0.0.1:8300）+ 24h bench（8 发布者、1 KiB、batch 256、R3、duration 模式）。drill 遗留的 root 属主 canary 集群已按计划停止。
- **启动核验**：cycle 1-2 即 health=200、queues_api=200、nats_up=3/3、bench=up；jsz 显示 ha_assets=3（R3 流已成形）、API 请求速率 ~5k/s（PubAck 逐条确认流量正常）。
- **验收标准**（完成后核对 `soak-tier1.json` + 证据日志）：integrity 零丢失/重复/损坏；吞吐 ≥ 3,000 msg/s；P99 ≤ 10 ms；health/queues_api 全周期 200；nats 零崩溃、mgmt 零非受控重启。
- **查看进度**：`ssh jdcloudremote 'tail -5 /home/sandbox/rc3/soak-tier1/logs/soak-evidence.log'`
