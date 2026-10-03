package api

import (
	"bytes"
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
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
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
	applyCalls   int
	declarations []topology.Declaration
	serverURL    string
	auditEvents  []jetstream.AuditEvent
	auditErr     error
	auditCalls   int
	auditFailAt  int
}

type fakeMonitor struct{ snapshot monitoring.Snapshot }
type fakeController struct{ status controller.Status }

func (f fakeMonitor) Nodes(context.Context) monitoring.Snapshot { return f.snapshot }
func (f fakeController) Status() controller.Status              { return f.status }

func (f *fakeBackend) Ready(context.Context) error { return f.err }
func (f *fakeBackend) AccountInfo(context.Context) (*jetstream.Account, error) {
	return &f.account, f.err
}
func (f *fakeBackend) ServerURL() string {
	if f.serverURL != "" {
		return f.serverURL
	}
	return "nats://nats-1:4222"
}
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
func (f *fakeBackend) QueueConsumers(context.Context, string) (*jetstream.QueueConsumerCollection, error) {
	return &jetstream.QueueConsumerCollection{Items: []jetstream.QueueConsumer{}}, f.err
}
func (f *fakeBackend) Preview(_ context.Context, plan topology.Plan, precondition jetstream.ApplyPrecondition) (*jetstream.PlanPreview, error) {
	return &jetstream.PlanPreview{Plan: plan, Result: f.applyResult, CreateOnly: precondition.CreateOnly}, f.err
}
func (f *fakeBackend) Consumer(_ context.Context, stream, name string) (*jetstream.Consumer, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.consumers {
		if f.consumers[i].Stream == stream && f.consumers[i].Name == name {
			return &f.consumers[i], nil
		}
	}
	return nil, jetstream.ErrNotFound
}
func (f *fakeBackend) Apply(context.Context, topology.Plan) (topology.ReconcileResult, error) {
	return f.applyResult, f.err
}
func (f *fakeBackend) ApplyConditional(context.Context, topology.Plan, jetstream.ApplyPrecondition) (topology.ReconcileResult, error) {
	f.applyCalls++
	return f.applyResult, f.err
}
func (f *fakeBackend) DeleteQueue(context.Context, string, bool) (topology.DeleteResult, error) {
	f.deleteCalls++
	return f.deleteResult, f.err
}
func (f *fakeBackend) DeleteQueueConditional(context.Context, string, bool, jetstream.ApplyPrecondition) (topology.DeleteResult, error) {
	f.deleteCalls++
	return f.deleteResult, f.err
}
func (f *fakeBackend) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	return f.declarations, f.err
}
func (f *fakeBackend) Declaration(_ context.Context, name string) (*topology.Declaration, error) {
	for index := range f.declarations {
		if f.declarations[index].Queue == name {
			return &f.declarations[index], f.err
		}
	}
	return nil, jetstream.ErrNotFound
}
func (f *fakeBackend) RecordAudit(_ context.Context, event jetstream.AuditEvent) (uint64, error) {
	f.auditCalls++
	if f.auditFailAt > 0 && f.auditCalls == f.auditFailAt {
		return 0, errors.New("audit failure")
	}
	if f.auditErr != nil {
		return 0, f.auditErr
	}
	f.auditEvents = append(f.auditEvents, event)
	return uint64(len(f.auditEvents)), nil
}
func (f *fakeBackend) ListAudit(_ context.Context, offset, limit int) (jetstream.AuditPage, error) {
	if f.auditErr != nil {
		return jetstream.AuditPage{}, f.auditErr
	}
	items := append([]jetstream.AuditEvent(nil), f.auditEvents...)
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return jetstream.AuditPage{Items: items[offset:end], Total: len(items), Offset: offset, Limit: limit}, nil
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

func TestOpenAPIContractIsServed(t *testing.T) {
	recorder := httptest.NewRecorder()
	newTestHandler(&fakeBackend{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/yaml" || !strings.HasPrefix(recorder.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestNativeSDKContractIsServed(t *testing.T) {
	recorder := httptest.NewRecorder()
	newTestHandler(&fakeBackend{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/native-sdk-contract.json", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/json" || !strings.Contains(recorder.Body.String(), `"schema": "rabbit-jetstream.io/native-sdk-contract/v1alpha1"`) {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestManagementResponsesSetBrowserSecurityHeaders(t *testing.T) {
	handler := newTestHandler(&fakeBackend{})
	for _, test := range []struct {
		path  string
		cache string
	}{
		{path: "/api/v1/info", cache: "no-store"},
		{path: "/healthz", cache: "no-store"},
		{path: "/api/v1/openapi.yaml", cache: "public, max-age=300"},
		{path: "/api/v1/native-sdk-contract.json", cache: "public, max-age=300"},
		{path: "/admin/", cache: "no-cache"},
	} {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			for name, expected := range map[string]string{
				"Cross-Origin-Resource-Policy": "same-origin",
				"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
				"Referrer-Policy":              "no-referrer",
				"X-Content-Type-Options":       "nosniff",
				"X-Frame-Options":              "DENY",
			} {
				if actual := recorder.Header().Get(name); actual != expected {
					t.Errorf("%s=%q, want %q", name, actual, expected)
				}
			}
			if actual := recorder.Header().Get("Cache-Control"); actual != test.cache {
				t.Errorf("Cache-Control=%q, want %q", actual, test.cache)
			}
			if csp := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
				t.Errorf("Content-Security-Policy=%q", csp)
			}
		})
	}
}

func TestReadyAndInfoEndpoints(t *testing.T) {
	backend := &fakeBackend{account: jetstream.Account{MemoryUsed: 10, StorageUsed: 20, Streams: 2, Consumers: 3}}
	h := newTestHandler(backend)
	for _, test := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/readyz", http.StatusOK, `"status":"ready"`},
		{"/api/v1/info", http.StatusOK, `"storage_used":20`},
	} {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.contains) {
			t.Fatalf("%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
	backend.err = errors.New("connect to nats://operator:super-secret@nats-1:4222: offline")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var readiness map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&readiness); err != nil {
		t.Fatal(err)
	}
	if len(readiness) != 1 || readiness["status"] != "not_ready" {
		t.Fatalf("readiness response exposes backend detail: %#v", readiness)
	}
}

func TestManagementResponsesRedactNATSCredentials(t *testing.T) {
	backend := &fakeBackend{serverURL: "nats://operator:super-secret@nats-1:4222"}
	handler := newTestHandler(backend)
	for _, path := range []string{"/api/v1/info", "/api/v1/cluster"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "super-secret") || strings.Contains(recorder.Body.String(), "operator") || !strings.Contains(recorder.Body.String(), "nats://nats-1:4222") {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestBackendFailuresDoNotExposeDependencyDetails(t *testing.T) {
	secret := "nats://operator:super-secret@nats-1:4222"
	backend := &fakeBackend{err: errors.New("connect to " + secret + ": unavailable"), auditErr: errors.New("read " + secret + ": unavailable")}
	var logs bytes.Buffer
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(&logs, nil)), "test", "dev", nil, nil,
		AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}})
	for _, path := range []string{"/readyz", "/api/v1/info", "/api/v1/cluster", "/api/v1/streams", "/api/v1/audit"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer operator")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "super-secret") || strings.Contains(recorder.Body.String(), "operator") || strings.Contains(recorder.Body.String(), "nats-1") {
				t.Fatalf("dependency detail exposed: %s", recorder.Body.String())
			}
		})
	}
	if strings.Contains(logs.String(), "super-secret") || strings.Contains(logs.String(), "nats-1") {
		t.Fatalf("dependency detail exposed in logs: %s", logs.String())
	}
}

