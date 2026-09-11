# Diagnostic Bundles

[English](diagnostics.md) | [简体中文](diagnostics.zh-CN.md)

`rjsctl diagnostics collect` captures a point-in-time support bundle from the management API without accessing JetStream data-plane messages.

```bash
rjsctl diagnostics collect \
  --url http://127.0.0.1:8223 \
  --output rabbit-jetstream-diagnostics.zip
```

The ZIP contains health, readiness, service/account, cluster, node, Queue, Stream, controller and Prometheus snapshots plus `manifest.json`. The manifest records the collection time, HTTP status, byte length and SHA-256 digest of every included file. Valid, redacted non-2xx JSON responses are retained because they are often useful failure evidence. A failed endpoint does not prevent collection of the remaining endpoints. Queue and Stream snapshots request only the first page, with a limit of 200; the bundle is not a complete account export or an atomic snapshot.

## Security and Operational Limits

- Existing output files are never overwritten.
- Final publication atomically creates a hard link to the completed temporary archive in the same directory. A destination created during collection is also preserved. The output filesystem must support hard links; otherwise publication fails without falling back to an overwriting rename.
- Each response is limited to 4 MiB to bound local memory and bundle size.
- JSON fields whose names contain authorization, credential, password, secret or token are replaced with `[REDACTED]`.
- JSON number tokens retain their precision, including uint64/int64 counters and revisions above JavaScript's safe-integer range. Redaction does not convert them through floating point.
- Known JSON sources are parsed even if they report the wrong content type. Invalid JSON is omitted rather than archived unredacted; its manifest entry retains HTTP status and a static error, without a file, size or digest. Other sources continue collecting.
- User information is removed from absolute URLs in API responses, manifests and request errors.
- The management API itself removes credentials from reported NATS URLs.

Review a bundle before sharing it outside the operator trust boundary. Queue and Stream names, cluster topology, resource usage and error messages are operationally sensitive even after credential redaction. Diagnostic bundles do not contain message payloads, NATS credentials files or environment variables.

Field-name and absolute-URL redaction is not a general secret detector for arbitrary text, embedded URLs or query parameters. Prometheus text is not processed as JSON. Do not treat this CLI collector alone as authorization for browser downloads: a WebUI endpoint still needs bounded task/storage lifecycle, access policy, expiry and access auditing. No such endpoint is provided by this change.
