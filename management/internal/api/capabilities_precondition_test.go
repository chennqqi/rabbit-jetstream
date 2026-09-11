package api

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCapabilitiesETagIdentity(t *testing.T) {
	base := &Handler{version: "one"}
	tag := base.capabilitiesETag()
	if !capabilitiesTagPattern.MatchString(tag) {
		t.Fatal(tag)
	}
	if tag != (&Handler{version: "one", name: "other"}).capabilitiesETag() {
		t.Fatal("name changed contract identity")
	}
	for _, other := range []*Handler{{version: "two"}, {version: "one", console: ConsoleConfig{DeploymentProfile: "cluster"}}} {
		if tag == other.capabilitiesETag() {
			t.Fatal("contract change not bound")
		}
	}
}

func TestCapabilitiesPreconditionRejectsBeforeBackendOrAudit(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"PUT", "/api/v1/queues/orders"}, {"DELETE", "/api/v1/queues/orders"}, {"POST", "/api/v1/queues/orders/preview"}, {"GET", "/api/v1/queues/orders/delete-preview"}} {
		for _, tc := range []struct {
			name   string
			values []string
			want   int
		}{
			{"changed", []string{(&Handler{version: "different"}).capabilitiesETag()}, 412},
			{"empty", []string{""}, 400}, {"weak", []string{"W/" + (&Handler{version: "dev"}).capabilitiesETag()}, 400},
			{"multiple", []string{"one", "two"}, 400},
		} {
			t.Run(route.method+route.path+tc.name, func(t *testing.T) {
				backend := &fakeBackend{}
				h := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}})
				r := httptest.NewRequest(route.method, route.path, strings.NewReader("not even parsed"))
				r.Header.Set("Authorization", "Bearer operator")
				for _, value := range tc.values {
					r.Header.Add(capabilitiesMatchHeader, value)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
				}
				if backend.auditCalls != 0 || backend.applyCalls != 0 || backend.deleteCalls != 0 {
					t.Fatal("precondition touched mutation/audit")
				}
			})
		}
	}
}

func TestCapabilitiesMatchingAndLegacyConditions(t *testing.T) {
	h := &Handler{version: "dev"}
	for _, tag := range []string{"", h.capabilitiesETag()} {
		r := httptest.NewRequest("PUT", "/", nil)
		if tag != "" {
			r.Header.Set(capabilitiesMatchHeader, tag)
		}
		if !h.checkCapabilitiesPrecondition(httptest.NewRecorder(), r) {
			t.Fatal("matching/legacy condition rejected")
		}
	}
}
