# Admin UI

This directory is reserved for the browser-based administration console. It consumes the versioned management API and must never connect to NATS with operator credentials directly.

The first screens will cover cluster health, streams/queues, consumers, backlog, delivery failures, and diagnostic exports. The frontend stack will be selected before implementation and recorded in an ADR.
