package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

func TestLocalAuthenticator_AccountLifecycleInvalidatesIssuedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	hash, err := HashLocalPassword("initial-password-123")
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"version":"rjs.local-accounts.v1","accounts":[{"username":"admin","password_hash":%q,"role":"operator","tenants":["local"],"platform_admin":true}]}`, hash)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewLocalFromFile(path, []byte(strings.Repeat("k", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := authenticator.Issue(context.Background(), "admin", "initial-password-123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = authenticator.CreateAccount(context.Background(), identity.LocalAccountChange{Username: "reader", Password: "reader-password-123", Memberships: []identity.TenantMembership{{Tenant: "local", Role: "auditor"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = authenticator.UpdateAccount(context.Background(), identity.LocalAccountChange{Username: "admin", Password: "", Memberships: []identity.TenantMembership{{Tenant: "local", Role: "operator"}}, PlatformAdmin: true, Disabled: true})
	if err == nil {
		t.Fatal("last enabled platform administrator was disabled")
	}
	_, err = authenticator.UpdateAccount(context.Background(), identity.LocalAccountChange{Username: "admin", Memberships: []identity.TenantMembership{{Tenant: "local", Role: "operator"}, {Tenant: "team-a", Role: "auditor"}}, PlatformAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.Verify(context.Background(), token); !errors.Is(err, ErrInvalidLocalCredentials) {
		t.Fatalf("old token remained valid: %v", err)
	}
	reloaded, err := NewLocalFromFile(path, []byte(strings.Repeat("k", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reloaded.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Username != "admin" || len(items[0].Memberships) != 2 || items[0].Memberships[1].Role != "auditor" {
		t.Fatalf("unexpected persisted accounts: %#v", items)
	}
}

func TestLocalAuthenticator_IssueAndVerify(t *testing.T) {
	password := "correct horse battery staple"
	hash, err := HashLocalPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	authenticator, err := NewLocal(LocalConfig{
		SigningKey: []byte(strings.Repeat("k", 32)), TTL: 15 * time.Minute, Now: func() time.Time { return now },
		Accounts: []LocalAccount{{Username: "alice", PasswordHash: hash, Role: "operator", Tenants: []string{"local", "team-a"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	token, issued, err := authenticator.Issue(context.Background(), "alice", password)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || strings.Contains(token, "alice") || issued.Actor != "local:alice" || issued.Role != "operator" || !issued.ExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("unexpected issued token/principal: token=%q principal=%+v", token, issued)
	}
	verified, err := authenticator.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Actor != issued.Actor || verified.Role != issued.Role || !verified.ExpiresAt.Equal(issued.ExpiresAt) || strings.Join(verified.Tenants, ",") != "local,team-a" {
		t.Fatalf("verified principal mismatch: %+v", verified)
	}
	verified.Tenants[0] = "changed"
	if issued.Tenants[0] != "local" {
		t.Fatal("tenant slices must not alias")
	}
}

func TestLocalAuthenticatorValidatesTenantRegistry(t *testing.T) {
	hash, err := HashLocalPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewLocal(LocalConfig{SigningKey: []byte("01234567890123456789012345678901"), TTL: time.Minute, Accounts: []LocalAccount{{Username: "alice", PasswordHash: hash, Role: "operator", Tenants: []string{"blue"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := authenticator.ValidateTenants(map[string]struct{}{"green": {}}); err == nil {
		t.Fatal("ValidateTenants() accepted unknown tenant")
	}
	if err := authenticator.ValidateTenants(map[string]struct{}{"blue": {}}); err != nil {
		t.Fatalf("ValidateTenants() = %v", err)
	}
}

func TestNewLocalFromFileStrictlyLoadsVersionedAccounts(t *testing.T) {
	hash, err := HashLocalPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accounts.json")
	document := fmt.Sprintf(`{"version":"rjs.local-accounts.v1","accounts":[{"username":"alice","password_hash":%q,"role":"operator","tenants":["local"]}]}`, hash)
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewLocalFromFile(path, []byte(strings.Repeat("k", 32)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, principal, err := authenticator.Issue(context.Background(), "alice", "a sufficiently long password"); err != nil || principal.Actor != "local:alice" {
		t.Fatalf("loaded account failed: %+v %v", principal, err)
	}
	for index, invalid := range []string{
		`{"version":"future","accounts":[]}`,
		`{"version":"rjs.local-accounts.v1","accounts":[],"extra":true}`,
		document + `{}`,
	} {
		invalidPath := filepath.Join(t.TempDir(), fmt.Sprintf("invalid-%d.json", index))
		if err := os.WriteFile(invalidPath, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewLocalFromFile(invalidPath, []byte(strings.Repeat("k", 32)), time.Minute); err == nil {
			t.Fatalf("invalid account file %d accepted", index)
		}
	}
}

func TestLocalAuthenticator_RejectsCredentialsTamperingAndExpiry(t *testing.T) {
	hash, err := HashLocalPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	authenticator, err := NewLocal(LocalConfig{SigningKey: []byte(strings.Repeat("s", 32)), TTL: time.Minute, Now: func() time.Time { return now }, Accounts: []LocalAccount{{Username: "auditor", PasswordHash: hash, Role: "auditor", Tenants: []string{"local"}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct{ username, password string }{{"missing", "a sufficiently long password"}, {"auditor", "wrong password value"}, {"", ""}} {
		if _, _, issueErr := authenticator.Issue(context.Background(), attempt.username, attempt.password); !errors.Is(issueErr, ErrInvalidLocalCredentials) {
			t.Fatalf("expected generic credential error for %#v, got %v", attempt, issueErr)
		}
	}
	token, _, err := authenticator.Issue(context.Background(), "auditor", "a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[1] = parts[1][:len(parts[1])-1] + "A"
	if _, err := authenticator.Verify(context.Background(), strings.Join(parts, ".")); !errors.Is(err, ErrInvalidLocalCredentials) {
		t.Fatalf("tampered token accepted: %v", err)
	}
	now = now.Add(time.Minute)
	if _, err := authenticator.Verify(context.Background(), token); !errors.Is(err, identity.ErrTokenExpired) || errors.Is(err, ErrInvalidLocalCredentials) {
		t.Fatalf("expired token must report identity.ErrTokenExpired, got %v", err)
	}
}

func TestLocalAuthenticator_ValidatesConfiguration(t *testing.T) {
	hash, err := HashLocalPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	valid := LocalAccount{Username: "alice", PasswordHash: hash, Role: "operator", Tenants: []string{"local"}}
	tests := []LocalConfig{
		{SigningKey: []byte("short"), TTL: time.Minute, Accounts: []LocalAccount{valid}},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: 25 * time.Hour, Accounts: []LocalAccount{valid}},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute, Accounts: []LocalAccount{{Username: "bad user", PasswordHash: hash, Role: "operator", Tenants: []string{"local"}}}},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute, Accounts: []LocalAccount{{Username: "alice", PasswordHash: hash, Role: "owner", Tenants: []string{"local"}}}},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute, Accounts: []LocalAccount{{Username: "alice", PasswordHash: hash, Role: "operator", Tenants: []string{"bad tenant"}}}},
		{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute, Accounts: []LocalAccount{valid, valid}},
	}
	for index, config := range tests {
		if _, err := NewLocal(config); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}
}
