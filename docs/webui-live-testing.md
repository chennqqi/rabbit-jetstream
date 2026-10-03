# Candidate WebUI real-service smoke

[English](webui-live-testing.md) | [简体中文](webui-live-testing.zh-CN.md)

## Browser selection

Routing-probe request evidence records only method, start time, HTTP status and header/finish/failure durations. Query values, authorization and response bodies are excluded. This distinguishes an unavailable route or transport stall from a UI assertion timeout without adding sensitive routing data to evidence.

The Queue template helper reviews DLQ and priority JSON locally, switches templates without leaking inactive fields, preserves an integer above JavaScript's safe range, captures Chinese mobile layout and checks page overflow. It then prepares and previews the priority draft, confirms a single create-only write, reads the actual saved declaration and verifies that no DLQ target was created. “Create another Queue” must clear template selection. This check uses only owned isolated fixtures; a started run is not a passing result. See [template scope](webui-queue-templates.md).

The Consumer replica helper now tabs from the raw-observation summary into the named scroll region, uses ArrowRight to reach its right edge, verifies the last follower metric's exact text and horizontal visibility, captures `consumer-replica-keyboard-right-columns-zh.png`, then uses Shift+Tab to return to the summary. This tests the exercised mobile keyboard path, not full accessibility or real cluster health. Pending runs do not count as qualification.

Consumer replica diagnosis coverage injects synthetic named Leader/Follower observations, checks offline/not-current and exact large lag hints, then injects duplicate identities and requires no partial topology or replica hints. Stale diagnosis must clear both counter and replica evidence. The mobile screenshot contains synthetic cluster data, not evidence of real cluster failure, quorum or node health.

Consumer diagnosis checks require a new local backend build: `go build -o bin/rjs-management-consumer-diagnosis.exe ./management/cmd/rjs-management`, then `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-consumer-diagnosis.exe` (omit `.exe` off Windows). This supersedes the older executable selections below. The helper verifies the owned broker's default positive ACK limit, then injects explicitly synthetic exact counters to exercise threshold/no-waiting-pull/redelivery hints. It checks bilingual mobile rendering, 503 clearing of current diagnosis while retaining the original observation time, malformed limit handling and real-read recovery, with GET-only requests. `consumer-diagnosis-synthetic-mobile-zh.png` is synthetic-counter evidence, not real backlog saturation, message delivery or throughput qualification.

Candidate input evidence covers all regular files under the candidate build, not only HTML-referenced entry chunks. Both the complete file set and content fingerprints are rechecked after the run; additions, removals or edits fail verification, and symbolic links are rejected. Do not rebuild while testing. This before/after check does not detect a file changed and restored between observations, and is not an atomic filesystem snapshot.

For the current cycle error contract, build locally with `go build -o bin/rjs-management-dlq-errors.exe ./management/cmd/rjs-management` and set `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-dlq-errors.exe` (omit `.exe` off Windows). This supersedes the earlier `dlq-chain` executable selection below; do not overwrite inputs during an active test run.

Unknown-write batch coverage forwards exactly one real create, then supplies malformed JSON instead of its successful response. It confirms the root exists while its dependent remains absent, the UI keeps the original request ID and blocks apply/dependent preparation/archival, and read-only inspection plus in-app navigation do not unlock or repeat the write. This tests uncertainty containment, not authoritative outcome resolution; inspection remains non-attributing.

Archival coverage declines and then accepts the confirmation, verifies zero archival Queue API calls, parses the read-only history with exact integers/request IDs, navigates away/back and starts another batch. Reusing an earlier retained name must fail without a PUT. Archiving a fresh unsubmitted item must leave it uncreated and preserve the earlier archive byte-for-byte. A Chinese mobile history screenshot is saved. This does not qualify expired-session display or unknown-write recovery.

Single-file import coverage uses actual file selection with a malformed file followed by a valid Queue document. It checks that original form inputs survive rejection and declined replacement, requires explicit review, verifies no Queue API I/O before preview, and leaves unknown spec fields intact for server rejection. After correction, preview must still leave the resource missing; only separate confirmation may issue one create-only PUT. Exact int64/priority/labels and retained-name refusal are checked. Chinese mobile screenshots record the import panel. This does not qualify multi-file dependency migration.

