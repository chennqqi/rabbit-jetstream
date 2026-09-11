# WebUI Operational Alerts

[English](webui-operational-alerts.md) | [简体中文](webui-operational-alerts.zh-CN.md)

Status: WEB-026 is implemented in the current development candidate. It does not qualify the older frozen rc.2 runtime or constitute release approval.

Authenticated operators and auditors can open `/admin/alerts`. The management service reads the fixed Prometheus `/api/v1/rules?type=alert` endpoint and projects only the six alert names committed in `deploy/observability/alerts.yml`. The browser cannot submit a rule name, PromQL, upstream URL or credential. Responses show source, threshold expression, configured duration, severity, current firing/pending/inactive state and source observation time.

Inactive means only “not currently firing”. It is never relabeled as recovered on the first observation. The management process retains a bounded state for the six rules and reports recovered only after it observes that same rule transition from firing to inactive. Recovery evidence is process-local and is reset on management restart; it is not durable alert history. Missing repository-defined rules are returned and displayed explicitly, so a partial Prometheus rule set cannot look like complete coverage.

This console does not send notifications. `RJS_PROMETHEUS_PUBLIC_URL` may separately declare a browser-visible HTTPS Prometheus origin; when configured, the UI exposes only its `/alerts` page. It is never inferred from the internal scrape origin. Credentials, query strings, fragments and arbitrary paths are rejected; loopback HTTP is accepted only with the existing insecure development opt-in.

Verification on 2026-09-11: 379 frontend tests passed with one designed Windows symbolic-link skip; full Go tests and vet passed, followed by affected-package tests after the coverage-accounting refinement. Embedded Chromium passed all 135 live-service checks, including fixed rule projection, partial-coverage warning, threshold, duration, firing, observed recovery, explicit external link and the no-notification claim. Evidence: `artifacts/webui-live-r3J5uN/report.json`. The frozen local management candidate SHA-256 is `3a913f922b38d589e367e03ae8de5026aec4e565f46f2d8ab70a77180dea9238`.

Docker Compose configuration validation passed. Helm rendering remains assigned to the release gate because Helm is unavailable on this workstation.
