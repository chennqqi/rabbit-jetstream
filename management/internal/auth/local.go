package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

var ErrInvalidLocalCredentials = errors.New("invalid local credentials")

type LocalAccount struct {
	Username      string                      `json:"username"`
	PasswordHash  string                      `json:"password_hash"`
	Role          string                      `json:"role,omitempty"`
	Tenants       []string                    `json:"tenants,omitempty"`
	Memberships   []identity.TenantMembership `json:"memberships,omitempty"`
	Disabled      bool                        `json:"disabled,omitempty"`
	PlatformAdmin bool                        `json:"platform_admin,omitempty"`
}

type LocalConfig struct {
	SigningKey []byte
	TTL        time.Duration
	Accounts   []LocalAccount
	Now        func() time.Time
}

type localAccountFile struct {
	Version  string         `json:"version"`
	Accounts []LocalAccount `json:"accounts"`
}

func NewLocalFromFile(path string, signingKey []byte, ttl time.Duration) (*LocalAuthenticator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open local accounts file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 1<<20 {
		return nil, errors.New("local accounts file exceeds 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var document localAccountFile
	if err := decoder.Decode(&document); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("decode local accounts file")
	}
	if document.Version != "rjs.local-accounts.v1" && document.Version != "rjs.local-accounts.v2" {
		return nil, errors.New("unsupported local accounts file version")
	}
	authenticator, err := NewLocal(LocalConfig{SigningKey: signingKey, TTL: ttl, Accounts: document.Accounts})
	if err != nil {
		return nil, err
	}
	authenticator.path = path
	return authenticator, nil
}

type LocalAuthenticator struct {
	mu       sync.RWMutex
	key      []byte
	ttl      time.Duration
	accounts map[string]LocalAccount
	now      func() time.Time
	dummy    string
	path     string
}

// ValidateTenants rejects accounts that could authenticate into an
// unconfigured routing target.
func (a *LocalAuthenticator) ValidateTenants(known map[string]struct{}) error {
	for _, account := range a.accounts {
		for _, membership := range account.Memberships {
			id := membership.Tenant
			if _, ok := known[id]; !ok {
				return fmt.Errorf("local account %q references unknown tenant %q", account.Username, id)
			}
		}
	}
	return nil
}

type localClaims struct {
	Subject       string            `json:"sub"`
	Role          string            `json:"role"`
	Tenants       []string          `json:"tenants"`
	Issued        int64             `json:"iat"`
	Expires       int64             `json:"exp"`
	Nonce         string            `json:"jti"`
	PlatformAdmin bool              `json:"platform_admin,omitempty"`
	TenantRoles   map[string]string `json:"tenant_roles"`
}

func NewLocal(config LocalConfig) (*LocalAuthenticator, error) {
	if len(config.SigningKey) < 32 {
		return nil, errors.New("local authentication signing key must contain at least 32 bytes")
	}
	if config.TTL <= 0 || config.TTL > 24*time.Hour {
		return nil, errors.New("local authentication TTL must be between zero and 24 hours")
	}
	accounts := make(map[string]LocalAccount, len(config.Accounts))
	for _, account := range config.Accounts {
		account, err := normalizeLocalAccount(account)
		if err != nil {
			return nil, fmt.Errorf("invalid local account %q: %w", account.Username, err)
		}
		if _, exists := accounts[account.Username]; exists {
			return nil, fmt.Errorf("duplicate local account %q", account.Username)
		}
		if _, err := parseArgon2id(account.PasswordHash); err != nil {
			return nil, fmt.Errorf("local account %q password hash: %w", account.Username, err)
		}
		accounts[account.Username] = account
	}
	if len(accounts) == 0 {
		return nil, errors.New("at least one local account is required")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	dummy, err := HashLocalPassword("constant-time-invalid-account-password")
	if err != nil {
		return nil, err
	}
	return &LocalAuthenticator{key: append([]byte(nil), config.SigningKey...), ttl: config.TTL, accounts: accounts, now: now, dummy: dummy}, nil
}

func HashLocalPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password must contain 12 to 1024 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	value := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(value)), nil
}

