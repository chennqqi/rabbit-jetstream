# 运维配套清单

[English](operations-companion.md) | [简体中文](operations-companion.zh-CN.md)

状态：随改进计划 A3 交付 · 日期：2026-10-01
适用：[运维能力评审](ops-review.zh-CN.md)确定的四个断链环节——告警到人、消息级排障旁路、备份调度、值班流程。

控制台的边界是刻意设计：管理面只管队列声明与只读观测，不进入消息数据路径。因此以下配套是**部署即依赖**的组件与流程；本文给出可直接照做的基线，团队按规模扩展。

---

## 1. 告警到人（必须有，否则告警只是页面上的颜色）

链路：管理服务 `/metrics` → Prometheus（抓取 + 求值 `deploy/observability/alerts.yml` 六条规则）→ **Alertmanager（通知）** → 值班端点。

随货交付：
- `deploy/observability/alertmanager.yml`：基线路由（critical 10 秒分组、4 小时重复），默认 webhook 接收器**必须替换**为团队真实端点（Slack/PagerDuty/钉钉/邮件等）；
- `deploy/compose/standalone.yml` 的 `observability` profile 已包含 `alertmanager` 服务（9093），`prometheus.yml` 已配置 alertmanager 挂接。

启用步骤：

```bash
docker compose -f deploy/compose/standalone.yml --profile observability up -d
# 验证：Prometheus http://localhost:9090/alerts 可见规则；Alertmanager http://localhost:9093 收到告警
```

验证通知：临时把 `RabbitJetStreamQueueBacklogHigh` 阈值调低或在 Alertmanager UI 用 `_testOnly` 推一条告警，确认值班端点收到。**配置语法尚未用 amtool 深度校验**（评审环境无法拉取镜像），部署前执行：

```bash
docker run --rm -v $PWD/deploy/observability/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro prom/alertmanager:v0.28.1 amtool check-config /etc/alertmanager/alertmanager.yml
```

调优提示：`RabbitJetStreamQueueBacklogHigh` 默认 10 万条持续 15 分钟是"防呆"阈值，按各队列实际吞吐调整（可为高吞吐队列单独加规则）。

## 2. 消息级排障旁路（管理面刻意不做，必须有替代路径）

控制台**看不到也无法操作**单条消息。以下操作全部通过 `nats` CLI（[nats-box 容器](https://github.com/nats-io/nats-box)或本地安装）直连 JetStream 完成：

```bash
# 进入带 nats CLI 的容器（镜像按内网源调整）
docker run --rm -it --network <compose-network> natsio/nats-box:latest
nats context save rjs --server nats://nats:4222

# 场景：看某队列（Stream）的最新消息（只读浏览）
nats stream info RJSQ_orders-primary
nats stream view RJSQ_orders-primary --last 5

# 场景：确认某条消息是否存在
nats stream get RJSQ_orders-primary --last --raw --subject "orders.created"
```

死信重放样例（**先与业务方确认语义再执行**；重放是发布新消息，不删除死信原件）：

```bash
# 1) 找到死信 Stream 里的待处理消息
nats stream info RJSQ_dead-letters
nats stream view RJSQ_dead-letters --last 10 --json
# 2) 把选定消息重新发布到原 subject（--raw 消息体见上一步输出）
nats pub orders.created '<original-payload>'
# 3) 复核：原队列待投递增长、死信 Stream 按保留策略老化
```

约定：重放类操作必须记录到工单（包含消息 Subject、Stream 序号、执行人、时间）；`nats` CLI 直接操作数据面，不经过管理面审计，工单即审计。

## 3. 备份调度（CLI 能力，需自行调度）

`rjsctl backup` 是单机工具链，随货无调度。基线 cron（每日 02:30 备份 + 校验，保留 7 天）：

```cron
30 2 * * *  /opt/rjs/bin/rjsctl backup create --output /var/backups/rjs/$(date +\%F) --server http://127.0.0.1:8223 && /opt/rjs/bin/rjsctl backup verify --input /var/backups/rjs/$(date +\%F) >> /var/log/rjs-backup.log 2>&1
0 4 * * *   find /var/backups/rjs -maxdepth 1 -mtime +7 -exec rm -rf {} \;
```

约束（来自工具边界）：`backup restore` 要求**空账户且停止写入者与控制器**，无跨 Stream 原子性——恢复是灾备流程不是日常操作，恢复演练每季度做一次并记录。

## 4. 值班 runbook（三场景）

### 场景 A：队列积压（告警 `RabbitJetStreamQueueBacklogHigh` 或控制台看到待投递增长）
1. **定位**：控制台 → Queue 列表按消息数排序；Queue 详情摘要看"待投递（主 Consumer）"。
2. **判断**：Consumer 页看该队列 Consumer 的待投递/待确认与重投——待投递高 + 待确认 0 通常是消费者离线；待确认同步高是消费端处理卡住。
3. **处置**：消费者侧修复（重启消费进程/扩容消费者）；**控制台没有任何处置动作，这是设计边界**。
4. **复核**：控制台待投递数字回落；历史指标页看 `Queue 存储消息` 15m/1h/24h 趋势。

### 场景 B：死信（告警 `RabbitJetStreamDeadLetterFailures` 或 DLQ 诊断页异常）
1. **定位队列**：控制台 DLQ 诊断（Queue 详情 → 配置 → DLQ 诊断）看目标 Stream/控制器状态六类证据。注意：传输计数是进程级全局指标，A12 改造前**无法直接按队列定位**——用 `nats stream info RJSQ_<队列名>` 逐个核对怀疑对象。
2. **评估**：死信消息的业务影响（丢消息 vs 重放）；`nats stream view RJSQ_dead-letters` 查看内容（脱敏注意）。
3. **处置**：按第 2 节样例重放或由业务方决定放弃；控制器会持续自动搬运新死信（at-least-once）。
4. **复核**：`rjs_dlq_failed_total` 停止增长（Alertmanager 恢复通知不可依赖——恢复态只在进程内存，重启即失）。

### 场景 C：删除 Queue 的不确定结局（删除请求返回 503/unknown）
1. **不要盲目重试**：删除流程对"结局未知"刻意锁定自动重试（防误删已删除的资源）。
2. **取证**：控制台 → Queue 详情 → 删除页下载"删除证据 JSON"（含请求 ID）；控制台 → 审计按该请求 ID 与时间窗口查 intent/outcome 配对记录。
3. **判定**：`nats stream info RJSQ_<名称>`——Stream 不存在 = 删除已生效；仍存在且消息数为 0 = 未执行，可在确认后重新走删除预检。
4. **记录**：把证据 JSON 与判定结论归档到工单；删除证据含配置与审计数据，按敏感文件保管。

---

## 5. 与控制台的分工速查

| 想做的事 | 控制台 | 旁路 |
|---|---|---|
| 看队列/消费者/节点状态 | ✔ 只读观测 | — |
| 队列创建/修改/删除 | ✔ 预览-确认 | `rjsctl queue apply/delete` |
| 查看一条消息内容 | ✖ | `nats stream view/get` |
| 重放死信 | ✖ | `nats pub`（工单审计） |
| 暂停消费者/清空队列 | ✖ | 消费者端应用 / `nats stream purge`（高危，双人在场） |
| 告警通知 | ✖ 只读投影 | Alertmanager |
| 备份/恢复 | ✖ | `rjsctl backup` + cron |
