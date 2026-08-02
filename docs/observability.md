# Observability

The management service exposes Prometheus text format at `GET /metrics`. Metrics cover service uptime, JetStream reachability and capacity, declared Queues, per-Queue message/byte backlog, NATS node availability, controller leadership, blocked declarations, DLQ outcomes, and HTTP request rate/duration.

HTTP labels use Go route patterns such as `GET /api/v1/queues/{queue}` rather than raw URLs, preventing Queue names from creating unbounded request-series cardinality. Per-Queue backlog metrics intentionally carry a `queue` label because each declaration is an operator-managed resource.

Start the optional pinned Prometheus service with Docker Compose:

```bash
docker compose -f deploy/compose/standalone.yml --profile observability up -d
```

Prometheus is available on port `9090`; its management target is `management:8223`. Configuration and alert rules are in `deploy/observability/`. Validate them before release:

```bash
promtool check config deploy/observability/prometheus.yml
promtool check rules deploy/observability/alerts.yml
```

The default alerts cover JetStream loss, a stalled elected controller, unavailable NATS nodes, DLQ transfer failures, sustained Queue backlog above 100,000 messages, and management API 5xx rates above 5%. Backlog thresholds must be tuned using workload capacity and drain-rate tests.

`/metrics` is intentionally unauthenticated for cluster-local scraping. Bind the management service to a private network or protect it with an ingress/network policy; never expose operational endpoints directly to an untrusted network.

For Helm and a Prometheus Operator in another namespace, enable `serviceMonitor.enabled` together with `networkPolicy.monitoring.enabled`, then set `networkPolicy.monitoring.namespaceSelector` and `networkPolicy.monitoring.podSelector` to the exact labels of the Prometheus namespace and Pods. The chart fails rendering when ServiceMonitor is enabled without this explicit peer, preventing a silently unreachable scrape target.
