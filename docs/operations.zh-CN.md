# 运维手册

[English](operations.md) | [简体中文](operations.zh-CN.md)

## 日常巡检

```bash
rjsctl status --url http://management:8223
rjsctl diagnostics collect --url http://management:8223 --output diagnostics.zip
curl -fsS http://management:8223/readyz
curl -fsS http://management:8223/metrics
```

检查集群可用性、JetStream 存储余量、非 current 副本、Queue 积压、重投、DLQ 失败、controller leader、路由重连、API 错误和证书到期时间。每次 Queue 变更都应核对审计事件，并按[容量规划](capacity-planning.md)在达到阈值前告警。

## 故障处置

1. 冻结拓扑变更，禁止同时重启多个 NATS 成员。
2. 保存 readiness、节点、集群、Stream、controller 状态并收集诊断包。
3. 判断故障仅影响控制面，还是已经影响发布/消费确认。
4. 保留日志、指标、审计事件、镜像 digest、配置哈希和时间线。
5. 每次只恢复一个成员；禁止通过删除数据卷“修复”启动失败。
6. 数据完整性不确定时隔离集群，执行[备份恢复](backup-restore.md)流程。

Admin UI 仅用于提高操作效率；故障期间以 API、CLI、指标和审计数据为准。

## 备份恢复

按计划创建 account 快照，加密并复制到集群外，定期执行恢复演练。保留或恢复前验证 manifest。Kubernetes 认证/TLS Secret 不包含在 JetStream 快照中，必须独立备份。未通过恢复演练的备份不能视为可恢复。

## 升级回滚

变更前确认近期备份、磁盘余量、current 副本、不可变的新旧镜像以及已测试的回滚命令。逐个升级 NATS follower，每一步等待全部副本恢复 current，再处理 leader 和管理副本。仲裁丢失、副本持续落后、消息不一致或 SLI 越界时立即停止并回滚。详见[滚动升级与回滚](upgrade-rollback.md)。

## 凭据轮换

Token 或 OIDC 签名密钥应重叠轮换：添加新凭据、验证新旧凭据、迁移客户端、移除旧凭据，再验证旧凭据已拒绝并保存审计证据。禁止将凭据写入诊断包、命令历史、Git 或 NATS URL。参见[凭据轮换](credential-rotation.md)和 [OIDC](oidc.md)。

## 升级支持所需证据

提供诊断 ZIP、Release/Tag 与镜像 digest、UTC 故障时间窗、受影响 Queue、客户端确认错误、节点事件、存储状态和已执行操作。未经明确批准并加密，不得附带消息正文。

扩容与拓扑变更（增减节点、副本变更）遵循[扩容与拓扑变更](scaling-topology.zh-CN.md)。
