package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

// Hide the optional AuditBackend interface while retaining the normal backend.
type withoutMutationAudit struct{ Backend }

func TestMutationEvidencePhases(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		for _, tc := range []struct {
			name, phase, effects, code string
			missing                    bool
			failAt, calls, status      int
			backendErr                 error
		}{
			{name: "missing audit", phase: "audit_intent", effects: "none", code: "audit_unavailable", missing: true, status: 503},
			{name: "intent failure", phase: "audit_intent", effects: "none", code: "audit_unavailable", failAt: 1, status: 503},
			{name: "outcome failure", phase: "audit_outcome", effects: "possible", code: "audit_unavailable", failAt: 2, calls: 1, status: 503},
			{name: "backend failure", phase: "backend", effects: "possible", code: "jetstream_unavailable", backendErr: errors.New("offline"), calls: 1, status: 503},
			{name: "DLQ cycle retains conservative evidence", phase: "backend", effects: "possible", code: "dlq_dependency_cycle", backendErr: jetstream.ErrDeadLetterCycle, calls: 1, status: 400},
			{name: "conflict", phase: "backend", effects: "possible", code: "conflict", backendErr: jetstream.ErrConflict, calls: 1, status: 409},
			{name: "failed backend and outcome", phase: "audit_outcome", effects: "possible", code: "audit_unavailable", backendErr: jetstream.ErrConflict, failAt: 2, calls: 1, status: 503},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				backend := &fakeBackend{auditFailAt: tc.failAt, err: tc.backendErr}
				var client Backend = backend
				if tc.missing {
					client = withoutMutationAudit{backend}
				}
				h := New(client, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
				req := httptest.NewRequest(method, "/api/v1/queues/orders", strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`))
				req.Header.Set("Authorization", "Bearer secret")
				req.Header.Set("If-None-Match", "*")
				req.Header.Set("X-RJS-Confirm-Queue", "orders")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				var body struct {
					Error struct {
						Code     string
						Mutation mutationEvidence
					}
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				e := body.Error.Mutation
				if rec.Code != tc.status || body.Error.Code != tc.code || backend.applyCalls+backend.deleteCalls != tc.calls || e.SchemaVersion != "rjs.mutation-evidence.v1" || e.Scope != "receiving-attempt" || e.Phase != tc.phase || e.ResourceEffects != tc.effects {
					t.Fatalf("status=%d calls=%d/%d body=%s", rec.Code, backend.applyCalls, backend.deleteCalls, rec.Body.String())
				}
				if tc.missing {
					if e.IntentID != "" {
						t.Fatal("missing audit must not invent intent")
					}
				} else if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(e.IntentID) {
					t.Fatalf("invalid intent %q", e.IntentID)
				}
				if len(backend.auditEvents) > 0 && e.IntentID != backend.auditEvents[0].ID {
					t.Fatal("wrong audit correlation")
				}
				if len(backend.auditEvents) > 1 && backend.auditEvents[1].IntentID != e.IntentID {
					t.Fatal("wrong outcome correlation")
				}
			})
		}
	}
}

func TestReadFailureHasNoMutationEvidence(t *testing.T) {
	h := New(&fakeBackend{err: errors.New("offline")}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/streams", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 503 || strings.Contains(rec.Body.String(), `"mutation"`) {
		t.Fatalf("unexpected read error: %d %s", rec.Code, rec.Body.String())
	}
}
