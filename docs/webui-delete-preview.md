# Queue deletion preflight

[English](webui-delete-preview.md) | [简体中文](webui-delete-preview.zh-CN.md)

Status: preflight and a single-attempt deletion UI are implemented in source and the local candidate, not released. Durable outcome resolution and full deletion UX qualification remain pending.

`GET /api/v1/queues/{queue}/delete-preview` requires an operator bearer and the original quoted declaration `If-Match`. It rejects query parameters, including `force`, and create-only conditions. No lock, resource/declaration mutation or audit intent/outcome is written. Reads have a five-second timeout.

The response identifies the Queue and Stream, retains the original revision in `base_revision` and `ETag`, and reports management-side read completion time (`observed_at`). Declaration identity/version and the original KV revision are checked; revision is rechecked before returning. Responses use `Cache-Control: no-store`.

| Observation | Response meaning |
| --- | --- |
| Owned, empty Stream | `blocked=false`, `requires_force=false`; this does not authorize deletion |
| Owned Stream containing messages | `blocked=true`, `requires_force=true`; default non-force deletion would be blocked |
| Unmarked or differently owned Stream | `blocked=true`; force cannot bypass ownership protection |
| Missing Stream with existing declaration | `stream_present=false`, ownership `unobserved`, message/Consumer counts omitted; declaration-only cleanup may be possible |
| Existing Consumers | Count includes all observed Stream Consumers; presence is not a deletion blocker in the current backend |

Successful observations, including blocked ones, return 200. Invalid name/query returns 400, missing/invalid bearer 401, insufficient role 403, missing declaration or disabled authenticated API 404, changed revision or inconsistent declaration identity 409, missing/invalid edit condition 428, and backend failure 503. General resource-read authorization may reject the request before the operator-specific guard.

Message counts are unsigned 64-bit values and must be decoded losslessly. Missing counts are unknown, not zero. `blocked` describes only the default `force=false` protection at observation time. `requires_force` means observed messages are nonzero, not that force makes deletion safe or permitted. An ETag versions the declaration, not Stream contents or Consumer activity. Separate reads do not form an atomic snapshot. Preflight does not inspect DLQ dependents or reserve a deletion.

The UI preserves these distinctions, shows current impact and ownership, defaults force off and requires exact-name confirmation plus explicit destructive-impact acknowledgment. Refresh clears both confirmations and force; changing force clears confirmation. Final DELETE still needs its own conditional checks. Transport failure or audit failure may leave an unknown outcome; do not blindly retry.

## Candidate deletion workflow

Operators can open **Review deletion impact** on Queue detail. `/admin/queues/by-name/{queue}/delete` is a dedicated session-owned workflow. It first reads the declaration, retains its original ETag, then validates the entire matching preflight response. Invalid/inconsistent evidence cannot enable submission. Counts are displayed losslessly; Consumer presence is not labelled as a backend delete blocker.

A confirmed attempt sends one DELETE with original `If-Match`, exact `X-RJS-Confirm-Queue`, explicit `force=false/true` and a generated `X-Request-ID`. Duplicate submissions are blocked. A valid success is shown as server acknowledgment, not a fresh absence observation or protection from recreation. Any request error or malformed success stays **unknown**, including 409/401/503: a browser replay may have observed a different result from the first attempt. Read-only declaration, Stream and request-audit inspection never unlocks retry or attributes missing resources to this request. Audit inspection supports explicit older-window reads using exact cursors, including empty matching windows. Durable outcome resolution remains pending.

Review and request evidence survive SPA navigation in memory. Credential expiry retains evidence but blocks new requests. Session clearing while submitting is blocked; clearing unresolved evidence requires confirmation and never cancels/rolls back server work. Same-Queue edit/create and deletion are fenced within the session. A retained editor can be explicitly archived through the per-Queue handoff described below, without clearing the session. Unsubmitted deletion review can be canceled without clearing the session. Submitted deletion evidence is retained until explicitly clearing the session. Selectable JSON and dedicated deletion evidence downloads are available.

### Per-Queue editor handoff

The deletion page offers an explicit confirmed action to end that Queue's edit/create review and archive its evidence. All same-Queue editor entries must first be in a releasable state: idle/load-error/uneditable, editable review/error/conflict, or acknowledged accepted. Loading, previewing, submitting, inspection, conflict-read, next-edit-read and unknown-outcome states block the entire handoff. The control is disabled while blocked and the model rechecks before acting.

The synchronous handoff snapshots original/raw/merge drafts, revisions and accepted request evidence before putting the old models into a read-only archived state. Archived models cannot resume editing or submit. Active/parked/completed creation references to those models are detached; unrelated drafts and the credential remain unchanged. No HTTP request, apply, delete, logout or backend cleanup occurs. Cancelling confirmation changes nothing. Deletion remains a separate explicit preflight reading the current declaration and version; archived ETags are never passed into a deletion attempt.

Archived editor records are accessible and downloadable on that Queue's detail/edit/delete routes and from the expired-session view. They are session-memory evidence, not resumable drafts or outcome certificates. Reloading or explicitly clearing the session loses them, so the session-clear prompt includes retained evidence. Download files remain on disk. Re-entering the editor after cancelling an unsubmitted deletion starts a fresh editor; the archived record remains read-only. Unknown deletion recovery still does not permit a new write or handoff.

### Evidence paging and downloads

Each older-window request validates request identity, cursor progress and sequence bounds. Failure retains the previous windows and cursor for explicit read retry; it never permits DELETE retry. A null cursor means this scan has no older retained range, not that historical retention is complete. Refreshing outcome observations archives the previous inspection (including all loaded windows and read failures) in session memory. Separate read times remain visible; no atomic snapshot or causal attribution is implied.

`rjs-queue-delete-evidence.json` uses schema `rjs.queue-delete-evidence.v1`. It contains the original ETag, force selection, preflight, correlation IDs, result/error classification, current inspection and prior inspection history. Only explicit fields are exported; session credentials, raw error text and transport headers are excluded. Observed resource/audit bodies remain sensitive and are intentionally retained. Downloads do not send requests or alter write locks and remain available from retained evidence after credential expiry. Blob URLs are replaced/revoked with snapshot changes and unmount. Clearing the session does not delete downloaded files. Evidence is not an outcome certificate, full history or replay input.

Local browser qualification creates and deletes only `live_delete_fixture` and `live_delete_unknown` inside the harness-owned broker. The latter deliberately returns unreadable JSON after a real successful deletion to verify the unknown-outcome lock. Never point this harness at an existing service. No remote qualification or release promotion is implied.

Verification: backend read-only test doubles panic on write/lock/ensure paths; cases cover counts, ownership, missing resources/declaration, invalid identities/conditions, stale and mid-read revisions, cancellation and backend failure. HTTP tests cover authorization, timeout, no-store/ETag and errors. The local candidate harness records `delete-preview.json`, checks real ownership/counts/revision, auditor denial, stale conflict and unchanged declaration without issuing DELETE.