func (a *LocalAuthenticator) Issue(_ context.Context, username, password string) (string, identity.Principal, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	account, exists := a.accounts[username]
	hash := a.dummy
	if exists {
		hash = account.PasswordHash
	}
	valid := verifyArgon2id(password, hash)
	if !exists || !valid || account.Disabled {
		return "", identity.Principal{}, ErrInvalidLocalCredentials
	}
	now := a.now().UTC().Truncate(time.Second)
	expires := now.Add(a.ttl)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", identity.Principal{}, fmt.Errorf("generate access token nonce: %w", err)
	}
	tenants, roles, role := accountAuthorization(account)
	claims := localClaims{Subject: username, Role: role, Tenants: tenants, TenantRoles: roles, Issued: now.Unix(), Expires: expires.Unix(), Nonce: base64.RawURLEncoding.EncodeToString(nonce), PlatformAdmin: account.PlatformAdmin}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", identity.Principal{}, fmt.Errorf("encode access token: %w", err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"RJS-AT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	signed := header + "." + body
	token := signed + "." + base64.RawURLEncoding.EncodeToString(a.sign(signed))
	return token, identity.Principal{Actor: "local:" + username, Role: role, ExpiresAt: expires, Tenants: tenants, TenantRoles: roles, PlatformAdmin: account.PlatformAdmin}, nil
}

