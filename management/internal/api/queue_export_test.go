package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestQueueExportLimitAndCancellation(t *testing.T) {
	q := topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind, Metadata: topology.Metadata{Name: "orders"}, Spec: topology.QueueSpec{Replicas: 1, Subjects: []string{strings.Repeat("a", 1<<20)}}}
	plan, err := topology.BuildPlan(q)
	if err != nil {
		t.Fatal(err)
	}
	f := &routingProbeBackend{value: &topology.Declaration{Queue: "orders", Revision: plan.Revision, Plan: plan}}
	h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/export", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "export_limit") || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("partial export: %d", w.Code)
	}
	f.wait = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/export", nil).WithContext(ctx))
	if w.Code < 500 || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("canceled export: %d", w.Code)
	}
}

func TestQueueExportRoundTripAndLabelPolicy(t *testing.T) {
	queue, err := topology.ParseQueue(strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders","labels":{"owner":"private-owner"}},"spec":{"replicas":1,"subjects":["orders.created"],"retention":{"maxMessages":9223372036854775807},"maxPriority":0,"deadLetter":{"queue":"failed"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := topology.BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		f := &routingProbeBackend{value: &topology.Declaration{Queue: "orders", Revision: plan.Revision, Plan: plan, KVRevision: 9007199254740993}}
		before, _ := json.Marshal(f.value)
		h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		url := "/api/v1/queues/orders/export"
		if include {
			url += "?include_labels=true"
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		if w.Code != 200 || w.Header().Get("ETag") != `"9007199254740993"` || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Disposition") != `attachment; filename="orders.queue.json"` {
			t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body.String())
		}
		got, err := topology.ParseQueue(strings.NewReader(w.Body.String()))
		if err != nil {
			t.Fatal(err)
		}
		if got.Spec.Retention.MaxMessages != 9223372036854775807 || got.Spec.MaxPriority == nil || *got.Spec.MaxPriority != 0 || got.Spec.DeadLetter.Queue != "failed" {
			t.Fatal("lossy export")
		}
		if include {
			newPlan, err := topology.BuildPlan(*got)
			if err != nil || newPlan.Revision != plan.Revision || got.Metadata.Labels["owner"] != "private-owner" || w.Header().Get("X-RJS-Export-Omitted-Labels") != "0" {
				t.Fatal("full declaration did not round-trip")
			}
		} else if len(got.Metadata.Labels) != 0 || strings.Contains(w.Body.String(), "private-owner") || w.Header().Get("X-RJS-Export-Omitted-Labels") != "1" {
			t.Fatal("default label redaction failed")
		}
		after, _ := json.Marshal(f.value)
		if string(before) != string(after) || f.applyCalls != 0 || f.deleteCalls != 0 || f.auditCalls != 0 {
			t.Fatal("export mutated declaration")
		}
	}
}

func TestQueueExportAuthorizationAndFailures(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		for _, token := range []string{"", "bad", "auditor", "operator"} {
			f := &routingProbeBackend{value: routingDeclaration(t)}
			h := NewWithControllerAuth(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, AuditorTokens: []string{"auditor"}, OperatorTokens: []string{"operator"}})
			r := httptest.NewRequest(method, "/api/v1/queues/orders/export", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if token == "" || token == "bad" {
				if w.Code != 401 || f.reads != 0 {
					t.Fatal("unauthorized export")
				}
			} else if w.Code != 200 || f.reads != 1 {
				t.Fatalf("authorized export %d", w.Code)
			}
		}
	}
	for _, query := range []string{"include_labels=", "include_labels=1", "include_labels=true&include_labels=false", "unknown=true", "include_labels=%ZZ"} {
		f := &routingProbeBackend{value: routingDeclaration(t)}
		h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/export?"+query, nil))
		if w.Code != 400 || f.reads != 0 {
			t.Fatalf("accepted query %s: %d", query, w.Code)
		}
	}
	for _, scenario := range []string{"missing", "revision", "legacy"} {
		f := &routingProbeBackend{value: routingDeclaration(t)}
		want := 409
		if scenario == "missing" {
			f.value = nil
			want = 404
		} else if scenario == "revision" {
			f.value.Revision = "bad"
		} else {
			f.value.Plan.Stream.Subjects = []string{"unknown"}
		}
		h := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/queues/orders/export", nil))
		if w.Code != want || w.Header().Get("Content-Disposition") != "" {
			t.Fatalf("false export %s: %d", scenario, w.Code)
		}
	}
}
