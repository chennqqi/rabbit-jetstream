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

func TestConnectionDetailAPIRealMonitoringAndDisabledAuth(t *testing.T) {
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.URL.Path == "/varz" {
			fmt.Fprint(w, `{"server_id":"node"}`)
			return
		}
		if r.URL.Path != "/connz" || r.URL.Query().Get("auth") != "false" || r.URL.Query().Get("subs") != "false" {
			t.Error("unexpected monitoring query")
		}
		if r.URL.Query().Get("cid") == "7" {
			fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","offset":0,"limit":1,"total":1,"num_connections":1,"connections":[{"cid":7,"jwt":"secret"}]}`)
		} else {
			fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","offset":0,"limit":1,"total":0,"num_connections":0,"connections":[]}`)
		}
	}))
	defer server.Close()
	for _, configured := range []bool{false, true} {
		auth := AuthConfig{RequireReadAuth: true}
		if configured {
			auth.AuditorTokens = []string{"auditor"}
		}
		h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", monitoring.New(server.URL, time.Second), nil, auth)
		before := reads.Load()
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/nodes/node/connections/7", nil))
			want := 404
			if configured {
				want = 401
			}
			if w.Code != want || reads.Load() != before {
				t.Fatal("unauthorized monitoring I/O")
			}
		}
		if !configured {
			continue
		}
		for _, tc := range []struct {
			path   string
			status int
			code   string
		}{{"node/connections/7", 200, `"cid":7`}, {"node/connections/8", 404, "connection_not_found"}, {"other/connections/7", 404, `"code":"not_found"`}} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/api/v1/nodes/"+tc.path, nil)
			r.Header.Set("Authorization", "Bearer auditor")
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("real detail: %d %s", w.Code, w.Body.String())
			}
		}
	}
}

func (f *connectionFake) NodeConnection(ctx context.Context, node string, cid uint64) (*monitoring.ConnectionDetail, error) {
	f.calls++
	f.node = node
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
		panic("missing bounded deadline")
	}
	if f.err != nil {
		return nil, f.err
	}
	return &monitoring.ConnectionDetail{NodeID: node, ObservedAt: time.Now(), ReadAt: time.Now(), Item: monitoring.ConnectionSample{CID: cid}}, nil
}

func TestConnectionDetailAPIAuthAndCID(t *testing.T) {
	for _, token := range []string{"", "invalid", "auditor", "operator"} {
		for _, method := range []string{"GET", "HEAD"} {
			f := &connectionFake{}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "/api/v1/nodes/node/connections/18446744073709551615", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			connectionHandler(f, true).ServeHTTP(w, r)
			if token == "" || token == "invalid" {
				if w.Code != 401 || f.calls != 0 {
					t.Fatal("unauthorized detail read")
				}
				continue
			}
			if w.Code != 200 || f.calls != 1 || f.node != "node" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("detail failed: %d %s", w.Code, w.Body.String())
			}
			if method == "GET" && !strings.Contains(w.Body.String(), `"cid":18446744073709551615`) {
				t.Fatal("CID lost precision")
			}
		}
	}
}

func TestConnectionDetailAPIRejectsInvalidQueryBeforeIO(t *testing.T) {
	for _, suffix := range []string{"0", "-1", "+1", "1.5", "18446744073709551616", "1?offset=0", "1?subs=true", "1?q=x", "1?bad=%zz"} {
		f := &connectionFake{}
		w := httptest.NewRecorder()
		connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections/"+suffix, nil))
		if w.Code != 400 || f.calls != 0 {
			t.Fatalf("invalid detail accepted: %s %d", suffix, w.Code)
		}
	}
}

func TestConnectionDetailAPIErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{monitoring.ErrConnectionMissing, 404, "connection_not_found"}, {monitoring.ErrConnectionNodeMissing, 404, "not_found"}, {monitoring.ErrConnectionNodeAmbiguous, 409, "node_identity_ambiguous"}, {monitoring.ErrConnectionQuery, 400, "invalid_query"}, {monitoring.ErrConnectionEndpointLimit, 503, "connection_lookup_limit"}, {errors.New("secret-url"), 503, "connections_unavailable"},
	} {
		f := &connectionFake{err: tc.err}
		w := httptest.NewRecorder()
		connectionHandler(f, false).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/node/connections/7", nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("error mismatch: %d %s", w.Code, w.Body.String())
		}
	}
}
