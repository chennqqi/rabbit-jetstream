package contract

import _ "embed"

// OpenAPI is the immutable v1 management API contract shipped with the server.
//
//go:embed openapi.yaml
var OpenAPI []byte
