package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

func TestSession(t *testing.T) {
	expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for _, tc := range []struct {
		name, token string
		auth        AuthConfig
		status      int
		role        string
		expiry      *time.Time
	}{
		{"operator", "ops-secret", AuthConfig{OperatorTokens: []string{"ops-secret"}}, 200, "operator", nil},
		{"auditor", "audit-secret", AuthConfig{AuditorTokens: []string{"audit-secret"}}, 200, "auditor", nil},
		{"missing token", "", AuthConfig{OperatorTokens: []string{"ops-secret"}}, 401, "", nil},
		{"wrong token", "wrong", AuthConfig{OperatorTokens: []string{"ops-secret"}}, 401, "", nil},
		{"disabled", "ops-secret", AuthConfig{}, 404, "", nil},
		{"oidc", "jwt-secret", AuthConfig{OIDC: fakeIdentityVerifier{principal: identity.Principal{Actor: "oidc:issuer#alice", Role: "operator", ExpiresAt: expiry}}}, 200, "operator", &expiry},
		{"oidc no role", "jwt-secret", AuthConfig{OIDC: fakeIdentityVerifier{principal: identity.Principal{Actor: "alice"}}}, 403, "", nil},
		{"oidc invalid", "jwt-secret", AuthConfig{OIDC: fakeIdentityVerifier{err: errors.New("private verifier details")}}, 401, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &fakeBackend{}
			handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, tc.auth)
			request := httptest.NewRequest("GET", "/api/v1/session", nil)
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Set-Cookie") != "" {
				t.Fatal("session persisted")
			}
			if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "private verifier") {
				t.Fatal("credential/internal error disclosed")
			}
			if recorder.Code == 401 && recorder.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing auth challenge")
			}
			if tc.status == 200 {
				var got sessionResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Role != tc.role || got.Actor == "" || got.ResourceReadPolicy != "anonymous" {
					t.Fatalf("identity=%+v", got)
				}
				if !slices.Contains(got.Permissions, "audit:read") || slices.Contains(got.Permissions, "queue:apply") != (tc.role == "operator") {
					t.Fatalf("permissions=%v", got.Permissions)
				}
				if tc.expiry == nil && got.ExpiresAt != nil {
					t.Fatal("fabricated static-token expiry")
				}
				if tc.expiry != nil && (got.ExpiresAt == nil || !got.ExpiresAt.Equal(*tc.expiry)) {
					t.Fatal("verified expiry lost")
				}
			}
			if backend.applyCalls+backend.deleteCalls+backend.auditCalls != 0 {
				t.Fatal("identity read had side effects")
			}
		})
	}
}
