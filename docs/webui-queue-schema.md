# Queue authoring schema

[English](webui-queue-schema.md) | [简体中文](webui-queue-schema.zh-CN.md)

## Schema-controlled routing and label collections

The v1 adapter now compiles the collection contract as well as scalar fields. It checks string label values, optional label count/length bounds, nullable string-array Subjects with unique items, exclusive Subjects/Bindings branches, string exchange/key fields, unique keys and direct/topic/fanout key cardinalities. Rule summaries and expandable field-rule details come from that contract. No regular expression from metadata is executed in the browser; pattern/format rules remain server-validated.

Unsupported value alternatives, map structures, unsafe/reversed counts, changed exclusivity or unknown branch shapes disable the form adapter instead of leaving the old controls active. Reload preserves the raw draft. Array rows, duplicate Subjects/keys, empty and multiline label values remain recoverable for correction; rule display never sorts, deletes, fills or converts the draft. Mode/key removal keeps its existing explicit confirmation and shared JSON behavior.

This is a versioned adapter for the published Queue contract, with purpose-built collection widgets and local layout. It is not an arbitrary-schema layout generator or a replacement for authoritative backend validation. New unsupported contract structures require adapter support; they do not silently become editable.

## Schema-backed initial creation

Initial creation now loads the same authenticated capability/schema contract and compiled controls before enabling draft preparation. Replicas/storage options and the exact message-limit bounds come from that contract. The creation workflow still intentionally requires explicit storage/replicas and a positive initial message quota, even though the general Queue schema permits omitted storage and zero retention limits. No defaults are inserted into the form.

A missing, failed or changed contract disables prepare and deployment selectors but retains all entered values, including unavailable selections and exact integer text. Explicit reload validates the current contract without creating a Queue, previewing or writing. The prepare function also checks readiness and selected values, so a disabled button is not the only guard. Subsequent server preview and conditional create-only apply remain mandatory.

Starting another creation or parking the current draft resets rule authority before loading new rules; it does not reuse the old creation's approval. Disposing a contract suppresses late responses and unsubscribes observers. Form values remain owned by the existing session setup, independent of rule reads; navigation/remount rereads the contract without overwriting those values.

## Schema-derived scalar controls

The candidate draft editor now reads the advertised schema before enabling structured controls. Nine scalar fields derive types, enum choices, requiredness, exact integer bounds and omitted-default annotations from the verified contract; routing type choices also come from the schema. Labels/order and the supported v1 adapter remain local. Unknown field types, unsafe bounds, invalid enums or unsupported routing contracts disable the adapter. No default is inserted, no value is clamped, and blank still means omitted. Inputs remain text-based for exact int64 editing; preview is still authoritative.

Form metadata is session-owned separately from raw input and preview authority. Explicit reload is read-only. Failed/changed observations remove form rules while retaining raw/merge drafts and original ETags; invalid JSON also remains untouched by late metadata. A fresh successful preview can restore matching rules. Starting another edit after acceptance rereads rules. Form reads are canceled logically before preview/apply, archive or discard so late completion cannot rewrite retained outcomes. The prior full mutation guards still apply independently.

Creation's initial setup and supported routing/label collections now consume the same contract as described above. Collection layouts remain dedicated v1 widgets; arbitrary schema-generated layouts and future contract versions are not implemented.

Status: local candidate contract and UI integration, not frozen release qualification or complete schema-generated forms.

## Publication and identity

The single source is [internal/topology/queue-schema.json](../internal/topology/queue-schema.json), embedded by the shared topology package. It describes all Queue, metadata, routing, retention, delivery and dead-letter fields. Reflection tests fail when Go document fields are added without schema coverage; defaults and priority limits are checked against the parser.