The routing binding helper also verifies native keyboard access: Tab from the probe button into the named results region, ArrowRight to the final column, and Shift+Tab back out. `routing-bindings-keyboard-zh.png` records the final-column view. Fault cases inject unknown/malformed 404/409 responses and malformed 400/403 bodies, require no false missing/changed conclusion, clear evidence and recover through real reads. The canceled-request gate uses a bounded assertion and always releases in `finally`.

The **current full suite** requires routing, declaration-export, batch-import-planning APIs including `review_order`, current DLQ target Stream checks and bounded transitive cycle validation. Build locally with `go build -o bin/rjs-management-dlq-chain.exe ./management/cmd/rjs-management` from the repository root, then set `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-dlq-chain.exe` (omit `.exe` off Windows). This supersedes older external-review/batch-execution/planning/export/routing/detail binary selections. Never replace candidate inputs while a browser run is active.

The transitive-cycle fixture creates three Queues only on the harness-owned broker, then proposes a closing DLQ edge. Both preview and conditional PUT must reject the cycle with HTTP 400 `dlq_dependency_cycle`, preserving the original declaration and ETag. Rebuild the backend for this error-contract revision; earlier 120-check reports exercised the previous 503 response. This is API regression within the browser harness, not a browser interaction or concurrent-update guarantee. Unknown-write inspection separately requires exact `available` status for declaration, Consumers and audit; `unavailable` must not satisfy the assertion.

External-dependency coverage plans a two-item chain rooted in a missing external Queue. Internal order must remain empty while preview order puts prerequisites first. The missing target must reject preview with no PUT. Explicit test setup then creates only that target in the harness-owned broker; a fresh preview and separate confirmations are required before two dependent create-only PUTs. Exact saved integers and transitive prerequisite gating are checked. This is not a message-delivery test or exhaustive target drift/unknown-outcome qualification.

Per-item execution coverage starts a separately acknowledged batch, blocks its dependent until the prerequisite is accepted, previews/applies both through the actual editor and checks exactly two create-only PUTs in dependency order. Preview alone leaves each Queue missing. Switching away invalidates the dependent preview, and in-app navigation plus returning from the creation page preserve the batch. Saved int64 values and a Chinese mobile outcome screenshot are checked. This does not cover unknown-write recovery, external dependency execution or full accessibility.

Batch planning coverage uses actual multi-file selection, unreadable-file blocking, exact integer transport, source-before-target input reordered dependency-first, and API reads proving planning created neither Queue. It checks duplicate/invalid declarations and present/missing external references remain blocked, malformed-response evidence clearing, explicit retry and clearing files. The Chinese mobile screenshot includes a long local filename. This is planning coverage, not batch execution qualification.

Declaration export coverage reads real browser-downloaded JSON with the lossless decoder, verifies exact int64/priority-zero and default/explicit label policy, and observes Blob URL revocation after policy change and panel exit. It checks real declaration changes, malformed responses, cancel/retry and Chinese mobile rendering. The helper intercepts only URL revocation for observation; it still calls the native method. Files contain synthetic fixture values, never real credentials. Download tests do not qualify import or complete dependency migration.

Routing coverage creates a separate Queue in the harness-owned broker with direct/topic/fanout bindings. It checks selected versus unselected binding rows, trailing `#` zero-token matching, Chinese mobile presentation, a real saved-declaration revision change, refresh recovery and canceled-read retry. Fixture writes are test setup only: the production probe remains read-only. Mobile checks require both no page overflow and a horizontally scrolling binding table with readable minimum widths; a no-overflow assertion alone previously missed vertically squeezed text. Inspect `routing-bindings-mobile-zh.png` and the exact input fingerprints in each report. Scoped contrast scans are not full accessibility qualification.

Exact connection-detail checks require `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-connection-detail.exe` (omit `.exe` off Windows), built from the detail-capable backend. The suite enters a real CID from a 25-row list context, waits for an actual periodic detail read, tests unavailable history versus node/CID missing, denied, ambiguous and malformed-response clearing, and recovers through real reads. It checks bilingual rendering, desktop/mobile accessibility checkpoints, 375px overflow, return-page size and browser back/forward navigation. The mobile screenshot is saved before the overflow assertion so failures retain visual evidence. These checks do not qualify client search, subscription detail or cluster faults.

