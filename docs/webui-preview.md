# Read-only Queue preview

[English](webui-preview.md) | [简体中文](webui-preview.zh-CN.md)

## DLQ dependency Stream validation

An observed cycle now returns HTTP 400 with `dlq_dependency_cycle`, distinct from broker unavailability and source revision conflict. Other dependency failures retain their existing classification; exceeding the traversal bound is not proof of a cycle. Preview failure preserves the editable draft without permitting apply or conflict rebase. A dispatched PUT rejection still remains uncertain in the browser: this error code is not request-outcome proof or automatic retry authorization.

The source implementation now also follows up to 100 saved target declarations, rejecting a return to the proposed source or any repeated target, missing/inconsistent/unrepresentable declarations, and a longer chain. Traversal, the direct target Stream read and all observed declaration revision rechecks share a five-second deadline (or the shorter caller deadline). No 101st target is read. This supersedes the earlier direct-cycle-only limitation below. Downstream Stream/Consumer health is not inferred; only the immediate target Stream is compared. Rechecking revisions detects observed changes but cannot make concurrent multi-Queue updates atomic.

Preview and apply now require more than a saved target declaration. The target must have consistent Queue/revision/Stream identity and a faithfully reconstructable canonical document. Its live Stream must exist and match the saved plan's ownership, routing Subjects, configuration and RJS metadata according to the same reconciliation comparison used for ordinary changes. Unrelated metadata remains allowed. The target declaration KV revision is checked again after the Stream read; changed observations fail. Apply repeats these checks after preview and before DLQ infrastructure/source resource changes.

No target is created, adopted or repaired by this check. A missing, foreign, drifted or unrepresentable target requires separate operator investigation/reconciliation; declarations that previously passed presence-only checks can now be rejected. These are bounded reads under the caller's context, not a cross-Queue transaction or protection against an external change after observation. Target Consumers, replica health and actual message delivery are not qualified by this check. The initial two-Queue cycle and priority compatibility checks remain in place; this does not claim general transitive cycle detection across existing declarations.

## Diagnostic field navigation

Recognized server field diagnostics now offer keyboard-operable “Locate field or JSON” buttons. Exact scalar fields, Subject indices, Binding exchange/type and routing-key indices map only to known controls within the current editor. Containing details open before focus. Group constraints, unknown paths and absent/disabled controls fall back to the original JSON textarea; no guessed child field, selector interpolation, draft mutation or API request is involved. Editing clears the old preview diagnostic and its navigation controls. This navigation is available only for eligible current read-only preview errors, never unknown write outcomes.

## Structured Queue validation diagnostics

Semantic Queue validation now returns additive `issues_version: rjs.queue-validation.v1` and `issues` containing JSON Pointer `path`, stable rule `code` and human-readable `message`. Existing `invalid_queue` and summary text remain. Preview and PUT reject before backend/audit work. Parser/type errors do not invent paths. Defaults are applied before validation, but canonical sorting happens only after successful validation, so array pointers refer to submitted order. Group constraints may point to `/spec` or `/spec/retention`; missing required values may point to absent fields.

The editor renders only recognized, bounded issue arrays on a current read-only preview 400 response. Unknown/malformed versions fall back to the existing bounded text; no prose parsing, HTML injection, automatic field editing or write-outcome inference occurs. This is structured diagnostics, not full schema-driven forms.

Status: implemented in source, unreleased; part of C-05, not completion of the WebUI goal.

Existing Stream/Consumer resources must carry the same nonempty `rabbit-jetstream.io/queue` marker as the generated plan. Missing or different markers block reconciliation with an ownership reason; automatic adoption is not supported. Preview returns the blocked result without writes; apply rechecks observation and returns 409 without changing resources or persisting the declaration (normal audit and queue-lock activity may occur). Previously unmarked resources now require explicit operator investigation, not an automatic overwrite. Matching metadata is observation evidence, not a substitute for broker access control or an atomic protection against external writers changing ownership after the read.

Resource metadata reconciliation synchronizes `rabbit-jetstream.io/` keys with the generated plan and preserves other observed keys unless explicitly supplied by that plan. Preview and apply share this rule. Metadata change values use quoted string representations, including `""` for an explicit empty value, and the unquoted marker `(absent)` for a missing key. This corrects previously omitted label removals/empty-value drift; consumers of change strings must account for the representation. External metadata preservation uses the current observation and is not an atomic merge with concurrent external writes.

