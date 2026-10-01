# WebUI mutation recovery design

[English](webui-mutation-design.md) | [简体中文](webui-mutation-design.zh-CN.md)

Date: 2026-09-09. Scope: executable **isolated-prototype** follow-up for L-03/L-06/L-09/L-10 in the [logic review](webui-logic-review.md). No production integration or qualification is claimed.

## State and action contract

| Result/state | What may change | Allowed recovery |
| --- | --- | --- |
| Editing/review | Draft only; immutable base configuration and revision retained | Validate, semantic diff, explicit submit |
| Submitting | One captured request; duplicate dispatch guarded synchronously | Await outcome; closing UI is not a cancellation guarantee |
| Accepted | Desired declaration advances; previous observation retained | Independently observe convergence |
| Conflict | Current declaration differs from original base; this write rejected | Compare base/local/current; explicitly retain local changes over current values, edit and review again |
| Forbidden/expired | No change in these fixtures; draft retained in memory | Explicit simulated access recovery, edit/review again; never request real credentials |
| Unknown | A write may have happened; no success or automatic retry | Inspect resources and audit using the same correlation ID; closing retains an unresolved-operation entry |
| Partial | Some resource operations completed; no assumed rollback | Inspect actual fields; then explicitly review manual repair |
| Resource verified, audit missing | Resource result known; audit remains incomplete | Report both independently; do not equate resource confirmation with audit success |

Canonical base/local/current values and mock KV revisions are separate. Rebase copies only local semantic changes onto the current configuration, and only after an explicit user action. A later submission can conflict again in production; the prototype's one-shot failure fixture resets to success for demonstrating recovery. It does not bypass a real ETag check.

Unknown/partial results block normal refresh and new submissions until inspected. Their fixture identity cannot be switched while unresolved. Dirty editor history navigation retains the editor context; unloading warns about losing memory-only state. Browser reload can still discard all synthetic session data if the user confirms; this is not durable recovery storage.

## Concrete fixtures and limits

- Conflict: another mock writer changes ACK wait to 90s (120s if base already equals 90s), KV revision advances by one; local 45s remains uncommitted until explicit recovery and resubmission.
- Unknown response: the fixture's resource change did succeed; inspection reveals the accepted declaration and observation. This demonstrates one possible uncertain outcome, not every timeout outcome.
- Partial: Stream max age reflects the request; Consumer settings remain at their previous observation; desired declaration and KV revision stay at the base. There is no automatic rollback or retry.
- Missing audit result: resources are confirmed after inspection, but the audit warning remains in the session history.
- Correlation IDs such as `mock-op-1` are local fixture IDs, not production identifiers. Session `queue.apply` and `resource.inspect` entries describe prototype actions, not persisted server audit records.

These deterministic fixtures do not implement authentication, actual ETag concurrency, metadata/content-revision reconciliation, durable audit queries, general broker partial-failure recovery or deletion. Production outcomes must come from server/resource evidence, never from a frontend scenario selector. Real read errors during inspection and uncertainty that cannot be resolved remain required integration cases.

## Verification and review evidence

The first browser run found a recovery-button default-action bug: rerendering the button as a form submit unexpectedly entered review. Cancelling the recovery click's default action fixed it. Regression now asserts that conflict/access/partial recovery returns to editable fields before a separate review action.

Latest complete verification: **26 browser checks, 15 model tests, 4 packaging tests; zero automated accessibility violations in 15 scanned states; no browser errors or external requests**. Build and formatting passed. Includes duplicate submit dispatch and dirty-editor browser-history protection. [Results](../design-prototypes/webui/evidence/results.json), [visual QA](../design-prototypes/webui/design-qa.md).

Screenshots: [conflict](../design-prototypes/webui/evidence/mutation-conflict.png), [mobile conflict](../design-prototypes/webui/evidence/mobile-mutation-conflict.png), [unknown outcome](../design-prototypes/webui/evidence/mutation-unknown.png), [partial inspection](../design-prototypes/webui/evidence/mutation-partial.png). Images are ignored local evidence, not release artifacts.

Try the [prototype](http://127.0.0.1:18224/): Edit configuration → expand Mock write scenarios → choose a scenario → change a value → review and apply. Real API acceptance, release approval and remote qualification remain separate gates.