DLQ regression now injects invalid `lastRun`/`lastSuccess` calendar dates and malformed controller JSON, then checks incompatible presentation, exclusion from exported values, preservation of independent target/Stream observations and real-read recovery. Two gated controller responses verify that refresh remains disabled while that source is pending; a third is held until the production ten-second transport deadline settles it as unavailable, after which export and a fresh read must work. Only this deliberate deadline assertion allows 15 seconds. Normal export readiness keeps its five-second assertion. These synthetic faults do not establish the cause of an earlier spontaneous readiness failure.

Connection timestamp regression injects an impossible calendar date independently into `observed_at` and `read_at`, requiring the page and times to clear before a real read recovers. With accessibility enabled, all six numeric cells in the first connection row are individually scrolled fully into view and must receive a positive `color-contrast` pass with no violations or incomplete findings. `accessibility-connections-mobile-visible-column-*.json` and matching screenshots retain this scoped evidence; the original clipped scan remains intact.

For a repository-local browser cache, set `PLAYWRIGHT_BROWSERS_PATH` to the absolute `artifacts/playwright-browsers` directory before both installation and test execution. From `admin-ui`, run `node node_modules/playwright/cli.js install chromium firefox`. These downloaded test tools are not release assets.

Connection-page integration requires a management executable containing the node connection API. Set `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-connections.exe` (relative to repository root) to use the separately built executable without overwriting the older default candidate. On non-Windows hosts omit `.exe`. The selected executable is fingerprinted before/after the run just like the default. Do not replace it or frontend assets during a run. Added connection checks cover node entry, real totals and periodic reads, injected failure retention/clearing, page-size history, bilingual rendering and desktop/mobile accessibility checkpoints. Passing evidence must come from the report for those exact inputs.

Node refresh coverage verifies the shared manual preference, an actual periodic detail read, a synthetic 503 retaining the old snapshot/time with visible stale labels, and a synthetic missing Node ID clearing the selected detail before recovery. The separate real broker-loss test still checks unavailable endpoints without stale identity links. These are monitoring/read tests, not node-management actions or native host qualification.

Queue/Stream list refresh coverage waits for a real periodic read, checks unchanged URL and unsubmitted search input, and exercises failed-read retention/recovery. The shared Settings preference is then switched to manual: both lists must read on entry, issue no periodic requests over 11 seconds, and issue exactly one GET on explicit refresh. These checks use the same isolated services as the rest of the suite, not external accounts or existing listeners.

Mobile contrast follow-up preserves the original clipped-content scan, scrolls each incomplete contrast target into view without changing styles, and runs `color-contrast` on that exact element. Each follow-up must have a positive pass with a nonzero tested-node count, no violations and no incomplete results; otherwise the run fails. `accessibility-mobile-contrast-visible-*.json` and matching screenshots record the follow-up. This resolves only those specific targets in that rendered state, not other manual accessibility work. Scroll positions are restored before normal navigation tests continue.

Set `RJS_TEST_A11Y=1` to run pinned axe-core 4.10.3 checks at login, desktop Compatibility, desktop/mobile Queue summary and the unknown-write state. The tags are `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`, `wcag22aa` and `best-practice`, with no disabled rules. Violations fail the run after collecting the checkpoints. `accessibility-*.json` and the report retain both violations and incomplete findings, using rule/check identifiers and selectors rather than raw HTML, text or input values. Incomplete findings require manual review and are not counted as automatic passes. The engine is a development dependency injected by the local harness, not a shipped browser asset. The normal 72 functional checks remain in place. This covers only these rendered states, not all dialogs, languages, contrast/zoom cases or screen-reader workflows.

The same isolated live suite supports Chromium (default) and Firefox, without reducing the assertions for either engine. Reports record `browserEngine` and the actual `browserVersion` alongside candidate artifact fingerprints. Install the matching test browser locally if absent, then select it explicitly:

