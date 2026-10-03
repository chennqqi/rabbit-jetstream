# Queue DLQ diagnosis

[English](webui-dlq-diagnostics.md) | [简体中文](webui-dlq-diagnostics.zh-CN.md)

Queue Configuration now includes a read-only DLQ diagnosis section (WEB-022). It uses the already-read declaration Plan as desired configuration, not a live transfer verdict. No DLQ means “not configured” and no additional requests. Unsupported mechanism, invalid names or a self-reference block related reads.

For an `advisory-republish` target, the view reads the exact target declaration and this management process's controller status independently. Only a matching target declaration (Queue identity, Plan revision and target Stream mapping) permits the Stream lookup. It follows no further DLQ dependencies. A refresh therefore issues at most three existing GET requests; no account-wide enumeration, publish, acknowledgment, retry, replay or mutation occurs. Cancellation prevents late results and chained reads from a departed view.

The view distinguishes available, missing, denied, incompatible, unavailable and unobserved evidence. Missing declaration does not imply missing Stream: that Stream remains unobserved because it was not read. Failures clear old evidence; successful sources keep their independent read times. Source declaration, target ETag, Stream existence and controller status are not an atomic snapshot. Refreshing DLQ evidence does not replace the source declaration; use whole-page refresh to reread it. Existence does not prove target ownership, readiness or successful transfer. Exact target links provide independent declaration/Stream inspection.

Controller flags, instance, last run, last successful run and error presence are reported without a synthetic health verdict. Zero Go timestamps mean unreported. Raw error text is not copied into the model. DLQ processed/moved/failed counters are cumulative reports from this controller process across **all Queues**, can reset, and are not a complete event ledger. Missing counters remain unreported, while zero and exact large integers are preserved. Error-free or disabled/follower status must not be interpreted as per-Queue success/failure. Per-Queue transfer history and a complete transfer-health contract require further backend instrumentation and remain unimplemented.

Tests cover one-hop reads, missing/denied/unavailable target outcomes, mismatched mappings, invalid/self-referencing plans, controller failures, exact counters, timestamps, privacy and canceled chained requests. The isolated live suite creates two dedicated empty Queues (`live_dlq_source`, `live_dlq_target`), verifies real target declaration/Stream reads, injects one target 404 to test unobserved Stream presentation, recovers, and asserts no diagnostic PUT. It publishes no messages and does not prove actual DLQ transfer. These resources exist only in the harness-owned broker; never run this fixture against an existing service.

## Backend counter semantics

Controller timestamps require real RFC3339 calendar dates, an explicit timezone and at most nine fractional digits. Valid source strings retain their original precision and offset. Only exact `0001-01-01T00:00:00Z` (also allowing all-zero fractional digits) is unreported; invalid times make controller evidence incompatible and exclude its values from export. Malformed JSON is also incompatible for any DLQ source, while HTTP 401/403 remains access denied. Independent valid sources are not cleared by another source's validation failure.

Backend counter semantics: partial results are retained even when the batch later reports a read or acknowledgment error; the run still reports failure and does not advance its last-success timestamp. These are processing-attempt counters, not unique-message counts: redelivered advisories can be counted again, and `moved` can include an already-absent source message. Failed transfer attempts may be reported in a batch that otherwise completes without a batch-level error. Do not interpret `moved` as proof of that many new destination messages or `lastSuccess` as proof that every transfer succeeded.

## Ignored advisory attempts

`GET /api/v1/controller` now includes `dlqIgnored`, accumulated under the same lock as `dlqProcessed`, `dlqMoved`, `dlqFailed` and run status, including partial failed batches. It reports advisory attempts whose JSON could not be decoded or whose Stream/Consumer did not match a declared source in that batch. It does not establish message loss, a completed acknowledgment or a business-rule rejection. This is additive: older servers may omit the field, which the UI reports as unreported and the local export leaves absent. Zero is explicitly reported by the current backend; exact large integers remain lossless in the frontend/export.

Example counter subset of a controller response (not per-Queue statistics):

```json
{"instanceId":"management-1","dlqProcessed":10,"dlqMoved":4,"dlqFailed":2,"dlqIgnored":4}
```

Prometheus now also exposes `rjs_dlq_processed_total` and `rjs_dlq_ignored_total`, labeled only by controller instance. Existing moved/failed metric names remain unchanged; moved help text now explicitly includes already-absent source messages and disclaims unique transfers. Metrics retain the existing floating-point exposition path, not the JSON/export exact-integer guarantee. No new event storage, reason ledger, Queue labels, message processing, retry or acknowledgment behavior is introduced.

## Local evidence download

Readiness is independent per source: a missing target can leave the Stream unobserved while the controller is still loading. The refresh button remains disabled until no source is loading; a transport timeout settles the affected source as unavailable, allowing another explicit read. Tests cover a stalled controller, deadline recovery and late-result fencing. Browser reports now include allowlisted DLQ request timings and, on export-readiness failure, source statuses and a section screenshot. A previously observed Chromium five-second readiness failure remains unresolved despite a passing rerun; see the [execution record](webui-development.md).

“Download DLQ evidence JSON” saves `rjs.dlq-diagnostic-evidence.v1`: original source Queue/Plan revision/declaration ETag/read time and target mapping, followed by each already-read source's state and timestamp. Only available observations include allowlisted values. Nonavailable observations never export leftover values or raw errors. Controller counters retain exact integer values and their process-wide scope. The file does not contain credentials, full declarations, arbitrary raw backend responses or per-Queue message history.

Download creates a local browser Blob and issues no API request, reread or mutation. Changed observations replace the prepared download; old Blob URLs are revoked. A preparation failure is visible and does not clear page evidence. Downloads contain operational/configuration metadata and remain on disk after session clearing. The file explicitly disclaims atomicity, completeness, ownership/readiness guarantees and transfer outcome attribution.
