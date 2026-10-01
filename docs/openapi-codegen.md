# OpenAPI Model Generation

[简体中文](openapi-codegen.zh-CN.md)

The checked-in OpenAPI models are generated from [the published API contract](../api/openapi.yaml) and the authoritative [Queue authoring schema](../internal/topology/queue-schema.json). The public OpenAPI document intentionally points its `Queue` component at the authenticated runtime schema endpoint. `tools/openapibundle` replaces that reference in a temporary, generation-only document and rewrites Queue-local `$defs` references; generated code never depends on a running server.

Run:

```shell
make generate-openapi
make verify-openapi-generated
```

The PowerShell entry point is `scripts/openapi-codegen.ps1`. It pins `oapi-codegen` v2.8.0 and uses the exact `openapi-typescript` version in `admin-ui/package-lock.json`. Outputs are:

- `management/internal/apigen/models.gen.go` — Go schema models only; handlers and routes remain handwritten.
- `admin-ui/src/generated/openapi.d.ts` — TypeScript contract types only; the existing authenticated, tenant-aware, lossless HTTP transport remains handwritten.

Generated files are committed. CI regenerates into a temporary directory and compares exact content. Change the OpenAPI/Queue schemas or generator versions, run generation, review semantic changes, and commit the corresponding outputs together.

Wire integers marked `format: uint64` become Go `uint64` and TypeScript `bigint`. This is intentional: the Admin UI's lossless JSON decoder must not narrow broker counters, CIDs, sequence cursors, or replica lag to JavaScript `number`. Do not consume generated response types through ordinary `JSON.parse`.

Generic OpenAPI objects remain visible in generated output but are not permission to remove runtime validation. Migrate handlers and UI decoders incrementally only after the relevant response Schema is concrete and equivalent failure/precision tests exist.

