# Lossless editable Queue documents

[English](webui-queue-document.md) | [简体中文](webui-queue-document.zh-CN.md)

Status: source implemented, unreleased; partial C-02 delivery. Existing declaration storage and apply behavior are unchanged.

`GET /api/v1/queues/{queue}` now adds:

- `document`: a normalized versioned Queue document, or `null` if conversion cannot be proven lossless.
- `document_error`: present when the editor must not use the declaration as an editable draft.

All existing fields and the original quoted KV `ETag` remain available. HTTP 200 with `document=null` means the declaration is inspectable but not safely editable through this adapter; it is not a missing Queue or successful migration. No broker resource or declaration is changed. Existing anonymous read policy is unchanged pending D-05.

The backend reconstructs labels, original subjects/bindings, replicas/storage, retention, delivery, priority and DLQ, then runs the canonical `BuildPlan`. The complete generated Plan JSON must equal the original Plan JSON, including content revision, metadata, derived resources and warnings. Queue identity and declaration/plan revision must also agree. This intentionally rejects older plans missing necessary provenance, unsupported metadata/settings, inconsistent names and changed content revisions. It never guesses missing subjects or silently fills a lossy default.

Defaults are normalized, so lexical duration/unit formatting and explicit versus omitted default values may differ; generated configuration must not. Omitted priority remains omitted and explicit priority zero remains zero. Go integer/nanosecond values retain their precision. The browser must use the [lossless decoder](../admin-ui/src/api.mjs), never ordinary `JSON.parse` before converting large integers.

Editor integration requirements:

1. Use `document` as the only draft source, preserving its supported fields and labels. Do not fall back to the old browser `queueFromPlan` guess when document is null.
2. Keep the ETag from this read as an opaque string throughout edit, preview and apply. The document's generated content hash is not the mutation precondition.
3. Null document blocks editing with the reason visible; read-only Plan/observation inspection remains available. Any legacy migration needs an explicit separate path, not an automatic write during GET.
4. Changed document/base/schema invalidates the preview. Form and expert JSON views share the same draft and exact serializer.

Verification performed:

```sh
go test -count=1 ./internal/topology ./management/internal/api ./management/internal/jetstream ./api
node --test admin-ui/tests/api.test.mjs
go vet ./...
```

All passed locally. Tests cover labels, subject and direct/topic/fanout binding variants, DLQ, priority omitted/0/7/255, max int64 values, nanoseconds, no input aliasing/mutation, rejecting unrepresentable plans, HTTP additive compatibility, exact large ETag and read-only calls. No live NATS or browser integration is claimed. Full Queue/Plan JSON Schema, deployment capabilities and actual form integration remain outstanding; this does not close C-02, WP-02 or the overall development goal.
