# Queue Declaration Schema

`rabbit-jetstream.io/v1alpha1` is the first versioned logical Queue contract. It is deliberately independent from raw NATS Stream JSON so validation, migration, Admin UI, and SDK naming can share one stable model.

```bash
rjsctl queue validate examples/queues/orders.yaml
rjsctl queue diff current.yaml desired.yaml
```

Parsing is strict: unknown fields, multiple YAML documents, malformed durations, invalid byte sizes, duplicate subjects, and invalid NATS wildcard placement are rejected. Subjects are normalized into lexical order before comparison.

## Current Fields

- `metadata.name`: letters, digits, `_`, and `-`; maps to logical resource identity.
- `spec.subjects`: one or more NATS subjects; `*` must occupy a whole token and `>` must be final.
- `spec.replicas`: `1`, `3`, or `5`. Production clusters normally use `3`.
- `spec.storage`: `file` by default, or `memory`.
- `retention`: optional `maxAge`, `maxBytes`, and `maxMessages`; zero means unlimited.
- `delivery`: defaults to `ackWait: 30s` and `maxDeliver: 5`.
- `deadLetter.queue`: optional logical DLQ and cannot reference the queue itself.

Byte sizes accept binary units (`KiB`, `MiB`, `GiB`), decimal units (`KB`, `MB`, `GB`), bytes, or a raw integer. Fractional and negative values are rejected.

## Diff Impact

Diff output is deterministic and classifies every change:

- `safe`: labels, added subjects, or expanded retention limits;
- `disruptive`: replicas, delivery behavior, DLQ, or removed subjects;
- `destructive`: identity, storage type, or reduced retention limits.

This stage performs no JetStream writes. A later controller will require explicit confirmation or policy approval for destructive changes.
