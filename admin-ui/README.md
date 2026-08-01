# Admin UI

The dependency-free administration console is embedded into `rjs-management` and served at `/admin/`. It consumes only the versioned management HTTP API and never connects to NATS directly.

The current read-only MVP covers service and cluster health, JetStream capacity, Queue declarations, Stream backlog, consumer delivery state, controller leadership, and DLQ transfer counters. Keep source assets in `dist/`; tests ensure the embedded files remain available in the production binary.
