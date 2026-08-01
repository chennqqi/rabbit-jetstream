# Queue to JetStream Mapping

`rjsctl queue plan` converts a validated logical Queue into deterministic JetStream resources without contacting the server.

| Logical resource | JetStream plan |
|---|---|
| Queue `orders` | Stream `RJSQ_orders` |
| Competing consumer group | Durable pull Consumer `RJSQC_orders` |
| Subjects | Stream subjects and Consumer filter subjects |
| Storage and replicas | Stream storage and replica count |
| Retention limits | Stream `MaxAge`, `MaxBytes`, and `MaxMsgs` |
| Delivery | Explicit Ack, AckWait, MaxDeliver, instant replay |
| DLQ | Dependency Stream plus server-side advisory republisher |

The Stream uses WorkQueue retention. All SDK instances consuming the same logical Queue share the durable Consumer, which provides competing-consumer behavior. Resource names preserve Queue name case to avoid collisions.

```bash
rjsctl queue plan examples/queues/orders.yaml
rjsctl queue reconcile --url http://127.0.0.1:8223 examples/queues/orders.yaml
```

Every plan includes a deterministic 128-bit revision derived from the normalized Queue document. The revision and logical Queue identity are copied into resource metadata. Subject ordering does not change the revision.

## Important DLQ Boundary

JetStream `MaxDeliver` stops delivery attempts but does not automatically move the message to another Stream. A Queue with `deadLetter` therefore plans an `advisory-republish` worker and declares the target Queue as a dependency. Until that server-side worker exists, apply must reject DLQ-enabled plans rather than silently promise RabbitMQ DLX behavior.

This mapping intentionally covers Queue semantics only. Exchange-style direct/topic/fanout declarations and priority subjects require separate versioned contracts before they can enter an apply controller.

## Read-only Reconcile

`queue reconcile` reads the Stream and Consumer from the management API and emits ordered operations:

- `create`: the resource is absent;
- `update`: mutable fields differ;
- `noop`: observed and desired state match;
- `recreate`: immutable or destructive identity/storage semantics differ;
- `reject`: the requested capability is not implemented.

The result is `blocked` for destructive retention reductions, recreation, or unsupported DLQ workers. Reconcile only performs HTTP GET requests and never writes JetStream resources.

## Authenticated Apply

Set `RJS_ADMIN_TOKEN` on the management service to enable the write endpoint. With no token configured, the endpoint returns 404 and the deployment remains read-only. Prefer passing the CLI token through the environment so it is not exposed in the process list:

```bash
export RJS_ADMIN_TOKEN='replace-with-a-secret-manager-value'
rjsctl queue apply --url http://127.0.0.1:8223 examples/queues/orders.yaml
```

Apply runs the same reconcile first. It executes only `create`, safe `update`, and `noop` operations; blocked recreation, retention reduction, and DLQ plans return HTTP 409 without writes. Creation is recoverable rather than transactional: if Consumer creation fails after Stream creation, repeating apply resumes from the observed partial state.

## Safe Delete

Deletion requires the admin token and an exact Queue-name confirmation. It only removes Streams carrying matching `rabbit-jetstream.io/queue` ownership metadata. Non-empty Streams return HTTP 409 unless `--force` is supplied; missing resources return `noop`.

```bash
rjsctl queue delete --confirm orders orders
rjsctl queue delete --confirm orders --force orders
```
