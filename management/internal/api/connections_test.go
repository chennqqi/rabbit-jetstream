package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type connectionFake struct {
	calls         int
	node          string
	offset, limit int
	cid           uint64
	err           error
	kind, value   string
}

func (f *connectionFake) NodeConnectionIdentityPage(ctx context.Context, node, kind, value string, offset, limit int) (*monitoring.ConnectionPage, error) {
	f.calls++
	f.node, f.kind, f.value, f.offset, f.limit = node, kind, value, offset, limit
	if f.err != nil {
		return nil, f.err
	}
	return &monitoring.ConnectionPage{NodeID: node, Offset: offset, Limit: limit, Total: 1, ObservedAt: time.Now(), ReadAt: time.Now(), Items: []monitoring.ConnectionSample{{CID: 7}}}, nil
}

func TestConnectionIdentitySearchUsesBodyAndReadAuthorization(t *testing.T) {
	body := `{"kind":"user","value":"private user","offset":0,"limit":25}`
	for _, token := range []string{"", "auditor"} {
		f := &connectionFake{}
		r := httptest.NewRequest("POST", "/api/v1/nodes/node/connections/search", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		connectionHandler(f, true).ServeHTTP(w, r)
		if token == "" && (w.Code != 401 || f.calls != 0) {
			t.Fatalf("unauthorized=%d calls=%d", w.Code, f.calls)
		}
		if token != "" && (w.Code != 200 || f.kind != "user" || f.value != "private user" || f.limit != 25 || strings.Contains(w.Body.String(), "private")) {
			t.Fatalf("search=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func TestConnectionIdentitySearchRejectsUnsafeRequests(t *testing.T) {
	for _, tc := range []struct {
		path, contentType, body string
		status                  int
	}{
		{"/api/v1/nodes/node/connections/search?q=x", "application/json", `{}`, 400},
		{"/api/v1/nodes/node/connections/search", "text/plain", `{}`, 415},
		{"/api/v1/nodes/node/connections/search", "application/json", `{"kind":"unknown","value":"x","offset":0,"limit":50}`, 400},
		{"/api/v1/nodes/node/connections/search", "application/json", `{"kind":"user","value":"x","offset":0,"limit":50,"extra":1}`, 400},
	} {
		f := &connectionFake{}
		r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.contentType)
		w := httptest.NewRecorder()
		connectionHandler(f, false).ServeHTTP(w, r)
		if w.Code != tc.status || f.calls != 0 {
			t.Fatalf("status=%d calls=%d body=%s", w.Code, f.calls, w.Body.String())
		}
	}
}

func (f *connectionFake) NodeConnectionCIDPage(ctx context.Context, node string, cid uint64, limit int) (*monitoring.ConnectionPage, error) {
	f.calls++
	f.node, f.cid, f.limit = node, cid, limit
	if _, ok := ctx.Deadline(); !ok {
		panic("missing deadline")
	}
	if f.err != nil {
		return nil, f.err
	}
	return &monitoring.ConnectionPage{NodeID: node, Offset: 0, Limit: limit, Total: 1, ObservedAt: time.Now(), ReadAt: time.Now(), Items: []monitoring.ConnectionSample{{CID: cid}}}, nil
}

func TestConnectionAPIDisabledAuthAndMissingMonitor(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		f := &connectionFake{}
		h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", f, nil, AuthConfig{RequireReadAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/nodes/node/connections", nil))
		if w.Code != 404 || f.calls != 0 || !strings.Contains(w.Body.String(), "read_api_disabled") {
			t.Fatalf("disabled auth: %d %s", w.Code, w.Body.String())
		}
	}
	h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, AuditorTokens: []string{"auditor"}})
	r := httptest.NewRequest("GET", "/api/v1/nodes/node/connections", nil)
	r.Header.Set("Authorization", "Bearer auditor")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "connections_unavailable") {
		t.Fatalf("missing monitor: %d %s", w.Code, w.Body.String())
	}
}

