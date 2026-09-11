package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type deletePreviewBackend struct {
	fakeBackend
	reads       int
	hasDeadline bool
}

func (f *deletePreviewBackend) PreviewDelete(ctx context.Context, name string, condition jetstream.ApplyPrecondition) (*jetstream.DeletePreview, error) {
	f.reads++
	_, f.hasDeadline = ctx.Deadline()
	return f.fakeBackend.PreviewDelete(ctx, name, condition)
}

func (f *fakeBackend) PreviewDelete(_ context.Context, name string, precondition jetstream.ApplyPrecondition) (*jetstream.DeletePreview, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &jetstream.DeletePreview{Queue: name, BaseRevision: fmt.Sprintf("\"%d\"", *precondition.ExpectedRevision)}, nil
}

func TestDeletePreviewHTTPReadOnlyAuthorizationAndPrecondition(t *testing.T) {
	for _, tc := range []struct {
		name, token, match, create, query string
		status                            int
	}{
		{"operator", "operator", `"7"`, "", "", 200}, {"auditor", "auditor", `"7"`, "", "", 403},
		{"missing auth", "", `"7"`, "", "", 401}, {"missing revision", "operator", "", "", "", 428},
		{"create only", "operator", "", "*", "", 428}, {"force query", "operator", `"7"`, "", "?force=true", 400},
		{"weak revision", "operator", `W/"7"`, "", "", 428}, {"both conditions", "operator", `"7"`, "*", "", 428},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &deletePreviewBackend{}
			handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
			request := httptest.NewRequest("GET", "/api/v1/queues/orders/delete-preview"+tc.query, nil)
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			request.Header.Set("If-Match", tc.match)
			request.Header.Set("If-None-Match", tc.create)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if backend.auditCalls != 0 || backend.applyCalls != 0 || backend.deleteCalls != 0 {
				t.Fatal("preview caused mutation/audit")
			}
			if (tc.status == 200 && (backend.reads != 1 || !backend.hasDeadline)) || (tc.status != 200 && backend.reads != 0) {
				t.Fatal("unexpected backend read or missing timeout")
			}
			if tc.status == 200 && (recorder.Header().Get("ETag") != `"7"` || recorder.Header().Get("Cache-Control") != "no-store") {
				t.Fatal("missing revision/no-store headers")
			}
		})
	}
}

func TestDeletePreviewHTTPBackendFailures(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{jetstream.ErrConflict, 409}, {jetstream.ErrNotFound, 404}, {context.DeadlineExceeded, 503}, {errors.New("offline"), 503},
	} {
		backend := &fakeBackend{err: tc.err}
		handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}})
		request := httptest.NewRequest("GET", "/api/v1/queues/orders/delete-preview", nil)
		request.Header.Set("Authorization", "Bearer operator")
		request.Header.Set("If-Match", `"7"`)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != tc.status || recorder.Header().Get("ETag") != "" {
			t.Fatalf("error=%v status=%d body=%s", tc.err, recorder.Code, recorder.Body.String())
		}
	}
}
