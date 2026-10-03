# WebUI Metric History

[English](webui-metric-history.md) | [简体中文](webui-metric-history.zh-CN.md)

Status: WEB-017 is implemented in the current development candidate. It is not part of the older frozen rc.2 qualification and does not by itself approve a release.

The Overview and Queue detail pages read real range data through authenticated `GET /api/v1/history`. The browser can select only the declared metric and window identifiers. It cannot submit PromQL, a Prometheus URL, credentials, arbitrary labels, start/end timestamps or a step. The management service maps the request to a fixed PromQL allowlist and fixed 15-minute, 1-hour and 24-hour ranges with 15, 60 and 300 second steps.

Prometheus is configured only on the management service with `RJS_PROMETHEUS_URL` and, when required, `RJS_PROMETHEUS_TOKEN`. The token is never returned to the browser. HTTPS is required by default; `RJS_PROMETHEUS_ALLOW_INSECURE=true` is restricted to isolated development or internal Compose networking, and the Helm production validation rejects it. Redirects, URL credentials, unexpected origin paths, oversized responses, excessive series/samples, duplicate projected identities, unapproved labels and invalid numeric/timestamp ordering are rejected.

The chart preserves exact source values in an accessible sample table. Prometheus `NaN` samples become visible gaps rather than zero or interpolated values. A decrease in management uptime starts a new segment and is labeled as a process reset. Empty series mean “no samples / unknown”, never zero or healthy. Backend disablement, access denial and unavailable/incompatible responses are distinct and clear earlier evidence.

Local verification on 2026-09-11:

- 375 frontend tests passed; one Windows symbolic-link test was skipped as designed.
- `go test ./...` and `go vet ./...` passed.
- The promoted four-file embedded UI passed its identity check.
- Embedded Chromium passed all 134 live-service checks. The report contains the fixed-query, real-range, gap and window assertion and 32 captured server-side Prometheus requests: `artifacts/webui-live-7YEWqc/report.json`.
- Frozen local management candidate SHA-256: `3b16a67f94d1bc49d56ef2d184238b5131a1520242a59f369c9b6f44614b79c5`.

Docker Compose configuration validation passed. Helm CLI was not available locally, so chart rendering remains to be verified by the Helm/release gate. Prometheus availability, retention and scrape health remain operator responsibilities; this feature is not an alert-delivery service.
