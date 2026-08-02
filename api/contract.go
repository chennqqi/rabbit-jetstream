package contract

import _ "embed"

// OpenAPI is the immutable v1 management API contract shipped with the server.
//
//go:embed openapi.yaml
var OpenAPI []byte

// NativeSDK is the versioned resource, header, priority and delivery contract
// shared with native clients. Availability is explicit in the document.
//
//go:embed native-sdk-contract.json
var NativeSDK []byte
