package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
)

func TestOIDCVerifiesRolesAudienceAndKeyRotation(t *testing.T) {
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	provider := &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: key1.Public(), KeyID: "key-1", Algorithm: oidc.RS256}}}
	server := httptest.NewServer(provider)
	defer server.Close()
	provider.SetIssuer(server.URL)
	verifier, err := NewOIDC(context.Background(), OIDCConfig{Issuer: server.URL, Audience: "management", RoleClaim: "roles", OperatorRole: "ops", AuditorRole: "audit", AllowInsecureIssuer: true})
	if err != nil {
		t.Fatal(err)
	}
	claims := func(subject string, roles any) map[string]any {
		return map[string]any{"iss": server.URL, "aud": "management", "sub": subject, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "roles": roles}
	}
	sign := func(key *rsa.PrivateKey, kid string, value map[string]any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return oidctest.SignIDToken(key, kid, oidc.RS256, string(raw))
	}
	operatorToken := sign(key1, "key-1", claims("alice", []string{"audit", "ops"}))
	principal, err := verifier.Verify(context.Background(), operatorToken)
	if err != nil || principal.Role != "operator" || principal.Actor != "oidc:"+server.URL+"#alice" {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
	if principal.ExpiresAt.IsZero() || time.Until(principal.ExpiresAt) <= 0 || time.Until(principal.ExpiresAt) > time.Hour {
		t.Fatalf("verified expiry not propagated: %v", principal.ExpiresAt)
	}
	auditorToken := sign(key1, "key-1", claims("bob", "audit"))
	principal, err = verifier.Verify(context.Background(), auditorToken)
	if err != nil || principal.Role != "auditor" {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)
	provider.PublicKeys = []oidctest.PublicKey{{PublicKey: key2.Public(), KeyID: "key-2", Algorithm: oidc.RS256}}
	rotated := sign(key2, "key-2", claims("carol", []string{"ops"}))
	if principal, err = verifier.Verify(context.Background(), rotated); err != nil || principal.Role != "operator" {
		t.Fatalf("rotated principal=%+v err=%v", principal, err)
	}
	wrongAudience := claims("mallory", []string{"ops"})
	wrongAudience["aud"] = "other"
	if _, err = verifier.Verify(context.Background(), sign(key2, "key-2", wrongAudience)); err == nil {
		t.Fatal("wrong audience was accepted")
	}
}

func TestOIDCBrowserCodeExchangeUsesFixedEndpointAndVerifiesIDToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	provider := &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "browser-key", Algorithm: oidc.RS256}}}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			provider.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.ParseForm() != nil || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "one-time" || r.Form.Get("code_verifier") != strings.Repeat("v", 43) || r.Form.Get("client_id") != "management" || r.Form.Get("redirect_uri") != "http://127.0.0.1:9443/admin/oidc/callback" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		claims, _ := json.Marshal(map[string]any{"iss": server.URL, "aud": "management", "sub": "browser-user", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "roles": []string{"ops"}})
		token := oidctest.SignIDToken(key, "browser-key", oidc.RS256, string(claims))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "must-not-be-returned", "refresh_token": "must-not-be-returned", "token_type": "Bearer", "id_token": token})
	}))
	defer server.Close()
	provider.SetIssuer(server.URL)
	verifier, err := NewOIDC(context.Background(), OIDCConfig{Issuer: server.URL, Audience: "management", RoleClaim: "roles", OperatorRole: "ops", AuditorRole: "audit", AllowInsecureIssuer: true, BrowserClientID: "management", BrowserRedirectOrigin: "http://127.0.0.1:9443"})
	if err != nil {
		t.Fatal(err)
	}
	config := verifier.BrowserLogin()
	if config == nil || config.AuthorizationEndpoint != server.URL+"/auth" || config.CallbackURL != "http://127.0.0.1:9443/admin/oidc/callback" {
		t.Fatalf("browser config=%+v", config)
	}
	raw, err := verifier.ExchangeBrowserCode(context.Background(), "one-time", strings.Repeat("v", 43))
	if err != nil || strings.Contains(raw, "must-not-be-returned") {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
	principal, err := verifier.Verify(context.Background(), raw)
	if err != nil || principal.Role != "operator" || principal.Actor != "oidc:"+server.URL+"#browser-user" {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
}

func TestOIDCRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	base := OIDCConfig{Issuer: "http://issuer.example", Audience: "management", RoleClaim: "roles", OperatorRole: "ops", AuditorRole: "audit"}
	if _, err := NewOIDC(context.Background(), base); err == nil {
		t.Fatal("insecure issuer was accepted")
	}
	base.Issuer = "https://"
	if _, err := NewOIDC(context.Background(), base); err == nil {
		t.Fatal("invalid issuer was accepted")
	}
}

func TestClaimStringsRejectsInvalidType(t *testing.T) {
	if _, err := claimStrings([]byte(`42`)); err == nil {
		t.Fatal("numeric role claim was accepted")
	}
}