func (a *LocalAuthenticator) Verify(_ context.Context, token string) (identity.Principal, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"RJS-AT"}`)) {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(signature, a.sign(parts[0]+"."+parts[1])) != 1 {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	var claims localClaims
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil || !validLocalName(claims.Subject) || (claims.Role != "operator" && claims.Role != "auditor") || len(claims.Tenants) == 0 || len(claims.TenantRoles) != len(claims.Tenants) || claims.Issued <= 0 || claims.Expires <= claims.Issued || claims.Expires-claims.Issued > int64((24*time.Hour)/time.Second) || claims.Nonce == "" {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	now := a.now().UTC().Unix()
	if claims.Issued > now+30 {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	if claims.Expires <= now {
		return identity.Principal{}, identity.ErrTokenExpired
	}
	for _, tenant := range claims.Tenants {
		if !validLocalName(tenant) || (claims.TenantRoles[tenant] != "operator" && claims.TenantRoles[tenant] != "auditor") {
			return identity.Principal{}, ErrInvalidLocalCredentials
		}
	}
	account, ok := a.accounts[claims.Subject]
	tenants, roles, role := accountAuthorization(account)
	if !ok || account.Disabled || role != claims.Role || account.PlatformAdmin != claims.PlatformAdmin || !sameStrings(tenants, claims.Tenants) || !sameRoles(roles, claims.TenantRoles) {
		return identity.Principal{}, ErrInvalidLocalCredentials
	}
	return identity.Principal{Actor: "local:" + claims.Subject, Role: claims.Role, ExpiresAt: time.Unix(claims.Expires, 0).UTC(), Tenants: append([]string(nil), claims.Tenants...), TenantRoles: cloneRoles(claims.TenantRoles), PlatformAdmin: claims.PlatformAdmin}, nil
}

func sameRoles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
func cloneRoles(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func accountAuthorization(account LocalAccount) ([]string, map[string]string, string) {
	tenants := make([]string, 0, len(account.Memberships))
	roles := make(map[string]string, len(account.Memberships))
	role := "auditor"
	for _, membership := range account.Memberships {
		tenants = append(tenants, membership.Tenant)
		roles[membership.Tenant] = membership.Role
		if membership.Role == "operator" {
			role = "operator"
		}
	}
	return tenants, roles, role
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a *LocalAuthenticator) ListAccounts(context.Context) ([]identity.LocalAccountView, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	items := make([]identity.LocalAccountView, 0, len(a.accounts))
	for _, account := range a.accounts {
		items = append(items, accountView(account))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Username < items[j].Username })
	return items, nil
}

func (a *LocalAuthenticator) CreateAccount(_ context.Context, change identity.LocalAccountChange) (identity.LocalAccountView, error) {
	if change.Password == "" {
		return identity.LocalAccountView{}, fmt.Errorf("password is required: %w", identity.ErrAccountValidation)
	}
	hash, err := HashLocalPassword(change.Password)
	if err != nil {
		return identity.LocalAccountView{}, err
	}
	account := LocalAccount{Username: change.Username, PasswordHash: hash, Memberships: cloneMemberships(change.Memberships), Disabled: change.Disabled, PlatformAdmin: change.PlatformAdmin}
	account, err = normalizeLocalAccount(account)
	if err != nil {
		return identity.LocalAccountView{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.accounts[account.Username]; exists {
		return identity.LocalAccountView{}, identity.ErrAccountExists
	}
	if err := validateLocalAccount(account); err != nil {
		return identity.LocalAccountView{}, err
	}
	a.accounts[account.Username] = account
	if err := a.persistLocked(); err != nil {
		delete(a.accounts, account.Username)
		return identity.LocalAccountView{}, err
	}
	return accountView(account), nil
}

func (a *LocalAuthenticator) UpdateAccount(_ context.Context, change identity.LocalAccountChange) (identity.LocalAccountView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	previous, exists := a.accounts[change.Username]
	if !exists {
		return identity.LocalAccountView{}, identity.ErrAccountNotFound
	}
	next := previous
	next.Memberships = cloneMemberships(change.Memberships)
	next.Disabled = change.Disabled
	next.PlatformAdmin = change.PlatformAdmin
	if change.Password != "" {
		hash, err := HashLocalPassword(change.Password)
		if err != nil {
			return identity.LocalAccountView{}, err
		}
		next.PasswordHash = hash
	}
	next, err := normalizeLocalAccount(next)
	if err != nil {
		return identity.LocalAccountView{}, err
	}
	a.accounts[change.Username] = next
	if !a.hasEnabledPlatformAdminLocked() {
		a.accounts[change.Username] = previous
		return identity.LocalAccountView{}, fmt.Errorf("at least one enabled platform administrator is required: %w", identity.ErrAccountPolicy)
	}
	if err := a.persistLocked(); err != nil {
		a.accounts[change.Username] = previous
		return identity.LocalAccountView{}, err
	}
	return accountView(next), nil
}

func (a *LocalAuthenticator) DeleteAccount(_ context.Context, username string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	previous, exists := a.accounts[username]
	if !exists {
		return identity.ErrAccountNotFound
	}
	delete(a.accounts, username)
	if !a.hasEnabledPlatformAdminLocked() {
		a.accounts[username] = previous
		return fmt.Errorf("at least one enabled platform administrator is required: %w", identity.ErrAccountPolicy)
	}
	if err := a.persistLocked(); err != nil {
		a.accounts[username] = previous
		return err
	}
	return nil
}

func validateLocalAccount(account LocalAccount) error {
	if !validLocalName(account.Username) || len(account.Memberships) == 0 {
		return fmt.Errorf("invalid local account: %w", identity.ErrAccountValidation)
	}
	if _, err := parseArgon2id(account.PasswordHash); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, membership := range account.Memberships {
		id := membership.Tenant
		if !validLocalName(id) || (membership.Role != "operator" && membership.Role != "auditor") {
			return fmt.Errorf("invalid tenant: %w", identity.ErrAccountValidation)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate tenant: %w", identity.ErrAccountValidation)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func accountView(account LocalAccount) identity.LocalAccountView {
	return identity.LocalAccountView{Username: account.Username, Memberships: cloneMemberships(account.Memberships), Disabled: account.Disabled, PlatformAdmin: account.PlatformAdmin}
}
func cloneMemberships(items []identity.TenantMembership) []identity.TenantMembership {
	return append([]identity.TenantMembership(nil), items...)
}

func normalizeLocalAccount(account LocalAccount) (LocalAccount, error) {
	if len(account.Memberships) == 0 {
		if (account.Role != "operator" && account.Role != "auditor") || len(account.Tenants) == 0 {
			return account, fmt.Errorf("memberships are required: %w", identity.ErrAccountValidation)
		}
		account.Memberships = make([]identity.TenantMembership, 0, len(account.Tenants))
		for _, id := range account.Tenants {
			account.Memberships = append(account.Memberships, identity.TenantMembership{Tenant: id, Role: account.Role})
		}
	} else if account.Role != "" || len(account.Tenants) != 0 {
		return account, fmt.Errorf("legacy role/tenants cannot be combined with memberships: %w", identity.ErrAccountValidation)
	}
	account.Role = ""
	account.Tenants = nil
	sort.Slice(account.Memberships, func(i, j int) bool { return account.Memberships[i].Tenant < account.Memberships[j].Tenant })
	if err := validateLocalAccount(account); err != nil {
		return account, err
	}
	return account, nil
}
func (a *LocalAuthenticator) hasEnabledPlatformAdminLocked() bool {
	for _, account := range a.accounts {
		if account.PlatformAdmin && !account.Disabled {
			return true
		}
	}
	return false
}
func (a *LocalAuthenticator) persistLocked() error {
	if a.path == "" {
		return fmt.Errorf("local account store is not writable: %w", identity.ErrAccountStoreUnavailable)
	}
	accounts := make([]LocalAccount, 0, len(a.accounts))
	for _, account := range a.accounts {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Username < accounts[j].Username })
	document := localAccountFile{Version: "rjs.local-accounts.v2", Accounts: accounts}
	directory := filepath.Dir(a.path)
	temp, err := os.CreateTemp(directory, ".rjs-local-accounts-*")
	if err != nil {
		return fmt.Errorf("create account file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	rollback := func(err error) error {
		temp.Close()
		return fmt.Errorf("%v: %w", err, identity.ErrAccountStoreUnavailable)
	}
	if err := temp.Chmod(0600); err != nil {
		return rollback(err)
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return rollback(err)
	}
	if err := temp.Sync(); err != nil {
		return rollback(err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("%v: %w", err, identity.ErrAccountStoreUnavailable)
	}
	if err := os.Rename(tempName, a.path); err != nil {
		return fmt.Errorf("replace local accounts file: %w: %w", err, identity.ErrAccountStoreUnavailable)
	}
	return nil
}

func (a *LocalAuthenticator) sign(value string) []byte {
	mac := hmac.New(sha256.New, a.key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

type argon2idHash struct {
	memory uint32
	time   uint32
	lanes  uint8
	salt   []byte
	value  []byte
}

func parseArgon2id(encoded string) (argon2idHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argon2idHash{}, errors.New("unsupported Argon2id encoding")
	}
	var result argon2idHash
	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return result, errors.New("invalid Argon2id parameters")
	}
	values := make(map[string]uint64, 3)
	for _, parameter := range parameters {
		pair := strings.SplitN(parameter, "=", 2)
		if len(pair) != 2 {
			return result, errors.New("invalid Argon2id parameter")
		}
		value, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return result, errors.New("invalid Argon2id parameter")
		}
		values[pair[0]] = value
	}
	if values["m"] != 64*1024 || values["t"] != 3 || values["p"] != 2 {
		return result, errors.New("Argon2id parameters must be m=65536,t=3,p=2")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return result, errors.New("invalid Argon2id salt")
	}
	value, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(value) != 32 {
		return result, errors.New("invalid Argon2id value")
	}
	result.memory, result.time, result.lanes, result.salt, result.value = uint32(values["m"]), uint32(values["t"]), uint8(values["p"]), salt, value
	return result, nil
}

func verifyArgon2id(password, encoded string) bool {
	parsed, err := parseArgon2id(encoded)
	if err != nil || len(password) > 1024 {
		return false
	}
	value := argon2.IDKey([]byte(password), parsed.salt, parsed.time, parsed.memory, parsed.lanes, uint32(len(parsed.value)))
	return subtle.ConstantTimeCompare(value, parsed.value) == 1
}

func validLocalName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char == '-' || char == '_' || char == '.' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return false
		}
	}
	return true
}
