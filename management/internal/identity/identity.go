package identity

import (
	"context"
	"errors"
	"time"
)

// ErrTokenExpired reports a well-formed bearer credential whose validity
// window has passed. Verifiers return it so the API can answer 401 with the
// distinct `token_expired` code instead of the generic unauthorized signal;
// exposing expiry is safe because the holder already knows its own expiry.
var ErrTokenExpired = errors.New("access token expired")

// Local account store outcomes. The store returns these sentinels (wrapping
// its operational detail) so the access-management API can answer with
// distinct status codes instead of one flattened 409 bucket: 404 unknown
// account, 409 existing username or protection policy, 400 store-side
// validation, 503 persistence failure.
var (
	ErrAccountExists           = errors.New("account already exists")
	ErrAccountNotFound         = errors.New("account not found")
	ErrAccountPolicy           = errors.New("account change violates an account protection policy")
	ErrAccountValidation       = errors.New("account change is invalid")
	ErrAccountStoreUnavailable = errors.New("local account store is unavailable")
)

// Principal is an authenticated management-plane identity.
type Principal struct {
	Actor         string
	Role          string
	ExpiresAt     time.Time
	Tenants       []string
	PlatformAdmin bool
	TenantRoles   map[string]string
}

func (p Principal) RoleForTenant(tenant string) string {
	if role := p.TenantRoles[tenant]; role != "" {
		return role
	}
	return p.Role
}

// Verifier validates a bearer credential and returns its principal.
type Verifier interface {
	Verify(context.Context, string) (Principal, error)
}

// PasswordIssuer verifies a local account password and returns a short-lived
// bearer credential. Implementations must use the same generic error for an
// unknown account and an invalid password.
type PasswordIssuer interface {
	Issue(context.Context, string, string) (string, Principal, error)
}

// LocalAccountView is the credential-free representation exposed to access
// administrators. Password hashes never cross the package or API boundary.
type LocalAccountView struct {
	Username      string             `json:"username"`
	Memberships   []TenantMembership `json:"memberships"`
	Disabled      bool               `json:"disabled"`
	PlatformAdmin bool               `json:"platform_admin"`
}

type TenantMembership struct {
	Tenant string `json:"tenant"`
	Role   string `json:"role"`
}

type LocalAccountChange struct {
	Username      string
	Password      string
	Memberships   []TenantMembership
	Disabled      bool
	PlatformAdmin bool
}

// LocalAccountManager mutates the same durable account source used to issue
// and verify local access tokens.
type LocalAccountManager interface {
	ListAccounts(context.Context) ([]LocalAccountView, error)
	CreateAccount(context.Context, LocalAccountChange) (LocalAccountView, error)
	UpdateAccount(context.Context, LocalAccountChange) (LocalAccountView, error)
	DeleteAccount(context.Context, string) error
}