The additive `declaration_review` response separates declaration changes from observed-resource reconciliation. `status=available` includes `diff` against the original `base_revision` and a normalized target `document`; `status=create` has the target document but no fabricated old declaration/diff. `status=unavailable` reports `target_unrepresentable`, `base_unrepresentable` or `base_identity_mismatch` rather than guessing a lossless conversion. Target reconstruction failure omits the document. Old servers may omit this entire field. Numeric configuration remains exact; duration/unit spelling and defaults may normalize. The submitted draft is not replaced. An empty declaration diff does not mean observed resources need no repair. Reads check the original KV revision while obtaining the comparison base and again before returning; this does not lock the declaration or reserve future apply. See the [execution record](webui-development.md) for current real-service verification; the original verification notes below describe the initial implementation.

`POST /api/v1/queues/{queue}/preview` accepts the same versioned Queue document and original conditional headers as apply. Operator authorization is required; auditor receives 403, missing/invalid token 401, and unconfigured authorization disables the route with 404. Example against a locally rebuilt service (use the actual host/port and a locally supplied operator token):

```sh
curl --fail http://127.0.0.1:8080/api/v1/queues/orders/preview \
  -H "Authorization: Bearer ${RJS_OPERATOR_TOKEN}" \
  -H 'Content-Type: application/json' -H 'If-None-Match: *' \
  --data '{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.events"],"replicas":1}}'
```

For editing, replace `If-None-Match: *` with the **original editor-load** `If-Match` value. Never replace it with a fresh revision merely to make a stale draft pass.

Response: `plan`, `result` (including operations/changes/impact/blocked), `observed_at`, `base_revision`, `create_only`. The plan's content revision is distinct from the quoted declaration KV version in `base_revision`; create-only uses an empty base revision. A blocked plan is a successful advisory read: **200 with `result.blocked=true`**, not a successful mutation. `ready` and `noop` are also plan results, not health or evidence that this preview applied anything.

Implementation reuses apply's precondition and DLQ dependency checks and the topology reconciler. It checks the declaration before and after observation. It does not acquire a write lock, create the metadata bucket, persist a declaration, ensure DLQ infrastructure, modify Streams/Consumers, or record audit intent/outcome. DLQ validation follows apply's ordering and limitations, including the current two-Queue cycle check; it does not claim generalized graph-cycle validation. An unsafe plan is reported before checking DLQ dependencies, like apply.

The request body is limited to 1 MiB, with a five-second read context and no-store responses. Malformed/oversized documents return 400; name mismatch and stale/colliding declaration return 409; absent/invalid precondition returns 428; missing edit target or DLQ dependency can return 404; backend failures return 503. These errors do not mutate resources. Complete response codes are registered in [OpenAPI](../api/openapi.yaml); the generated Plan remains a generic schema until C-02's full schema work is delivered.

`observed_at` marks advisory read completion, not a single atomic broker snapshot. Preview does not reserve a revision or operation slot. Changes to the draft/base/schema invalidate it; final apply must use the same original precondition and recheck safety under its normal lock. Passing preview does not prove future authorization, audit availability, quota/capacity or write success. Broker and dependency changes may occur after preview. The observation reader currently inherits existing Stream-local enumeration; only its request deadline is bounded here, not a demonstrated indexed/large-scale query capability.

Verification: backend tests use a read-only SDK/metadata wrapper whose unimplemented write/lock methods fail immediately; cover create/noop/blocked, old revision, create collision, missing target, failure/cancellation, DLQ missing/valid/cycle and concurrent declaration change. HTTP tests cover operator/auditor/disabled auth, request size, error/status handling and zero audit/apply/delete calls. A separate test verifies preview followed by another writer still causes conditional apply to fail.

Targeted API/backend tests and `go vet ./...` passed. The first `go test -count=1 ./...` run passed other packages but failed `tools/baremetal-run/TestSupervisedLifecycle` because Windows refused a temporary port bind; the test left existing listeners untouched. See the execution record for retry evidence. No live-service preview, browser integration, release deployment or remote operation was performed.
