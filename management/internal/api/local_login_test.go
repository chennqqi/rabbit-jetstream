package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type fakePasswordIssuer struct {
	token     string
	principal identity.Principal
	err       error
	username  string
	password  string
}

func (f *fakePasswordIssuer) Issue(_ context.Context, username, password string) (string, identity.Principal, error) {
	f.username, f.password = username, password
	return f.token, f.principal, f.err
}

func TestLocalLoginIssuesBearerWithoutCookie(t *testing.T) {
	expires := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	issuer := &fakePasswordIssuer{token: "short-lived-token", principal: identity.Principal{Actor: "local:alice", Role: "operator", ExpiresAt: expires, Tenants: []string{"local", "team-a"}}}
	handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{Local: issuer})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"correct horse battery staple"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if issuer.username != "alice" || issuer.password != "correct horse battery staple" {
		t.Fatalf("issuer received unexpected credentials: %q/%q", issuer.username, issuer.password)
	}
	if recorder.Header().Get("Set-Cookie") != "" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected login headers: %#v", recorder.Header())
	}
	var response localLoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken != issuer.token || response.TokenType != "Bearer" || response.Actor != "local:alice" || response.Role != "operator" || !response.ExpiresAt.Equal(expires) || strings.Join(response.Tenants, ",") != "local,team-a" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestLocalLoginFailsClosed(t *testing.T) {
	valid := `{"username":"alice","password":"correct horse battery staple"}`
	tests := []struct {
		name, target, contentType, body string
		issuer                          *fakePasswordIssuer
		status                          int
		code                            string
	}{
		{"disabled", "/api/v1/auth/login", "application/json", valid, nil, 404, "local_auth_disabled"},
		{"query", "/api/v1/auth/login?next=/admin", "application/json", valid, &fakePasswordIssuer{}, 400, "invalid_query"},
		{"content type", "/api/v1/auth/login", "text/plain", valid, &fakePasswordIssuer{}, 415, "invalid_content_type"},
		{"unknown field", "/api/v1/auth/login", "application/json", `{"username":"alice","password":"secret value","tenant":"local"}`, &fakePasswordIssuer{}, 400, "invalid_login_request"},
		{"trailing JSON", "/api/v1/auth/login", "application/json", valid + `{}`, &fakePasswordIssuer{}, 400, "invalid_login_request"},
		{"bad credentials", "/api/v1/auth/login", "application/json", valid, &fakePasswordIssuer{err: errors.New("private detail")}, 401, "invalid_credentials"},
		{"invalid issuer result", "/api/v1/auth/login", "application/json", valid, &fakePasswordIssuer{token: "token", principal: identity.Principal{Actor: "local:alice", Role: "operator"}}, 503, "local_auth_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth := AuthConfig{}
			if test.issuer != nil {
				auth.Local = test.issuer
			}
			handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, auth)
			request := httptest.NewRequest(http.MethodPost, test.target, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !bytes.Contains(recorder.Body.Bytes(), []byte(`"code":"`+test.code+`"`)) || bytes.Contains(recorder.Body.Bytes(), []byte("private detail")) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestLocalLoginRateLimitsRepeatedFailures(t *testing.T) {
	issuer := &fakePasswordIssuer{err: errors.New("bad")}
	handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{Local: issuer})
	for attempt := 1; attempt <= 6; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"wrong password value"}`))
		request.RemoteAddr = "192.0.2.10:1234"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d: status=%d body=%s", attempt, recorder.Code, recorder.Body.String())
		}
		if attempt == 6 && recorder.Header().Get("Retry-After") == "" {
			t.Fatal("rate limited response omitted Retry-After")
		}
	}
}
