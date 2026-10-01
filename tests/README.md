# Test Suites

[English](README.md) | [简体中文](README.zh-CN.md)

- `integration/`: black-box tests against real single-node and three-node JetStream deployments.
- `fault/`: node loss, network interruption, process crash, disk pressure, and recovery scenarios.
- `compatibility/`: RabbitMQ-style semantic contracts shared with client SDK repositories.
- `performance/`: reproducible throughput, latency, resource, and soak benchmarks.

Unit tests remain next to Go packages. Fixtures must be deterministic and contain no credentials.

## Local connection API qualification

The suite also reads exact detail for all 201 clients, checks missing-node versus missing-CID errors, rejects invalid CID/query inputs, verifies anonymous GET/HEAD denial, and confirms a closed CID becomes missing while a surviving CID stays readable. It then creates exactly 1,000 subscriptions on one owned client and performs 50 complete detail reads while continuously sampling same-client PING latency and NATS RSS. Finally it grows the owned node to exactly 1,000 account-matching connections, verifies identity-search first/last pages and filtered total, then proves the 1,001st match returns 422 without a partial result. Set `RJS_TEST_MANAGEMENT_BINARY` to the selected locally built binary; both selected binaries are fingerprinted before and after execution.

Build locally with `go build -o bin/rjs-management-connections.exe ./management/cmd/rjs-management`, then run `node tests/integration/connections-live.mjs` from the repository root. Requires the locally built `bin/nats-server-candidate.exe` and Node.js. On non-Windows hosts omit `.exe` from both binary filenames. Do not replace either binary while the run is active.

The script starts dedicated loopback services and only test-owned clients/subscriptions. It publishes no messages, uses no containers or remote hosts, and stops only owned processes/connections. Random API credentials are memory-only; reports under `artifacts/connections-live-*` include latency percentiles, RSS samples, host metadata, thresholds and before/after binary fingerprints but no credentials. Temporary broker data is retained in that evidence directory. This qualifies the bounded local connection/identity/subscription paths on the recorded host; it is not long-duration, multi-node, WebUI or formal production-host qualification.