func TestBackendPublicMessagesPreserveStableKnownSemantics(t *testing.T) {
	for code, expected := range map[string]string{
		"dlq_dependency_cycle":  "DLQ dependency cycle",
		"conflict":              "resource conflict",
		"not_found":             "resource not found",
		"jetstream_unavailable": "JetStream is unavailable",
	} {
		if actual := backendPublicMessage(code); actual != expected {
			t.Fatalf("%s message=%q, want %q", code, actual, expected)
		}
	}
}

func TestAdminUIEndpoints(t *testing.T) {
	h := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	for _, test := range []struct {
		path, location, content string
		status                  int
	}{
		{"/", "/admin/", "", http.StatusTemporaryRedirect},
		{"/admin", "/admin/", "", http.StatusPermanentRedirect},
		{"/admin/", "", "Rabbit JetStream", http.StatusOK},
		{"/admin/queues/example", "", "Rabbit JetStream", http.StatusOK},
	} {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status || (test.location != "" && recorder.Header().Get("Location") != test.location) || (test.content != "" && !strings.Contains(recorder.Body.String(), test.content)) {
			t.Fatalf("%s: status=%d location=%q body=%q", test.path, recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
		}
	}
}

func TestControllerStatusEndpoint(t *testing.T) {
	h := NewWithController(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, fakeController{status: controller.Status{InstanceID: "one", Leader: true}}, "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/controller", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"instanceId":"one"`) || !strings.Contains(rec.Body.String(), `"leader":true`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadOnlyManagementEndpoints(t *testing.T) {
	backend := &fakeBackend{
		account:      jetstream.Account{Streams: 2, Consumers: 1, APILevel: 4},
		streams:      []jetstream.Stream{{Name: "alpha", Messages: 10}, {Name: "beta", Messages: 20}},
		consumers:    []jetstream.Consumer{{Stream: "alpha", Name: "worker", Pending: 3}},
		declarations: []topology.Declaration{{APIVersion: topology.DeclarationAPIVersion, Queue: "orders", Revision: "abc"}},
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
		{"/api/v1/queues", []string{`"queue":"orders"`, `"revision":"abc"`}},
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
			if test.name == "accepted" {
				req.Header.Set("If-None-Match", "*")
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

func TestApplyQueueRequiresConditionalHeader(t *testing.T) {
	handler := New(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	body := `{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestApplyPreconditionParsing(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/", nil)
	request.Header.Set("If-None-Match", "*")
	condition, err := applyPrecondition(request)
	if err != nil || !condition.CreateOnly {
		t.Fatalf("condition=%#v err=%v", condition, err)
	}
	request = httptest.NewRequest(http.MethodPut, "/", nil)
	request.Header.Set("If-Match", `"42"`)
	condition, err = applyPrecondition(request)
	if err != nil || condition.ExpectedRevision == nil || *condition.ExpectedRevision != 42 {
		t.Fatalf("condition=%#v err=%v", condition, err)
	}
}

func TestQueueDetailReturnsKVRevisionETag(t *testing.T) {
	backend := &fakeBackend{declarations: []topology.Declaration{{Queue: "orders", KVRevision: 17}}}
	rec := httptest.NewRecorder()
	newTestHandler(backend).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/queues/orders", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") != `"17"` {
		t.Fatalf("status=%d etag=%q", rec.Code, rec.Header().Get("ETag"))
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
	req.Header.Set("If-None-Match", "*")
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
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestApplyWritesCorrelatedAuditIntentAndOutcome(t *testing.T) {
	backend := &fakeBackend{applyResult: topology.ReconcileResult{Queue: "orders", Revision: "revision-2", Status: "ready"}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("X-Request-ID", "operator-request-42")
	req.RemoteAddr = "192.0.2.10:4321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || backend.applyCalls != 1 || len(backend.auditEvents) != 2 {
		t.Fatalf("status=%d applies=%d audit=%#v", rec.Code, backend.applyCalls, backend.auditEvents)
	}
	intent, outcome := backend.auditEvents[0], backend.auditEvents[1]
	if intent.Phase != "intent" || intent.Outcome != "attempted" || intent.RequestID != "operator-request-42" || intent.ActorRole != "operator" || intent.SourceIP != "192.0.2.10" || intent.Revision != "create" {
		t.Fatalf("intent=%#v", intent)
	}
	if outcome.Phase != "outcome" || outcome.IntentID != intent.ID || outcome.Outcome != "succeeded" || outcome.Revision != "revision-2" || rec.Header().Get("X-Request-ID") != intent.RequestID {
		t.Fatalf("outcome=%#v headers=%v", outcome, rec.Header())
	}
	encoded, _ := json.Marshal(backend.auditEvents)
	if strings.Contains(string(encoded), `"secret"`) || !strings.HasPrefix(intent.Actor, "token-sha256:") {
		t.Fatalf("audit leaked token or actor missing: %s", encoded)
	}
}

func TestAuditIntentFailureRejectsMutation(t *testing.T) {
	backend := &fakeBackend{auditErr: errors.New("write nats://operator:super-secret@nats-1:4222: unavailable")}
	var logs bytes.Buffer
	handler := New(backend, slog.New(slog.NewTextHandler(&logs, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queues/orders", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-RJS-Confirm-Queue", "orders")
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || backend.deleteCalls != 0 || !strings.Contains(rec.Body.String(), "audit_unavailable") {
		t.Fatalf("status=%d deletes=%d body=%s", rec.Code, backend.deleteCalls, rec.Body.String())
	}
	if strings.Contains(logs.String(), "super-secret") || strings.Contains(logs.String(), "nats-1") || !strings.Contains(logs.String(), "error_kind=audit_unavailable") {
		t.Fatalf("unsafe or unclassified audit log: %s", logs.String())
	}
}

func TestAuditOutcomeFailureReportsUncertainMutation(t *testing.T) {
	backend := &fakeBackend{auditFailAt: 2, applyResult: topology.ReconcileResult{Queue: "orders", Status: "ready"}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || backend.applyCalls != 1 || len(backend.auditEvents) != 1 || !strings.Contains(rec.Body.String(), "inspect resource state") {
		t.Fatalf("status=%d applies=%d audits=%d body=%s", rec.Code, backend.applyCalls, len(backend.auditEvents), rec.Body.String())
	}
}

func TestAuditListRequiresAdminTokenAndPaginates(t *testing.T) {
	backend := &fakeBackend{auditEvents: []jetstream.AuditEvent{{ID: "one"}, {ID: "two"}}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, "secret")
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?offset=1&limit=1", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"one"`) || !strings.Contains(rec.Body.String(), `"total":2`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRoleTokensSupportRotationAndLeastPrivilege(t *testing.T) {
	backend := &fakeBackend{auditEvents: []jetstream.AuditEvent{{ID: "one"}}, applyResult: topology.ReconcileResult{Queue: "orders", Status: "ready"}}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{
		OperatorTokens: []string{"old-operator", "new-operator"},
		AuditorTokens:  []string{"audit-reader"},
	})
	for _, token := range []string{"old-operator", "new-operator", "audit-reader"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("token=%s audit status=%d body=%s", token, rec.Code, rec.Body.String())
		}
	}
	for _, token := range []string{"old-operator", "new-operator"} {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("If-None-Match", "*")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("token=%s write status=%d body=%s", token, rec.Code, rec.Body.String())
		}
	}
	auditorWrite := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{}`))
	auditorWrite.Header.Set("Authorization", "Bearer audit-reader")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, auditorWrite)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("auditor write status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type fakeIdentityVerifier struct {
	principal identity.Principal
	err       error
}

func (v fakeIdentityVerifier) Verify(context.Context, string) (identity.Principal, error) {
	return v.principal, v.err
}

func TestFederatedIdentityAuthorizesAndAttributesAudit(t *testing.T) {
	backend := &fakeBackend{applyResult: topology.ReconcileResult{Queue: "orders", Status: "ready"}}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OIDC: fakeIdentityVerifier{principal: identity.Principal{Actor: "oidc:https://idp.example#alice", Role: "operator"}}})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.>"],"replicas":1}}`))
	req.Header.Set("Authorization", "Bearer signed.jwt")
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || len(backend.auditEvents) != 2 || backend.auditEvents[0].Actor != "oidc:https://idp.example#alice" || backend.auditEvents[0].ActorRole != "operator" {
		t.Fatalf("status=%d audit=%+v body=%s", rec.Code, backend.auditEvents, rec.Body.String())
	}
}

func TestFederatedAuditorCannotWriteAndInvalidTokenIsUnauthorized(t *testing.T) {
	for _, test := range []struct {
		verifier identity.Verifier
		want     int
	}{
		{fakeIdentityVerifier{principal: identity.Principal{Actor: "auditor", Role: "auditor"}}, http.StatusForbidden},
		{fakeIdentityVerifier{err: errors.New("bad token")}, http.StatusUnauthorized},
	} {
		handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OIDC: test.verifier})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/queues/orders", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer credential")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != test.want {
			t.Fatalf("status=%d want=%d body=%s", rec.Code, test.want, rec.Body.String())
		}
	}
}

func TestDisabledAuditAPIIsHidden(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(&fakeBackend{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "audit_api_disabled") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMatchesTokenChecksAllConfiguredCredentials(t *testing.T) {
	if !matchesToken("second", []string{"first", "second"}) || matchesToken("missing", []string{"first", "second"}) || len(tokenList("")) != 0 || len(cleanTokens([]string{"", "valid"})) != 1 {
		t.Fatal("token matching failed")
	}
}

func TestAuditHelpersRejectUnsafeRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "bad request id\n")
	if got := auditRequestID(req); got == "bad request id\n" || len(got) != 32 {
		t.Fatalf("request id=%q", got)
	}
	if got := remoteIP("not-a-socket"); got != "not-a-socket" {
		t.Fatalf("ip=%q", got)
	}
}

func newTestHandler(backend Backend) http.Handler {
	monitor := fakeMonitor{snapshot: monitoring.Snapshot{
		Status: "available", Total: 1, Available: 1,
		Nodes: []monitoring.Node{{Name: "nats-1", Status: "available"}},
	}}
	return New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", monitor)
}
