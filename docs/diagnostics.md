# Diagnostic Bundles

`rjsctl diagnostics collect` captures a point-in-time support bundle from the management API without accessing JetStream data-plane messages.

```bash
rjsctl diagnostics collect \
  --url http://127.0.0.1:8223 \
  --output rabbit-jetstream-diagnostics.zip
```

The ZIP contains health, readiness, service/account, cluster, node, Queue, Stream, controller and Prometheus snapshots plus `manifest.json`. The manifest records the collection time, HTTP status, byte length and SHA-256 digest of every included file. Non-2xx responses are retained because they are often the most useful failure evidence. A failed endpoint does not prevent collection of the remaining endpoints.

## Security and Operational Limits

- Existing output files are never overwritten.
- Each response is limited to 4 MiB to bound local memory and bundle size.
- JSON fields whose names contain authorization, credential, password, secret or token are replaced with `[REDACTED]`.
- User information is removed from absolute URLs in API responses, manifests and request errors.
- The management API itself removes credentials from reported NATS URLs.

Review a bundle before sharing it outside the operator trust boundary. Queue and Stream names, cluster topology, resource usage and error messages are operationally sensitive even after credential redaction. Diagnostic bundles do not contain message payloads, NATS credentials files or environment variables.
