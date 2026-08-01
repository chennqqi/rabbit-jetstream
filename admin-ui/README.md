# Admin UI

The dependency-free administration console is embedded into `rjs-management` and served at `/admin/`. It consumes only the versioned management HTTP API and never connects to NATS directly.

The console covers service and cluster health, JetStream capacity, Queue declarations, Stream backlog, consumer delivery state, controller leadership, and DLQ transfer counters. Operators can also create, update, and explicitly confirm Queue deletion through the versioned API. Bearer credentials remain in page memory and are never written to browser storage; server-side RBAC, audit, ownership checks, and ETag preconditions remain authoritative. Keep source assets in `dist/`; tests ensure the embedded files and mutation safety contract remain available in the production binary.
