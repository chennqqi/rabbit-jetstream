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
