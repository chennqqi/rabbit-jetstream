package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
)

type fakeBrowserOIDC struct {
	config         *managementauth.BrowserConfig
	token          string
	err            error
	code, verifier string
}

func (f *fakeBrowserOIDC) BrowserLogin() *managementauth.BrowserConfig { return f.config }
func (f *fakeBrowserOIDC) ExchangeBrowserCode(_ context.Context, code, verifier string) (string, error) {
	f.code, f.verifier = code, verifier
	return f.token, f.err
}

func TestBrowserOIDCPreLoginFlow(t *testing.T) {
	backend := &fakeBrowserOIDC{config: &managementauth.BrowserConfig{AuthorizationEndpoint: "https://id.example/authorize", ClientID: "management", CallbackURL: "https://console.example/admin/oidc/callback", Scopes: []string{"openid", "profile"}}, token: "signed.id.token"}
	handler := NewWithControllerAuth(&fakeBackend{}, slog.Default(), "test", "dev", nil, nil, AuthConfig{BrowserOIDC: backend})
	config := httptest.NewRecorder()
	handler.ServeHTTP(config, httptest.NewRequest(http.MethodGet, "/api/v1/oidc/config", nil))
	if config.Code != 200 || config.Header().Get("Cache-Control") != "no-store" || strings.Contains(config.Body.String(), "tokenEndpoint") {
		t.Fatalf("config status=%d headers=%v body=%s", config.Code, config.Header(), config.Body.String())
	}
	verifier := strings.Repeat("a", 43)
	exchange := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/oidc/token", strings.NewReader(`{"schema":"rjs.browser-oidc-callback.v1","code":"one-time","verifier":"`+verifier+`"}`))
	handler.ServeHTTP(exchange, request)
	if exchange.Code != 200 || exchange.Header().Get("Cache-Control") != "no-store" || backend.code != "one-time" || backend.verifier != verifier || !strings.Contains(exchange.Body.String(), "signed.id.token") {
		t.Fatalf("exchange status=%d code=%q verifier=%q body=%s", exchange.Code, backend.code, backend.verifier, exchange.Body.String())
	}
}

func TestBrowserOIDCDisabledInvalidAndSafeFailure(t *testing.T) {
	disabled := NewWithControllerAuth(&fakeBackend{}, slog.Default(), "test", "dev", nil, nil, AuthConfig{})
	for _, spec := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/api/v1/oidc/config", "", 404}, {"POST", "/api/v1/oidc/token", "{}", 404}, {"GET", "/api/v1/oidc/config?x=1", "", 400}} {
		response := httptest.NewRecorder()
		disabled.ServeHTTP(response, httptest.NewRequest(spec.method, spec.path, strings.NewReader(spec.body)))
		if response.Code != spec.status {
			t.Fatalf("%s %s status=%d", spec.method, spec.path, response.Code)
		}
	}
	failing := &fakeBrowserOIDC{config: &managementauth.BrowserConfig{ClientID: "x"}, err: errors.New("private provider details")}
	handler := NewWithControllerAuth(&fakeBackend{}, slog.Default(), "test", "dev", nil, nil, AuthConfig{BrowserOIDC: failing})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/oidc/token", strings.NewReader(`{"schema":"rjs.browser-oidc-callback.v1","code":"x","verifier":"`+strings.Repeat("a", 43)+`"}`)))
	if response.Code != 401 || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
