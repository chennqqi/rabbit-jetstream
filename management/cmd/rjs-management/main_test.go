package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
)

func TestHealthcheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("RJS_HEALTHCHECK_URL", server.URL)
	if err := healthcheck(); err != nil {
		t.Fatal(err)
	}
}

func TestHashPasswordReadsStdinAndProducesUsableArgon2id(t *testing.T) {
	var output strings.Builder
	if err := hashPassword(strings.NewReader("correct horse battery staple\n"), &output); err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSpace(output.String())
	authenticator, err := managementauth.NewLocal(managementauth.LocalConfig{SigningKey: []byte(strings.Repeat("k", 32)), TTL: time.Minute, Accounts: []managementauth.LocalAccount{{Username: "alice", PasswordHash: hash, Role: "operator", Tenants: []string{"local"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authenticator.Issue(context.Background(), "alice", "correct horse battery staple"); err != nil {
		t.Fatalf("generated hash could not authenticate: %v", err)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash output contains plaintext")
	}
}

func TestHashPasswordRejectsUnsafeLengths(t *testing.T) {
	for _, value := range []string{"short\n", strings.Repeat("x", 1025)} {
		if err := hashPassword(strings.NewReader(value), &strings.Builder{}); err == nil {
			t.Fatalf("accepted password length %d", len(value))
		}
	}
}

func TestHealthcheckRejectsUnhealthyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("RJS_HEALTHCHECK_URL", server.URL)
	if err := healthcheck(); err == nil {
		t.Fatal("healthcheck succeeded for an unhealthy response")
	}
}

func TestVersionRequested(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		if !versionRequested(args) {
			t.Fatalf("versionRequested(%q) = false", args)
		}
	}
	for _, args := range [][]string{nil, {"-version"}, {"version", "extra"}} {
		if versionRequested(args) {
			t.Fatalf("versionRequested(%q) = true", args)
		}
	}
}
