# Management API Versioning

`/api/v1` is a stable operations contract. The exact OpenAPI 3.1 document shipped by a server is available without authentication at `/api/v1/openapi.yaml` and in [api/openapi.yaml](../api/openapi.yaml).

Within v1, releases may add endpoints, optional request fields, response fields, enum values, and new error codes. Clients must ignore unknown response fields and error codes they do not special-case. Existing paths, methods, documented response status codes, properties, required fields, and property types are not removed or changed.

Deprecations are documented in OpenAPI and release notes for at least two minor releases. A replacement and migration procedure must exist before removal. A necessary breaking change uses a new major path such as `/api/v2`; v1 remains available through the announced support window. Security fixes may disable unsafe behavior, but must preserve a machine-readable error response whenever disclosure is safe.

Queue resource `apiVersion` is versioned independently from the HTTP API. Adding `/api/v2` does not silently rewrite stored Queue declarations.

CI compares the proposed contract with the parent revision using `tools/apicompat`. It rejects removed paths, operations, response codes, schemas/properties, required fields, and property type changes. This automated floor does not replace review of semantic changes such as units, ordering, defaults, authorization, or pagination behavior.

Run a local comparison with:

```shell
go run ./tools/apicompat --baseline previous-openapi.yaml --current api/openapi.yaml
```