```powershell
cd admin-ui
node node_modules/@playwright/test/cli.js install firefox
$env:RJS_TEST_BROWSER='firefox'
npm.cmd run test:live
$env:RJS_TEST_BROWSER='chromium'
npm.cmd run test:live
```

Unsupported browser names fail before services start. Browser installation affects only the local Playwright cache; this workflow does not change the default browser or touch a remote qualification host. Runs still create isolated loopback services and delete only their own empty Queue fixtures. Do not rebuild candidate artifacts while a run is active. This engine matrix does not replace full accessibility auditing, release freezing or native host qualification. Consult the current development record for actual completed runs; engine support alone is not passing evidence.

Schema regression verifies revision/ETag and exact int64 text from the real authenticated endpoint and saves `queue-schema.json`. An incompatible Settings schema clears old metadata and recovers on explicit refresh; a pre-apply schema 503 produces zero PUT, retains the draft and requires fresh preview/confirmation. Independent offline checks run `python tests/admin-ui/queue-schema-check.py` (jsonschema 4.x); see [schema boundaries](webui-queue-schema.md).

Shared-observation regression retains an editor/deletion review, navigates to Settings with a synthetic changed capability response, then returns using browser history. It asserts invalidated confirmation, preserved editor draft and zero corresponding writes before fresh explicit preview/confirmation. Unit tests additionally cover unavailable reads, multiple retained models, old credentials, observer exceptions and unchanged pending/unknown/archived states. Evidence: `artifacts/webui-live-gWvCzY/report.json`; this does not simulate a real rolling upgrade.

Receiving-instance precondition checks send a deliberately different valid capabilities token to both preview routes and PUT/DELETE. All must return 412 and retain the Queue declaration ETag. Normal browser flows send the original opaque token from authenticated capabilities; synthetic body-change responses retain that token to isolate frontend fingerprint invalidation. This tests contract mismatch rejection, not a multi-version deployment or binary attestation.

Mutation capability checks use real capabilities for normal preview/write workflows. A valid but changed deployment profile is injected for one pre-dispatch read before apply and before each fixture deletion. The harness asserts zero corresponding write requests, retained draft or resource, cleared review/confirmation and successful explicit re-preview. The injected response is synthetic contract-change coverage, not a real server upgrade or atomicity test. `capability-change-before-apply.png` captures the no-write state.

Settings qualification reads the real authenticated capabilities endpoint as auditor and rejects anonymous access. The default isolated process has unknown/unspecified deployment intent, parser-supported values, canonical defaults and unreported qualification. One incompatible response is injected to prove stale capability values disappear, followed by a real explicit refresh. `console-capabilities.json` and `access-settings-mobile.png` preserve evidence. Explicit standalone/cluster configuration and invalid-startup cases have Go coverage; the browser does not infer deployment intent from the harness node count.

The deletion fixtures also exercise editor handoff: one archives an acknowledged edit, the other an invalid unsubmitted draft. The browser dismisses then accepts the handoff confirmation, verifies unchanged declaration ETag during archival, downloads `{fixture}-archived-editor.json`, and checks another Queue draft remains unchanged. An earlier unknown editor must expose disabled handoff/preflight controls. Handoff itself issues no writes; subsequent deletion retains all existing confirmation/preflight requirements.

Deletion evidence qualification adds 130 audited no-op applies on the harness-owned `live_candidate` after each dedicated deletion. The declaration content is unchanged; audit records move the deletion beyond the first 256-record scan. The UI must traverse the empty matching window, find older request records, retain previous windows on observation refresh and download them without credentials or unlocking retry. `{fixture}-evidence.json` records the native browser download. This padding is part of the standard smoke and must never target a non-harness service.

The standard smoke now creates and deletes two empty, dedicated Queue fixtures (`live_delete_fixture`, `live_delete_unknown`) only inside its own isolated loopback broker. It verifies typed confirmation, force/refresh invalidation, original-ETag DELETE, single dispatch and outcome evidence. For the second fixture it forwards the real deletion but substitutes an unreadable response; the UI must remain unknown and must not retry after readback. Review/outcome PNGs and the report retain evidence. Never target an existing service with this destructive fixture test.

