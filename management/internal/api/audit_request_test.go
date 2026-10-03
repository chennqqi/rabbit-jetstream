package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type requestAuditFake struct {
	*fakeBackend
	calls int
}

func TestAuditEvidenceFailuresDoNotExposeDependencyDetails(t *testing.T) {
	dependency := errors.New("read nats://operator:super-secret@nats-1:4222: unavailable")
	for _, tc := range []struct {
		path    string
		backend Backend
	}{
		{"/api/v1/audit/requests/request-123", &requestAuditFake{fakeBackend: &fakeBackend{err: dependency}}},
		{"/api/v1/audit/windows", &windowAuditFake{fakeBackend: &fakeBackend{err: dependency}}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			h := NewWithControllerAuth(tc.backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil,
				AuthConfig{OperatorTokens: []string{"operator"}})
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Authorization", "Bearer operator")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"audit_unavailable"`) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "super-secret") || strings.Contains(w.Body.String(), "nats-1") {
				t.Fatalf("dependency detail exposed: %s", w.Body.String())
			}
		})
	}
}

func (f *requestAuditFake) AuditRequest(ctx context.Context, id string, before *uint64) (jetstream.AuditRequestPage, error) {
	f.calls++
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded request")
	}
	return jetstream.AuditRequestPage{RequestID: id, Items: []jetstream.AuditEvent{}}, f.err
}
func TestAuditRequestAuthorizationAndQueries(t *testing.T) {
	for _, tc := range []struct {
		token, query string
		status       int
	}{
		{"", "", 401}, {"wrong", "", 401}, {"operator", "", 200}, {"auditor", "?before=18446744073709551615", 200},
		{"operator", "?before=-1", 400}, {"operator", "?before=1&before=2", 400}, {"operator", "?other=x", 400}, {"operator", "?before=18446744073709551616", 400},
	} {
		t.Run(tc.token+tc.query, func(t *testing.T) {
			backend := &requestAuditFake{fakeBackend: &fakeBackend{}}
			h := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
			r := httptest.NewRequest("GET", "/api/v1/audit/requests/request-123"+tc.query, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.status != 200 && backend.calls != 0 {
				t.Fatal("invalid request reached backend")
			}
		})
	}
}
