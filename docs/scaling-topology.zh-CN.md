# 扩容与拓扑变更

[English](scaling-topology.md) | [简体中文](scaling-topology.zh-CN.md)

变更 Rabbit JetStream 部署规模或拓扑的运维流程：Queue 副本变更、NATS 集群节点操作、management 副本变更与 R 级迁移。本文是 [Deployment](deployment.zh-CN.md)、[Kubernetes](kubernetes.md) 与 [Capacity Planning](capacity-planning.zh-CN.md) 的补充。

## 任何拓扑变更之前

1. 确认近期备份可用：`rjsctl backup verify --input <dir>`；没有成功恢复演练的备份不算可恢复（见 [Backup and Restore](backup-restore.md)）。
2. 复查 [Capacity Planning](capacity-planning.zh-CN.md)：存储余量、清空速率、告警阈值在副本/保留/节点变更后必须重查。
3. 维护窗口内冻结无关拓扑变更（[Incident Triage](operations.zh-CN.md) 规则 1 同样适用于计划变更）：一次一个成员，绝不同时重启多个 NATS 成员。
4. 采集变更前状态：`rjsctl status`、`/api/v1/cluster`、`kubectl -n <ns> get pods` 及诊断包（`rjsctl diagnostics collect`）。

## Queue 副本变更（1 ↔ 3 ↔ 5）

Queue 副本是声明式 Queue 文档的一部分（`spec.replicas`），必须经预览-确认控制面变更，绝不直接操作 JetStream。

1. 修改 Queue 文档（Admin UI "编辑草稿与预览"、`rjsctl queue diff` 或 API `POST /api/v1/queues/{queue}/preview`）。
2. 仔细阅读预览计划：**若计划报告 `recreate` 操作，该变更是破坏性的**——Stream 身份或存储语义发生变化，消息无法保留。recreate 类变更需要走备份路径（见下文）或在工单中记录并接受数据丢失决策。
3. 仅对安全（update 类）计划执行条件写入（`If-Match`），然后验证：`/api/v1/queues/{queue}` 显示新副本数且每个副本 `current`，Queue 告警保持安静。

对已含数据的 Queue 提高 `spec.replicas` 可能被计划判定为 `recreate`，因为 JetStream Stream 无法就地修改副本数。需要保留数据的 R 级变更走下文的迁移路径。

## NATS 集群节点操作（Kubernetes）

- **扩容**：`kubectl -n <ns> scale statefulset <release>-rabbit-jetstream-nats --replicas=<n>`。新 Pod 加入 NATS 集群与 JetStream 元数据 quorum，但**既有 Stream 不会自动重平衡到新 Pod**——副本位置在 Stream 创建时确定。既有 Queue 保持现有冗余度，直到其声明变更（见上文）。
- **缩容**：绝不在未摘除 peer 成员的情况下删除持有 JetStream peer 的 Pod。先用 NATS CLI 摘除 peer（operator 镜像内的 `nats server cluster peer step-down` / `peer remove`），确认 `jsz` 中所有 Stream 均无缺失副本，再缩容 StatefulSet。直接删 Pod 会留下丢失的 peer，阻塞 quorum 敏感操作。
- 缩容时 PVC 按设计保留；保留策略见 [Kubernetes](kubernetes.md)。

## NATS 集群节点操作（裸金属 / Compose）

裸金属节点使用每节点独立配置（`deploy/nats/cluster-N.conf`）：相同 `cluster.name`、唯一 `server_name`、独立的客户端/监控/集群端口，且每个节点都配置完整 route 列表。

- **新增节点**：准备存储，复制 stanza 模板并填新 `server_name` 与端口，把新 route 追加到所有既有节点的 `routes`，逐个滚动重启既有成员（保持 quorum），最后启动新节点。确认元数据 leader 报告所有 peer current 且已追平（与 [baremetal-qualification.md](baremetal-qualification.zh-CN.md) 相同的预热规则）后才能宣布变更完成。
- **摘除节点**：先摘除其 JetStream peer 并用 NATS CLI 移除成员让集群遗忘该节点；滚动重启更新其余节点的 `routes`；最后才停旧进程。绝不用删除节点数据目录的方式让故障成员重新启动（[Incident Triage](operations.zh-CN.md) 规则 5）。

## Management 副本

management 服务除 NATS 连接外无状态：两个方向都可以安全扩缩。Helm 默认两个副本挂在 Service 之后；裸金属每主机一个进程，应置于负载均衡之后（[Deployment](deployment.zh-CN.md)）。扩缩后验证每个副本的 `readyz`，并确认 `/api/v1/cluster` 上报预期 profile。

## R 级迁移（R3 → R5，或在其他 R 的集群恢复）

JetStream 副本数在 Stream 创建时固定，保留数据的 R 级变更是备份 → 恢复周期：

1. 对当前集群 `rjsctl backup create --output <dir>`，随后 `rjsctl backup verify --input <dir>`。
2. 以目标 R 部署新集群（三个或五个 JetStream 节点）。
3. `rjsctl backup restore --input <dir> --confirm RESTORE --replicas 5`——恢复要求空账户且写入者与控制器已停止，且跨 Stream 不具事务性（[Backup and Restore](backup-restore.zh-CN.md)、[remaining-release-work](remaining-release-work.md)）。
4. 用 `rjsctl migrate capture` + `reconcile` 或 canary 对账器核对，再切换客户端。

## 每次变更后的验证

1. 对每个被触碰的成员执行 `kubectl rollout status`（K8s）或进程/健康检查（裸金属）。
2. `/api/v1/cluster` 与 `jsz` 显示所有副本 `current`、无缺失 peer。
3. Canary 对账器（见 `tests/soak/`）：发布一小段突发流量，确认零丢失/零损坏且管理 5xx = 0。
4. 观察随货告警一个求值周期（JetStream 可用性、控制器停摆、节点可用性），确认安静。
5. 按 [Escalation Evidence](operations.zh-CN.md) 要求记录变更窗口、digest 与证据。
