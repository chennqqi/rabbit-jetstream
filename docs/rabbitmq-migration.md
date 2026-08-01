# RabbitMQ Topology Migration

RabbitMQ definitions are topology metadata, not message data. Export them with `rabbitmqctl export_definitions` or the management API, protect the file as sensitive, then convert one virtual host at a time:

```shell
rjsctl migrate rabbitmq-definitions \
  --vhost / --replicas 3 --output converted definitions.json
```

The command writes one strict Queue YAML per compatible queue plus `migration-report.json`. It exits non-zero when any semantic cannot be mapped. `--allow-lossy` writes the compatible subset but does not make the report compatible; use it only for reviewed planning artifacts.

Mapped fields include durable queues, direct/topic/fanout queue bindings, `x-message-ttl`, `x-max-length`, `x-max-length-bytes`, classic/quorum queue type, and an unambiguous fanout/direct dead-letter exchange resolving to one Queue. Queues without a mapped exchange binding receive `rjs.migrated.<queue>` native ingress and a warning.

The converter rejects transient/auto-delete queues, headers or plugin exchanges, exchange-to-exchange bindings, exchange/binding arguments, RabbitMQ stream queues, unsupported queue arguments, ambiguous DLX routing, invalid names, and effective policies. Flatten policies into explicit queue/exchange settings before conversion. These failures are deliberate: silently approximating routing or lifecycle behavior is unsafe for production messaging.

RabbitMQ documents definitions as exported cluster/vhost schema metadata and direct, topic, and fanout as distinct routing algorithms; the converter preserves only semantics representable by this project's Queue contract. See the official [definitions guide](https://www.rabbitmq.com/docs/definitions) and [exchange guide](https://www.rabbitmq.com/docs/exchanges).

Validate every generated declaration before apply:

```shell
for file in converted/*.yaml; do rjsctl queue validate "$file"; done
```

Definitions contain no queued messages. Follow [Message Migration, Shadow Verification, and Cutover](migration-cutover.md) for the observation contract, strict reconciliation gate, dual-publish outbox requirements, cutover, and rollback. Never delete the RabbitMQ source topology until message counts, business-level checksums, redelivery behavior, and rollback acceptance criteria pass.