`GET /api/v1/console/queue-schema` requires operator or auditor authentication, including local-demo mode. It performs no backend/monitor reads and returns the embedded bytes as `application/schema+json`, with `Cache-Control: no-store` and a strong `"rjs-queue-schema-v1:<SHA-256>"` ETag. Query parameters return 400; missing/invalid credentials 401, insufficient role 403, authentication disabled 404. The optional original `X-RJS-If-Capabilities-Match` is checked before publication; mismatches return 412.

Capabilities advertises `queue-schema` and `queue.schema.{id,version,url,etag}`. The schema ID is `urn:rabbit-jetstream:queue:v1alpha1:schema:1`; the consumer contract version is `rjs.queue-schema.v1`. The content revision participates in the capabilities ETag, so changing schema bytes invalidates old capability-bound reviews even if the reported binary version is unchanged. Neither ETag proves binary attestation, capacity or production qualification.

OpenAPI's Queue component references this same authenticated endpoint rather than maintaining another incomplete field list. Offline tools should use the source schema artifact; online reference resolution requires credentials.

## Meaning and limits

The dialect is [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-validation). `default` is an [annotation](https://json-schema.org/understanding-json-schema/reference/annotations), not an instruction to fill or mutate the draft. Replica selection remains required, omitted priority differs from explicit zero, and zero retention limits remain unset limits.

The schema enforces object fields, required identity/replicas, enumerations, integer bounds, unique Subject/key entries, routing exclusivity and direct/topic/fanout key cardinality. Binding-based normalized documents may have `subjects:null`. It describes typed authoring documents, not every legacy YAML scalar coercion; for example numeric label values can be accepted by the YAML parser but are not schema-valid string labels. Existing API parsing is not made stricter by publication.

Custom `rjs-duration`, `rjs-byte-size`, `rjs-subject` and `rjs-routing-key` formats are annotations for ordinary validators. Go duration nanosecond limits, size-unit multiplication overflow, wildcard syntax, dead-letter self-reference, generated-plan compatibility, ownership and declaration preconditions remain server-validated. Schema validation alone never authorizes a write. Exact int64 boundaries require lossless numeric handling; do not use JavaScript Number for those values.

## Candidate UI behavior

Settings reads capabilities and its schema together, verifies the pinned same-origin path, recognized descriptor/consumed shape, advertised body values and matching opaque ETag, and displays the schema without inserting defaults. Failed/incompatible refresh clears old displayed metadata; late reads cannot restore a cleared or superseded model.

When `queue-schema` is advertised, all capability-bound create/edit/delete preview and pre-dispatch checks also read the schema with the original capabilities condition. The review stores its schema, and subsequent checks compare the body as well as metadata. Missing/incompatible reads prevent dispatch and require explicit fresh review. Schema reads notify retained unsubmitted reviews; failed/changed observations invalidate confirmation while preserving drafts/ETags. In-flight/unknown/accepted/archived states are not unlocked. Old-credential responses cannot notify a new session.

The existing structured fields and routing/label controls remain explicit components; fully schema-driven form generation is still open. Server diagnostic navigation now focuses exact known controls, with original-JSON fallback for group/unknown/unavailable fields; see [preview diagnostics](webui-preview.md). Schema metadata is not a substitute for server preview or durable mutation-outcome resolution.

## Verification

Run locally:

```powershell
go test ./internal/topology ./management/internal/api ./api
python tests/admin-ui/queue-schema-check.py
cd admin-ui
npm.cmd test
npm.cmd run build
npm.cmd run test:live
```

The independent Python check requires `jsonschema` 4.x (validated with 4.25.1), uses the installed metaschema without network fetching, and shares 14 cases with Go parser tests. The corpus deliberately includes structural success/server failure and legacy-coercion differences. It is not exhaustive semantic equivalence proof. Frontend tests cover descriptor URL rejection, incompatible shapes/revisions, exact int64 preservation, changed schema bodies, failed refresh, and stale/old-session suppression. Browser tests use real authenticated schema data and inject incompatible/read-failure responses to verify Settings recovery and zero PUT dispatch.
