package api

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
)

// The application previously wired a nil *LocalAuthenticator into the
// LocalVerifier interface whenever no local accounts file was configured.
// The typed nil passed the `LocalVerifier != nil` guard, and the first
// request carrying an unrecognized bearer token panicked the process
// (auth.(*LocalAuthenticator).Verify dereferencing a nil receiver), which
// net/http converts into a dropped connection with no response. These tests
// pin the failure path: an unrecognized token must yield a clean 401 even if
// a typed nil ever reaches the auth fields again.
func TestSessionWithTypedNilLocalVerifier(t *testing.T) {
	h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), auth: AuthConfig{OperatorTokens: tokenList("operator"), LocalVerifier: (*managementauth.LocalAuthenticator)(nil)}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer not-a-real-token")
	h.session(recorder, request)
	if recorder.Code != 401 {
		t.Fatalf("status=%d body=%s, want 401 unauthorized", recorder.Code, recorder.Body)
	}
}

func TestSessionWithTypedNilLocalIssuer(t *testing.T) {
	h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), auth: AuthConfig{OperatorTokens: tokenList("operator"), Local: (*managementauth.LocalAuthenticator)(nil), LocalVerifier: (*managementauth.LocalAuthenticator)(nil)}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer not-a-real-token")
	h.session(recorder, request)
	if recorder.Code != 401 {
		t.Fatalf("status=%d body=%s, want 401 unauthorized", recorder.Code, recorder.Body)
	}
}
