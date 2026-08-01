# Management Audit Trail

Rabbit JetStream records every authenticated, validated Queue apply or delete as durable JetStream events. The management service writes an `intent` event before changing Queue state. If that write fails, the mutation is rejected. It writes a correlated `outcome` event after execution; if this second write fails, the API returns `503` and tells the operator to inspect resource state before retrying because the mutation result is uncertain.

Events are stored in the file-backed `RJS_AUDIT_EVENTS` Stream on `rjs.audit.events`. The Stream uses the configured metadata replica count, S2 compression, a 365-day/1 GiB bound, and denies normal message deletion and purge operations. It is included in full-account backups.

Each event contains a request ID, intent ID, phase, action, resource, outcome, HTTP status, revision, timestamp, source address, and actor fingerprint. The actor is a SHA-256 fingerprint of the bearer token; the credential itself is never persisted. Source addresses come from the direct network peer, not untrusted forwarding headers.

Query newest events first with an admin token:

```shell
RJS_ADMIN_TOKEN=... rjsctl audit list --url http://127.0.0.1:8223 --offset 0 --limit 100
```

The equivalent protected endpoint is `GET /api/v1/audit`. Limits are 1–200.

`DenyDelete` and `DenyPurge` protect normal operations but are not cryptographic non-repudiation: a sufficiently privileged NATS account administrator can remove the entire Stream. Compliance deployments should continuously export audit events to an external append-only or WORM system and restrict NATS administrative credentials.