The metadata option also injects a foreign Queue ownership marker on all four fixed fixture resources after drift repair. It verifies blocked preview, apply HTTP 409, unchanged declaration ETag and retained foreign markers. `metadata-ownership-conflict.json` and `metadata-verify-conflict.log` retain the evidence. These test-only mutations still target only the harness-owned broker; do not invoke the helper against an existing service.

Optional real metadata-drift qualification (run from the repository root in a dedicated PowerShell process):

```powershell
go build -o bin/metadata-drift-candidate.exe ./tests/helpers/metadata-drift
$env:RJS_TEST_METADATA_DRIFT='1'
Push-Location admin-ui
npm.cmd run test:live
Pop-Location
```

Build the normal current NATS/management/UI candidates as described below first. This option creates `live_metadata_check` only inside the harness-owned broker, injects stale managed labels, removes an empty-valued managed label, and adds external metadata on its Stream and three Consumers. It asserts empty declaration diff but four proposed resource repairs, unchanged ETag after preview, conditional apply, preserved external metadata and subsequent noop. The test-only helper requires an explicit loopback URL, fixed fixture identity and three owned Consumers; do not run it manually against an existing service. Helper SHA-256 is included in report inputs; `metadata-drift-preview.json` and helper logs retain evidence. No helper or fixture is included in release assets. This does not qualify external-writer concurrency or native Linux execution.

The candidate smoke runs real locally built NATS and management binaries, serves `admin-ui/build-candidate` with a same-origin API proxy and production-equivalent CSP, and drives a local headless browser. With `RJS_TEST_EMBEDDED_UI=1`, the proxy instead obtains `/admin/` and its assets from the explicitly selected management binary while retaining the documented API response-fault injection. Optional `RJS_TEST_SESSION_EXPIRY=1` replaces only expiry metadata in one authenticated session response and advances a controlled browser clock; this is synthetic lifecycle coverage, not real OIDC expiry qualification. This is an integration gate for implemented flows, not full WebUI, release, performance or native Linux qualification.

To include expiry evidence-retention coverage, run `$env:RJS_TEST_SESSION_EXPIRY='1'; npm.cmd run test:live` from `admin-ui` after building. Use a dedicated PowerShell process or clear this test variable afterward to return to default coverage.

## Local Windows commands

Each live report records SHA-256, byte length and repository-relative path for the NATS executable, management executable, candidate HTML and referenced JS/CSS assets in `inputs`. A successful run rechecks them before setting `passed` and records `inputsVerifiedAt`; changed inputs fail the run. These hashes identify tested files, not source provenance, reproducible-build proof or release signatures. Do not rebuild those paths during a run.

### Separate selected-data visual capture

After `npm.cmd run build`, run `npm.cmd run test:selected` from `admin-ui` to capture the current candidate components using the selected `orders_events` fixture. This starts only a temporary loopback static/fixture server, never NATS or a management process, and has no upstream proxy. Only four exact GET paths exist; writes and undefined routes are rejected, and browser requests outside that origin are blocked. It is not a substitute for `test:live`.

The browser uses Chinese, Asia/Shanghai, fixed instant `2026-09-09T08:20:00.000Z`, desktop 1487 × 1058 at density 1, and mobile 375 × 812. It asserts stored/pending/ack-pending 12480/8420/240, ETag 12, three replica rows and the offline Follower, then tests read-only refresh. Plan content revision remains distinct from KV ETag. Leader metrics stay unreported rather than inheriting fictional zero/current from the generated image. The candidate notice is the only DOM text replaced, solely to label screenshots as synthetic; no values or layout are changed for capture.

Artifacts under `artifacts/webui-selected-*` include viewport/full-page screenshots, header/evidence-region screenshots, source-image hash, JS/CSS build hashes and a request/error report. `passed` means fixture assertions/capture passed; `visualQAPassed: false` deliberately does not claim fidelity, accessibility, production health or release qualification. No fixture code is imported by `admin-ui/src` or embedded production assets. Owned browser/server resources close on completion or error. Use these captures for the next same-data design comparison; do not compare the previous live R1/English fixture as if it were the selected source state.

The default smoke also injects 503 and 404 responses for the exact primary-Consumer read from Summary. It checks that only that source becomes unknown, restores the real API, and follows exact detail/collection/Back links. These are browser response fixtures, not real Consumer deletion or a real backend outage.

