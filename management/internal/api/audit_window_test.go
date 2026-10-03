package api

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type windowAuditFake struct {
	*fakeBackend
	calls  int
	filter jetstream.AuditFilter
	before *uint64
}

func (f *windowAuditFake) AuditWindow(ctx context.Context, filter jetstream.AuditFilter, before *uint64) (jetstream.AuditWindowPage, error) {
	f.calls++
	f.filter = filter
	f.before = before
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded audit read")
	}
	return jetstream.AuditWindowPage{Items: []jetstream.AuditEvent{}, Filter: filter}, f.err
}
func TestAuditWindowAuthorizationAndValidation(t *testing.T) {
	for _, tc := range []struct {
		token, query string
		status       int
	}{
		{"", "", 401}, {"wrong", "", 401}, {"operator", "", 200}, {"auditor", "?requestId=r&resource=orders&actor=owner&phase=outcome&action=queue.apply&outcome=accepted&before=18446744073709551615", 200},
		{"operator", "?phase=unknown", 400}, {"operator", "?actor=a&actor=b", 400}, {"operator", "?actor=%00", 400}, {"operator", "?actor=%FF", 400}, {"operator", "?q=x", 400}, {"operator", "?offset=1", 400}, {"operator", "?before=-1", 400}, {"operator", "?before=18446744073709551616", 400},
	} {
		t.Run(tc.token+tc.query, func(t *testing.T) {
			backend := &windowAuditFake{fakeBackend: &fakeBackend{}}
			h := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
			r := httptest.NewRequest("GET", "/api/v1/audit/windows"+tc.query, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.status != 200 && backend.calls != 0 {
				t.Fatal("invalid or unauthorized request reached backend")
			}
			if tc.token == "auditor" && (backend.filter.Actor != "owner" || backend.before == nil || *backend.before != ^uint64(0)) {
				t.Fatalf("lost filter/cursor: %+v", backend)
			}
		})
	}
}
