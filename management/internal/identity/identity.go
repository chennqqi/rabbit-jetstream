package identity

import "context"

// Principal is an authenticated management-plane identity.
type Principal struct {
	Actor string
	Role  string
}

// Verifier validates a bearer credential and returns its principal.
type Verifier interface {
	Verify(context.Context, string) (Principal, error)
}
