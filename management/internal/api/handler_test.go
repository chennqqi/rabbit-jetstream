package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type fakeBackend struct {
	err          error
	account      jetstream.Account
	streams      []jetstream.Stream
	consumers    []jetstream.Consumer
	applyResult  topology.ReconcileResult
	deleteResult topology.DeleteResult
	deleteCalls  int
}

type fakeMonitor struct{ snapshot monitoring.Snapshot }

func (f fakeMonitor) Nodes(context.Context) monitoring.Snapshot { return f.snapshot }

func (f *fakeBackend) Ready(context.Context) error { return f.err }
func (f *fakeBackend) AccountInfo(context.Context) (*jetstream.Account, error) {
	return &f.account, f.err
}
func (f *fakeBackend) ServerURL() string { return "nats://nats-1:4222" }
func (f *fakeBackend) ListStreams(context.Context) ([]jetstream.Stream, error) {
	return f.streams, f.err
}
func (f *fakeBackend) Stream(_ context.Context, name string) (*jetstream.Stream, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.streams {
		if f.streams[i].Name == name {
			return &f.streams[i], nil
		}
	}
	return nil, jetstream.ErrNotFound
}
func (f *fakeBackend) ListConsumers(context.Context, string) ([]jetstream.Consumer, error) {
	return f.consumers, f.err
}
func (f *fakeBackend) Apply(context.Context, topology.Plan) (topology.ReconcileResult, error) {
	return f.applyResult, f.err
}
func (f *fakeBackend) DeleteQueue(context.Context, string, bool) (topology.DeleteResult, error) {
	f.deleteCalls++
	return f.deleteResult, f.err
}

func TestHealthEndpoint(t *testing.T) {
	h := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %#v", body)
	}
}

func TestReadOnlyManagementEndpoints(t *testing.T) {
	backend := &fakeBackend{
		account:   jetstream.Account{Streams: 2, Consumers: 1, APILevel: 4},
		streams:   []jetstream.Stream{{Name: "alpha", Messages: 10}, {Name: "beta", Messages: 20}},
		consumers: []jetstream.Consumer{{Stream: "alpha", Name: "worker", Pending: 3}},
	}
	handler := newTestHandler(backend)
	tests := []struct {
		path     string
		contains []string
	}{
		{"/api/v1/cluster", []string{`"server_url":"nats://nats-1:4222"`, `"api_level":4`}},
		{"/api/v1/nodes", []string{`"status":"available"`, `"name":"nats-1"`}},
		{"/api/v1/streams?offset=1&limit=1", []string{`"name":"beta"`, `"total":2`, `"offset":1`}},
		{"/api/v1/streams/alpha", []string{`"name":"alpha"`, `"messages":10`}},
		{"/api/v1/streams/alpha/consumers", []string{`"name":"worker"`, `"pending":3`}},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			for _, expected := range test.contains {
				if !strings.Contains(recorder.Body.String(), expected) {
					t.Errorf("body %q does not contain %q", recorder.Body.String(), expected)
				}
			}
		})
	}
}

func TestManagementEndpointErrors(t *testing.T) {
	tests := []struct {
		name    string
		backend *fakeBackend
		path    string
		status  int
		code    string
	}{
		{"invalid pagination", &fakeBackend{}, "/api/v1/streams?limit=0", http.StatusBadRequest, "invalid_pagination"},
		{"missing stream", &fakeBackend{}, "/api/v1/streams/missing", http.StatusNotFound, "not_found"},
		{"backend unavailable", &fakeBackend{err: errors.New("timeout")}, "/api/v1/streams", http.StatusServiceUnavailable, "jetstream_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newTestHandler(test.backend).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("body = %s", recorder.Body.String())
			}
		})
	}
}

func TestApplyQueueRequiresAuthenticationAndMatchingName(t *testing.T) {
	backend := &fakeBackend{applyResult: topology.ReconcileResult{Queue: "orders", Status: "ready"}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	body := `{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`
	for _, test := range []struct {
		name, path, token string
		status            int
	}{
		{"missing token", "/api/v1/queues/orders", "", http.StatusUnauthorized},
		{"wrong token", "/api/v1/queues/orders", "wrong", http.StatusUnauthorized},
		{"name mismatch", "/api/v1/queues/other", "secret", http.StatusConflict},
		{"accepted", "/api/v1/queues/orders", "secret", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, test.path, strings.NewReader(body))
			if test.token != "" {
				req.Header.Set("Authorization", "Bearer "+test.token)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != test.status {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestApplyQueueIsDisabledWithoutToken(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(&fakeBackend{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader("{}")))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestDeleteQueueRequiresExactConfirmation(t *testing.T) {
	backend := &fakeBackend{deleteResult: topology.DeleteResult{Queue: "orders", Stream: "RJSQ_orders", Status: "deleted"}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queues/orders", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || backend.deleteCalls != 0 {
		t.Fatalf("status = %d, calls = %d", rec.Code, backend.deleteCalls)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/queues/orders?force=true", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-RJS-Confirm-Queue", "orders")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || backend.deleteCalls != 1 {
		t.Fatalf("status = %d, calls = %d", rec.Code, backend.deleteCalls)
	}
}

func TestDeleteQueueReturnsConflictWhenBlocked(t *testing.T) {
	backend := &fakeBackend{deleteResult: topology.DeleteResult{Queue: "orders", Status: "blocked", Blocked: true, Messages: 2}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queues/orders", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-RJS-Confirm-Queue", "orders")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func newTestHandler(backend Backend) http.Handler {
	monitor := fakeMonitor{snapshot: monitoring.Snapshot{
		Status: "available", Total: 1, Available: 1,
		Nodes: []monitoring.Node{{Name: "nats-1", Status: "available"}},
	}}
	return New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", monitor)
}
