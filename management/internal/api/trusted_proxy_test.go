package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

func TestClientIPRespectsTrustedProxyHops(t *testing.T) {
	tests := []struct {
		name      string
		hops      int
		forwarded string
		want      string
	}{
		{name: "direct peer is authoritative without trusted proxies", hops: 0, forwarded: "198.51.100.9", want: "10.0.0.1"},
		{name: "single hop strips the rightmost proxy", hops: 1, forwarded: "198.51.100.9", want: "198.51.100.9"},
		{name: "two hops leave the client at the chain head", hops: 2, forwarded: "203.0.113.5, 198.51.100.9", want: "203.0.113.5"},
		{name: "shorter chain than the trust configuration falls back to the peer", hops: 3, forwarded: "198.51.100.9", want: "10.0.0.1"},
		{name: "missing header falls back to the peer", hops: 1, forwarded: "", want: "10.0.0.1"},
		{name: "non-IP attribution falls back to the peer", hops: 1, forwarded: "not-an-ip", want: "10.0.0.1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = "10.0.0.1:41000"
			if test.forwarded != "" {
				request.Header.Set("X-Forwarded-For", test.forwarded)
			}
			handler := &Handler{trustedProxyHops: test.hops}
			if got := handler.clientIP(request); got != test.want {
				t.Fatalf("clientIP=%q want %q", got, test.want)
			}
		})
	}
}

func TestLoginLimiterKeysOnForwardedClientBehindTrustedProxy(t *testing.T) {
	attempt := func(handler http.Handler, client string) int {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"wrong password value"}`))
		request.RemoteAddr = "10.0.0.1:41000"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", client)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}
	issuer := &fakePasswordIssuer{err: errors.New("bad")}
	proxied := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{Local: issuer, TrustedProxyHops: 1})
	for range 5 {
		if got := attempt(proxied, "198.51.100.9"); got != http.StatusUnauthorized {
			t.Fatalf("client A failure status=%d", got)
		}
	}
	if got := attempt(proxied, "198.51.100.9"); got != http.StatusTooManyRequests {
		t.Fatalf("client A must be rate limited, status=%d", got)
	}
	if got := attempt(proxied, "198.51.100.77"); got != http.StatusUnauthorized {
		t.Fatalf("client B must keep its own budget behind a trusted proxy, status=%d", got)
	}

	shared := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{Local: issuer})
	for range 5 {
		if got := attempt(shared, "198.51.100.9"); got != http.StatusUnauthorized {
			t.Fatalf("shared bucket failure status=%d", got)
		}
	}
	if got := attempt(shared, "198.51.100.77"); got != http.StatusTooManyRequests {
		t.Fatalf("without trusted proxies the peer bucket must be shared, status=%d", got)
	}
}

func TestExpiredLocalTokenReturnsDistinctUnauthorizedCode(t *testing.T) {
	request := func(verifier identity.Verifier) *httptest.ResponseRecorder {
		handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{LocalVerifier: verifier})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}
	expired := request(fakeIdentityVerifier{err: identity.ErrTokenExpired})
	if expired.Code != http.StatusUnauthorized || !strings.Contains(expired.Body.String(), `"code":"token_expired"`) {
		t.Fatalf("expired status=%d body=%s", expired.Code, expired.Body.String())
	}
	invalid := request(fakeIdentityVerifier{err: errors.New("signature mismatch")})
	if invalid.Code != http.StatusUnauthorized || !strings.Contains(invalid.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}
