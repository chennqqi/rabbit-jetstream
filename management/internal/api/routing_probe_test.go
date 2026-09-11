package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type routingProbeBackend struct {
	fakeBackend
	value *topology.Declaration
	reads int
	wait  bool
}

func (f *routingProbeBackend) Declaration(ctx context.Context, name string) (*topology.Declaration, error) {
	f.reads++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 3*time.Second {
		panic("unbounded declaration read")
	}
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.value == nil {
		return nil, jetstream.ErrNotFound
	}
	return f.value, nil
}

func routingDeclaration(t *testing.T) *topology.Declaration {
	t.Helper()
	queue := topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind,
		Metadata: topology.Metadata{Name: "orders"}, Spec: topology.QueueSpec{Replicas: 1,
			Bindings: []topology.Binding{{Exchange: "events", Type: "topic", Keys: []string{"orders.#"}}}}}
	plan, err := topology.BuildPlan(queue)
	if err != nil {
		t.Fatal(err)
	}
	return &topology.Declaration{Queue: "orders", Revision: plan.Revision, Plan: plan, KVRevision: 9007199254740993}
}

func TestRoutingProbeAPIAuthAndEvidence(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		for _, token := range []string{"", "bad", "auditor", "operator"} {
			f := &routingProbeBackend{value: routingDeclaration(t)}
			h := NewWithControllerAuth(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil,
				AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
			r := httptest.NewRequest(method, "/api/v1/queues/orders/routing-probe?exchange=events&type=topic&routingKey=orders", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if token == "" || token == "bad" {
				if w.Code != 401 || f.reads != 0 {
					t.Fatalf("unauthorized read: %d %d", w.Code, f.reads)
				}
				continue
			}
			if w.Code != 200 || f.reads != 1 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != `"9007199254740993"` {
				t.Fatalf("status=%d reads=%d headers=%v body=%s", w.Code, f.reads, w.Header(), w.Body.String())
			}
			if method == "GET" {
				var result topology.RoutingProbeResult
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Revision != f.value.Revision || result.Queue != "orders" || len(result.Bindings) != 1 || len(result.Bindings[0].MatchedSubjects) != 1 || len(result.MatchedStreamSubjects) != 1 {
					t.Fatalf("bad evidence: %+v", result)
				}
			}
			if f.applyCalls != 0 || f.deleteCalls != 0 || f.auditCalls != 0 {
				t.Fatal("read performed mutation")
			}
		}
	}
}

func TestRoutingProbeAPIValidationAndDeclarationFailures(t *testing.T) {
	for _, tc := range []struct {
		query         string
		status, reads int
		code          string
	}{
		{"", 400, 0, "invalid_query"}, {"subject=", 400, 0, "invalid_query"},
		{"subject=a&exchange=", 400, 0, "invalid_query"}, {"subject=a&subject=b", 400, 0, "invalid_query"},
		{"subject=a&unknown=b", 400, 0, "invalid_query"}, {"subject=%ZZ", 400, 0, "invalid_query"},
		{"subject=" + strings.Repeat("a", 1025), 400, 0, "invalid_query"},
		{"subject=" + strings.Repeat("a", 4097), 400, 0, "invalid_query"},
		{"subject=a.*", 400, 1, "invalid_query"}, {"exchange=events&type=headers", 400, 1, "invalid_query"},
		{"subject=unmatched", 200, 1, `"matchedStreamSubjects":[]`},
	} {
		f := &routingProbeBackend{value: routingDeclaration(t)}
		h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/routing-probe?"+tc.query, nil))
		if w.Code != tc.status || f.reads != tc.reads || !strings.Contains(w.Body.String(), tc.code) {
			t.Fatalf("query=%q status=%d reads=%d body=%s", tc.query, w.Code, f.reads, w.Body.String())
		}
	}
	for _, scenario := range []string{"missing", "wrong queue", "wrong plan", "revision", "unrepresentable", "disabled"} {
		f := &routingProbeBackend{value: routingDeclaration(t)}
		status, code := 409, "routing_declaration_unavailable"
		auth := AuthConfig{}
		switch scenario {
		case "missing":
			f.value = nil
			status, code = 404, "not_found"
		case "wrong queue":
			f.value.Queue = "other"
		case "wrong plan":
			f.value.Plan.Queue = "other"
		case "revision":
			f.value.Revision = "stale"
		case "unrepresentable":
			f.value.Plan.Stream.Subjects = []string{"unrecognized"}
		case "disabled":
			auth.RequireReadAuth = true
			status, code = 404, "read_api_disabled"
		}
		h := NewWithControllerAuth(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, auth)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/routing-probe?subject=orders", nil))
		if w.Code != status || !strings.Contains(w.Body.String(), code) {
			t.Fatalf("%s: %d %s", scenario, w.Code, w.Body.String())
		}
		if scenario == "disabled" && f.reads != 0 {
			t.Fatal("disabled read reached backend")
		}
	}
}

func TestRoutingProbeAPICancellation(t *testing.T) {
	f := &routingProbeBackend{wait: true}
	h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/routing-probe?subject=orders", nil).WithContext(ctx))
	if w.Code < 500 || strings.Contains(w.Body.String(), "matchedStreamSubjects") {
		t.Fatalf("partial evidence: %d %s", w.Code, w.Body.String())
	}
}

func TestRoutingProbeAPIResponseLimit(t *testing.T) {
	queue := topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind,
		Metadata: topology.Metadata{Name: "orders"}, Spec: topology.QueueSpec{Replicas: 1,
			Bindings: []topology.Binding{{Exchange: "events", Type: "direct", Keys: []string{strings.Repeat("a", 400000)}}}}}
	plan, err := topology.BuildPlan(queue)
	if err != nil {
		t.Fatal(err)
	}
	f := &routingProbeBackend{value: &topology.Declaration{Queue: "orders", Revision: plan.Revision, Plan: plan}}
	h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/routing-probe?subject=none", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "routing_probe_limit") || w.Header().Get("ETag") != "" || strings.Contains(w.Body.String(), "streamSubjects") {
		t.Fatalf("truncated success: %d %s", w.Code, w.Body.String())
	}
}