func TestConnectionAPIWithMonitoringTransport(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/varz":
			fmt.Fprint(w, `{"server_id":"node"}`)
		case "/connz":
			fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","offset":0,"limit":50,"total":1,"num_connections":1,"connections":[{"cid":7,"jwt":"do-not-expose","name":"do-not-expose"}]}`)
		default:
			t.Errorf("unexpected monitoring path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", monitoring.New(upstream.URL, time.Second), nil, AuthConfig{RequireReadAuth: true, AuditorTokens: []string{"auditor"}})
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/nodes/node/connections", nil))
		if w.Code != 401 || calls.Load() != 0 {
			t.Fatal("unauthorized monitoring access")
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/nodes/node/connections", nil)
	r.Header.Set("Authorization", "Bearer auditor")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || calls.Load() != 2 || strings.Contains(w.Body.String(), "do-not-expose") || !strings.Contains(w.Body.String(), `"cid":7`) {
		t.Fatalf("transport integration failed: %d %s", w.Code, w.Body.String())
	}
}

func (f *connectionFake) Nodes(context.Context) monitoring.Snapshot { return monitoring.Snapshot{} }
func (f *connectionFake) NodeConnections(ctx context.Context, node string, offset, limit int) (*monitoring.ConnectionPage, error) {
	f.calls++
	f.node, f.offset, f.limit = node, offset, limit
	if _, ok := ctx.Deadline(); !ok {
		panic("missing deadline")
	}
	if f.err != nil {
		return nil, f.err
	}
	return &monitoring.ConnectionPage{NodeID: node, Offset: offset, Limit: limit, Total: 201, ObservedAt: time.Now(), ReadAt: time.Now(), Items: []monitoring.ConnectionSample{{CID: ^uint64(0)}}}, nil
}
func connectionHandler(f *connectionFake, protect bool) http.Handler {
	return NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", f, nil, AuthConfig{RequireReadAuth: protect, OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
}
func TestConnectionAPIAuthAndExactPage(t *testing.T) {
	for _, token := range []string{"", "invalid", "auditor", "operator"} {
		f := &connectionFake{}
		r := httptest.NewRequest("GET", "/api/v1/nodes/node/connections?offset=100&limit=25", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		connectionHandler(f, true).ServeHTTP(w, r)
		if token == "" || token == "invalid" {
			if w.Code != 401 || f.calls != 0 {
				t.Fatalf("unauthorized request reached monitor: %d", w.Code)
			}
			continue
		}
		if w.Code != 200 || f.calls != 1 || f.node != "node" || f.offset != 100 || f.limit != 25 || !strings.Contains(w.Body.String(), "18446744073709551615") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("bad page: %d %s", w.Code, w.Body.String())
		}
	}
	f := &connectionFake{}
	w := httptest.NewRecorder()
	connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections", nil))
	if w.Code != 200 || f.limit != 50 || f.offset != 0 {
		t.Fatal("explicit demo policy/defaults changed")
	}
}
func TestConnectionAPIRejectsQueriesBeforeMonitor(t *testing.T) {
	for _, q := range []string{"q=x", "endpoint=http://host", "limit=1&limit=2", "offset=-1", "offset=+1", "offset=1.0", "offset=1000001", "limit=0", "limit=201", "limit=", "offset=9999999999999999999999", "offset=%ZZ", "cid=0", "cid=-1", "cid=18446744073709551616", "cid=7&offset=1"} {
		f := &connectionFake{}
		w := httptest.NewRecorder()
		connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections?"+q, nil))
		if w.Code != 400 || f.calls != 0 {
			t.Fatalf("accepted query %q: %d", q, w.Code)
		}
	}
}

func TestConnectionAPIExactCIDSearch(t *testing.T) {
	f := &connectionFake{}
	w := httptest.NewRecorder()
	connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections?cid=18446744073709551615&limit=25", nil))
	if w.Code != 200 || f.calls != 1 || f.cid != ^uint64(0) || f.limit != 25 || !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("search=%d calls=%d cid=%d body=%s", w.Code, f.calls, f.cid, w.Body.String())
	}
}
func TestConnectionAPIErrorsAreTypedAndSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{{monitoring.ErrConnectionQuery, 400, "invalid_query"}, {monitoring.ErrConnectionNodeMissing, 404, "not_found"}, {monitoring.ErrConnectionNodeAmbiguous, 409, "node_identity_ambiguous"}, {monitoring.ErrConnectionEndpointLimit, 503, "connection_lookup_limit"}, {errors.New("secret upstream URL"), 503, "connections_unavailable"}} {
		f := &connectionFake{err: tc.err}
		w := httptest.NewRecorder()
		connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections", nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("bad error: %d %s", w.Code, w.Body.String())
		}
	}
}
