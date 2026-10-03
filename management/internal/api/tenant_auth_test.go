package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

func TestTenantAuthorizationIsServerEnforced(t *testing.T) {
	tests := []struct {
		name    string
		tenants []string
		headers []string
		path    string
		status  int
		code    string
	}{
		{"explicit member", []string{"team-a", "team-b"}, []string{"team-b"}, "/api/v1/console/build", 200, ""},
		{"multi membership requires selection", []string{"team-a", "team-b"}, nil, "/api/v1/console/build", 400, "tenant_required"},
		{"nonmember", []string{"team-a", "team-b"}, []string{"team-c"}, "/api/v1/console/build", 403, "tenant_forbidden"},
		{"invalid", []string{"team-a"}, []string{"bad tenant"}, "/api/v1/console/build", 400, "invalid_tenant"},
		{"duplicate header", []string{"team-a"}, []string{"team-a", "team-a"}, "/api/v1/console/build", 400, "invalid_tenant"},
		{"single membership defaults", []string{"team-a"}, nil, "/api/v1/console/build", 200, ""},
		{"session lists memberships before selection", []string{"team-a", "team-b"}, nil, "/api/v1/session", 200, ""},
		{"legacy credential defaults local", nil, nil, "/api/v1/console/build", 200, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := fakeIdentityVerifier{principal: identity.Principal{Actor: "local:alice", Role: "operator", Tenants: test.tenants}}
			handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, LocalVerifier: verifier})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set("Authorization", "Bearer signed")
			for _, value := range test.headers {
				request.Header.Add("X-RJS-Tenant", value)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status || test.code != "" && !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantRoleIsSelectedBeforeAuthorization(t *testing.T) {
	h := &Handler{auth: AuthConfig{LocalVerifier: fakeIdentityVerifier{principal: identity.Principal{Actor: "local:alice", Role: "operator", Tenants: []string{"a", "b"}, TenantRoles: map[string]string{"a": "operator", "b": "auditor"}}}}}
	for _, test := range []struct {
		tenant string
		status int
	}{{"a", 0}, {"b", http.StatusForbidden}} {
		req := httptest.NewRequest(http.MethodPost, "/write", nil)
		req.Header.Set("Authorization", "Bearer signed")
		req.Header.Set("X-RJS-Tenant", test.tenant)
		recorder := httptest.NewRecorder()
		ok := h.authorize(recorder, req, map[string]bool{"operator": true}, "disabled", "write API")
		if ok != (test.status == 0) || test.status != 0 && recorder.Code != test.status {
			t.Fatalf("tenant=%s ok=%v status=%d", test.tenant, ok, recorder.Code)
		}
		if ok {
			principal := req.Context().Value(principalKey{}).(identity.Principal)
			if principal.Role != "operator" {
				t.Fatalf("effective role=%s", principal.Role)
			}
		}
	}
}
