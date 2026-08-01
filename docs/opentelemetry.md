# OpenTelemetry Tracing

管理服务可将 HTTP 控制面 traces 和 HTTP runtime metrics 以 OTLP/HTTP protobuf 发送至 Collector。未配置对应 endpoint 时不会创建 exporter，Prometheus `/metrics` 继续并行提供现有业务指标。

```sh
export RJS_OTEL_TRACES_ENDPOINT=https://otel-collector.example/v1/traces
export RJS_OTEL_METRICS_ENDPOINT=https://otel-collector.example/v1/metrics
export RJS_OTEL_METRIC_INTERVAL=30s
export RJS_OTEL_SAMPLE_RATIO=0.1
```

服务提取 W3C `traceparent` 与 `baggage`。HTTP spans 包含标准请求属性；Queue apply/delete 额外记录 `rjs.queue.name`、revision 或 force。采样后的结构化日志包含 `trace_id` 和 `span_id`，写操作仍可通过审计 `request_id` 关联。退出时服务在 shutdown timeout 内 flush 批量 spans。

OTLP metrics 使用与 traces 相同的 `service.name`/`service.version` resource，并在退出时完成最后一次 collect/export。生产 endpoint 默认必须使用 HTTPS。只有隔离的 Docker 测试 Collector 才可设置 `RJS_OTEL_ALLOW_INSECURE=true`。Collector 不可与消息数据共用未经限制的公网出口；认证头和 CA 应由 Collector sidecar、代理或平台密钥机制管理，不能写入仓库。
