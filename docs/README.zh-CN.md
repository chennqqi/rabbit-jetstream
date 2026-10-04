# 文档地图

[English](README.md) | [简体中文](README.zh-CN.md)

从这里开始。文档按受众分组；双语对照使用 `.zh-CN.md` 后缀。

## 我使用管理控制台（运营、值班）

| 文档 | 用途 |
|---|---|
| [管理控制台用户手册](console-user-guide.zh-CN.md) | 任务导向手册：登录、各页面、状态徽章、Queue 生命周期、审计、诊断。 |
| [故障排查指南](troubleshooting.zh-CN.md) | 控制台/API/SDK/CLI 错误的"现象 → 原因 → 处置"。 |
| [术语表](glossary.zh-CN.md) | 全局术语（Queue、Stream、Plan 修订、DLQ、租户……）。 |
| [运维告警](webui-operational-alerts.zh-CN.md) | 告警页的规则与状态语义。 |
| [消费者诊断](webui-consumer-diagnosis.zh-CN.md) / [DLQ 诊断](webui-dlq-diagnostics.zh-CN.md) | 积压与死信排查页面。 |

## 我负责部署运维（平台 / SRE）

| 文档 | 用途 |
|---|---|
| [部署指南](deployment.zh-CN.md) | 单机、Compose 集群、Kubernetes 安装。 |
| [Kubernetes](kubernetes.md) | Chart 细节、生产清单、滚动变更。 |
| [扩容与拓扑变更](scaling-topology.zh-CN.md) | Queue 副本、节点增减、management 副本、R 级迁移。 |
| [Operations](operations.md) + [运维配套清单](operations-companion.zh-CN.md) | 例行检查、事件排查、备份、升级回滚、告警到人、值班 runbook。 |
| [可观测性](observability.md) | 指标、Prometheus、随货告警。 |
| [配置](configuration.md) | 全部 `RJS_*` 配置项。 |
| [容量规划](capacity-planning.md) | 规格、阈值、排空测试。 |
| [备份与恢复](backup-restore.md) / [升级与回滚](upgrade-rollback.md) / [凭据轮换](credential-rotation.md) / [诊断](diagnostics.zh-CN.md) | operations.md 各节的深入文档。 |
| [本地账户](local-auth.zh-CN.md) / [OIDC](oidc.zh-CN.md) / [多租户](multi-tenancy.zh-CN.md) | 身份与隔离。 |

## 我基于 SDK 开发应用

| 文档 | 用途 |
|---|---|
| [SDK 入门](sdk-quickstart.zh-CN.md) | 端到端第一个优先级队列应用（声明 → 发布 → 消费）。 |
| SDK 仓库 `outlink/rabbit-jetstream-go` | 双语 README、`docs/api-v0.1.md`（API 契约）、`examples/priority`。 |
| [Native SDK 契约](native-sdk-contract.md) / [API 版本化](api-versioning.md) | 线级契约与稳定性规则。 |
| [客户端迁移](client-migration.md) | 从 RabbitMQ 迁移客户端。 |

## 我开发产品本身

| 文档 | 用途 |
|---|---|
| [架构](architecture.md) / [Queue 映射](queue-mapping.md) | 系统原理；RabbitMQ 概念映射。 |
| [Management API](management-api.md) + `api/openapi.yaml` | 端点参考；代码生成见 [openapi-codegen](openapi-codegen.zh-CN.md)。 |
| [测试](testing.zh-CN.md) / [性能测试](performance-testing.md) | 门禁清单与工作负载。 |
| WebUI 文档（`docs/webui-*.md`） | 控制台功能规格与资格记录。 |
| [发布说明](releases/) + [剩余发布工作](remaining-release-work.md) | 各候选证据与未关门禁。 |

## 治理与评审记录

[发布审批模板](release-approval.template.json)、`docs/releases/` 下的已签字记录（rc.3 的 soak、集群、canary/节点故障/回滚证据）、[运维能力评审](ops-review.zh-CN.md)、[控制台产品评审](../admin-ui/design-review.zh-CN.md)、[改进计划](review-improvement-plan.zh-CN.md)。

## AI Agent

自主操作的 Agent 技能在 `skills/`（`rjsctl-operations`、`rjs-incident-response`、`rjs-release-drills`、`rjs-console-automation`）；仓库工作约定见 [AGENTS.md](../AGENTS.md)。
