# SDK 入门：你的第一个优先级队列

[English](sdk-quickstart.md) | [简体中文](sdk-quickstart.zh-CN.md)

Native Go SDK（`github.com/chennqqi/rabbit-jetstream-go`）端到端教程：经控制面声明优先级 Queue、按优先级发布、以幂等处理器消费。完整 API 契约见 [SDK 仓库的 api-v0.1](../../outlink/rabbit-jetstream-go/docs/api-v0.1.md)；可运行示例在 `examples/priority`。

## 0. 前置条件

- 一个运行中的部署（见[部署指南](deployment.zh-CN.md)）：NATS `nats://127.0.0.1:4222`，管理服务 `127.0.0.1:8223` 且持有 operator 凭据。
- Go 1.25+，SDK 模块可用（`outlink/rabbit-jetstream-go`）。

## 1. 声明 Queue（控制面，而非 SDK）

生产拓扑由控制面持有。推荐控制台**创建 Queue**向导或 CLI：

```bash
rjsctl queue apply --url http://127.0.0.1:8223 --token <operator-token> orders.yaml
```

`orders.yaml`：

```yaml
apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata:
  name: orders
  labels: {team: commerce}
spec:
  subjects: ["orders.>"]
  replicas: 1            # 生产 R3 用 3
  storage: file
  maxPriority: 7         # 已取资格范围 0..7
```

核对生成的拓扑：Stream `RJSQ_orders`、主消费者 `RJSQC_orders`、每个优先级一个 durable 消费者（`rjs.q.orders.p.0..7`）。用控制台 Queue 详情或 `rjsctl queue list` 验证。

> SDK 中的 `EnsurePriorityQueue` 仅面向独立部署与测试——它校验但不重写拓扑。生产一律经控制面声明。

## 2. 按优先级发布

```go
client, err := rabbitjetstream.Connect(nats.DefaultURL)
if err != nil { log.Fatal(err) }
defer client.Close()

publisher, err := client.Publisher("orders", "queue-plan-revision", 7) // 优先级上界
if err != nil { log.Fatal(err) }

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
ack, err := publisher.Publish(ctx, rabbitjetstream.PublishMessage{
    ID:       "order-42",              // 用于去重的稳定 ID
    Priority: 9,                        // 0..max；会按 Queue 校验
    Data:     []byte(`{"order_id":42}`),
})
fmt.Printf("stream=%s sequence=%d duplicate=%t\n", ack.Stream, ack.Sequence, ack.Duplicate)
```

SDK 强制的规则：

- **每次 Publish 都需要稳定消息 ID**——at-least-once 投递意味着这个 ID 就是你的去重键。
- **Publish 返回即已获 JetStream PubAck**——没有 fire-and-forget；错误一律视为"尚未被接受"。
- **优先级会按 Queue 的 `maxPriority` 校验**后才上线。
- 契约保留 header 由 SDK 覆写；不要自行设置 `Nats-Msg-Id`、`Rjs-*`。

## 3. 幂等消费

```go
consumer, err := client.Consumer("orders")
if err != nil { log.Fatal(err) }

for {
    msg, err := consumer.Next(ctx) // 高→低扫描优先级缓冲，空优先级无网络往返
    if err != nil { /* handle */ }

    if err := handle(msg); err != nil {
        msg.Nak()               // 立即重投递
        continue
    }
    msg.Ack()                   // 应用工作落盘后才 Ack
}
```

投递语义：

- **At-least-once**：同一条消息可能到达多次（NAK、ack-wait 超时、重启）。用发布 ID 在应用侧去重。
- **工作落盘后才 Ack**；`Nak` 立即重试，`Term` 终止毒消息（不再重投——交给你的 DLQ 流程）。
- **优雅关闭时调用 Close**，缓冲消息会被 NAK 并由其他消费者迅速重投。
- **预取**（`ConsumerConfig.Prefetch`）是每进程本地缓冲预算；默认把 256 条均分到各级。高优先级消息到达时，低优先级消息可能已被预取分配——与 RabbitMQ 优先级队列的实际边界一致。

## 4. 在控制台验证

打开 Queue 详情：**存储消息数**应跟随你的发布速率；Consumer 显示待投递/待确认供积压排查；**审计**记录第 1 步的声明变更。

## 5. 下一步

- 保留策略、DLQ 策略与投递上限：在 `spec` 中声明（见 [Queue 文档 schema](webui-queue-schema.zh-CN.md) 与[术语表](glossary.zh-CN.md)）。
- 灾难恢复：`rjsctl backup create/verify/restore`（[备份与恢复](backup-restore.md)）。
- 从 RabbitMQ 迁移定义：[rjsctl migrate](rabbitmq-migration.md)。
- 报错排查：[故障排查指南](troubleshooting.zh-CN.md)。
