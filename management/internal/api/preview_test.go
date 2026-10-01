package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type previewBackend struct {
	*fakeBackend
	calls        int
	precondition jetstream.ApplyPrecondition
}

func (f *previewBackend) Preview(ctx context.Context, plan topology.Plan, precondition jetstream.ApplyPrecondition) (*jetstream.PlanPreview, error) {
	f.calls++
	f.precondition = precondition
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded preview")
	}
	return f.fakeBackend.Preview(ctx, plan, precondition)
}

func TestQueuePreviewHTTP(t *testing.T) {
	document := `{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.events"],"replicas":1}}`
	for _, tc := range []struct {
		name, token, match, create, body string
		status, calls                    int
		err                              error
		blocked                          bool
	}{
		{name: "create", token: "operator", create: "*", body: document, status: 200, calls: 1},
		{name: "edit", token: "operator", match: `"7"`, body: document, status: 200, calls: 1},
		{name: "blocked is preview", token: "operator", match: `"7"`, body: document, status: 200, calls: 1, blocked: true},
		{name: "missing token", create: "*", body: document, status: 401},
		{name: "auditor", token: "auditor", create: "*", body: document, status: 403},
		{name: "bad document", token: "operator", create: "*", body: "invalid", status: 400},
		{name: "no precondition", token: "operator", body: document, status: 428},
		{name: "ambiguous precondition", token: "operator", match: `"7"`, create: "*", body: document, status: 428},
		{name: "name mismatch", token: "operator", create: "*", body: strings.Replace(document, `"name":"orders"`, `"name":"another"`, 1), status: 409},
		{name: "conflict", token: "operator", match: `"7"`, body: document, status: 409, calls: 1, err: jetstream.ErrConflict},
		{name: "unavailable", token: "operator", create: "*", body: document, status: 503, calls: 1, err: context.DeadlineExceeded},
		{name: "DLQ cycle", token: "operator", create: "*", body: document, status: 400, calls: 1, err: fmt.Errorf("dependency: %w", jetstream.ErrDeadLetterCycle)},
		{name: "oversize", token: "operator", create: "*", body: document + strings.Repeat(" ", 1<<20), status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &previewBackend{fakeBackend: &fakeBackend{err: tc.err, applyResult: topology.ReconcileResult{Blocked: tc.blocked}}}
			handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
			request := httptest.NewRequest("POST", "/api/v1/queues/orders/preview", strings.NewReader(tc.body))
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			request.Header.Set("If-Match", tc.match)
			request.Header.Set("If-None-Match", tc.create)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status || backend.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, backend.calls, recorder.Body.String())
			}
			if backend.auditCalls != 0 || backend.applyCalls != 0 || backend.deleteCalls != 0 {
				t.Fatal("preview caused mutation/audit")
			}
			if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("preview cached")
			}
			if tc.status == 200 {
				var got jetstream.PlanPreview
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Result.Blocked != tc.blocked || got.Plan.Queue != "orders" {
					t.Fatalf("preview=%+v", got)
				}
				if tc.match != "" && (backend.precondition.ExpectedRevision == nil || *backend.precondition.ExpectedRevision != 7) {
					t.Fatal("original precondition lost")
				}
			}
			if tc.name == "DLQ cycle" && !strings.Contains(recorder.Body.String(), `"code":"dlq_dependency_cycle"`) {
				t.Fatal(recorder.Body.String())
			}
		})
	}
}

func TestQueuePreviewDisabledWithoutAuth(t *testing.T) {
	backend := &previewBackend{fakeBackend: &fakeBackend{}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/v1/queues/orders/preview", strings.NewReader("invalid")))
	if recorder.Code != 404 || backend.calls != 0 || backend.auditCalls != 0 {
		t.Fatalf("status=%d calls=%d", recorder.Code, backend.calls)
	}
}