Optional `RJS_TEST_CLUSTER=1` runs three locally built NATS processes on distinct loopback client/monitor/route ports with separate data directories. The harness waits for consistent metadata leadership and two current peers, creates an R3 Queue, stops an observed follower, verifies its offline row, restarts only that owned process using its retained test data, and waits for catch-up. It then runs the existing workflows and stops all owned brokers for loss-of-backend checks. Run `$env:RJS_TEST_CLUSTER='1'; npm.cmd run test:live` from `admin-ui` in a dedicated PowerShell process. This is local integration coverage, not native Linux release or network-partition qualification.

Abrupt follower termination can take several minutes to appear as `offline`: the pinned server's orphan threshold is 150 seconds with a 90-second sweep. The harness allows 270 seconds and records termination/observation times; it does not substitute process exit for the server's reported state. A reported `offline: false` is not proof that a process is still reachable.

From the repository root, build the current sources:

```powershell
go build -o bin/rjs-management-candidate.exe ./management/cmd/rjs-management
Push-Location upstream/nats-server
go build -o ../../bin/nats-server-candidate.exe .
Pop-Location
Push-Location admin-ui
npm.cmd ci --ignore-scripts
node node_modules/playwright/cli.js install chromium
npm.cmd run build
npm.cmd test
npm.cmd run test:live
Pop-Location
```

Check each command succeeds before proceeding. The browser installation is only needed if the matching local Chromium is absent. On Linux, use the same binary basenames without `.exe` and `npm`; build locally, not on release qualification hosts.

The script is [candidate-live.mjs](../tests/admin-ui/candidate-live.mjs). It uses freshly selected loopback ports and a unique `artifacts/webui-live-*` directory. It never attaches to existing services or containers. The brief port-reservation release creates a possible bind race: startup failure must fail the run, not stop the occupying process. NATS receives its own data directory. Inherited `RJS_`, `OTEL_` and `NATS_` variables are removed from child environments; fresh random test operator/auditor tokens are used. The controller is disabled; metadata and the initial Queue use one replica by default, three in cluster mode. Only owned child handles are terminated, including a deliberate broker-loss check; existing local/remote acceptance processes remain untouched. Test data and redacted logs are retained for inspection, not automatically deleted.

Checks include anonymous-read rejection; real read-only preview with no declaration creation; API setup of a priority Queue and its three Consumers; auditor login/list/collection/exact-detail/back navigation; no browser credential storage; auditor preview rejection; desktop/mobile rendering; and a real broker stop causing unavailable state with old collection rows cleared. Setup writes occur through the operator API in this isolated broker; this does **not** prove the candidate's create/edit UI is complete.

Evidence is `report.json`, `management.log`, `nats.log`, and desktop/mobile PNGs in the reported directory. The 2026-09-10 local run `artifacts/webui-live-BStzaL/report.json` passed all seven checks; 49 Node tests and candidate build also passed. Screenshot inspection caught compressed mobile columns that an earlier no-overflow assertion missed. Consumer tables now retain a readable minimum width inside a focusable scroll region, with raw Plan collapsed by default. Full selected-design fidelity, Firefox, accessibility audit, richer fault/mutation journeys and release embedded-build integration remain outstanding.

Follow-up `artifacts/webui-live-ttHLni/report.json` passed the existing checks plus operator draft/preview, unchanged persisted declaration, route-retained drafts and explicit session-clear confirmation. There are now 53 passing Node tests. Candidate editor apply/delete remains disabled; the preview gate is not evidence of completed mutation UI.

Later iterations enabled confirmed candidate apply and uncertain-result inspection; see the current [development ledger](webui-development.md), which supersedes the historical preview-only status. The default response fault is committed-write/truncated JSON. Set `RJS_TEST_RESPONSE_FAULT=socket-reset` for pre-header connection-reset testing. Reports include `responseFault` and each fault-stage PUT's request ID/status; browser transport replay may produce 200 followed by 409 with the same ID. The editor must remain unknown/locked rather than treat that 409 as proof of no write. Inspection must generate no additional PUT. Application-level no-retry does not guarantee transport-level no-replay.
