package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type queueCollectionBackend struct {
	*fakeBackend
	items []jetstream.QueueConsumer
	calls int
}

func (b *queueCollectionBackend) QueueConsumers(ctx context.Context, name string) (*jetstream.QueueConsumerCollection, error) {
	b.calls++
	if name != "orders" {
		panic("wrong Queue")
	}
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded query")
	}
	return &jetstream.QueueConsumerCollection{Queue: name, Stream: "RJSQ_orders", DeclarationRevision: "\"9\"", StreamStatus: "present", StreamOwnership: "matching", Items: b.items}, b.err
}

func TestQueueConsumerQueries(t *testing.T) {
	for _, tc := range []struct {
		query                         string
		status, total, length, offset int
		first                         string
	}{
		{"", 200, 202, 50, 0, "worker-000"},
		{"?q=WORKER-200", 200, 1, 1, 0, "worker-200"},
		{"?q=%20last.subject%20&mode=push", 200, 1, 1, 0, "worker-200"},
		{"?q=expected.subject&mode=pull", 200, 1, 1, 0, "worker-missing"},
		{"?q=worker-200&mode=pull", 200, 0, 0, 0, ""},
		{"?offset=200&limit=1", 200, 202, 1, 200, "worker-200"},
		{"?order=desc&limit=1", 200, 202, 1, 0, "worker-missing"},
		{"?q=unknown&offset=500", 200, 0, 0, 0, ""},
		{"?q=worker&offset=999999999", 200, 202, 0, 202, ""},
		{"?mode=bad", 400, 0, 0, 0, ""},
		{"?order=bad", 400, 0, 0, 0, ""},
		{"?unknown=value", 400, 0, 0, 0, ""},
		{"?q=one&q=two", 400, 0, 0, 0, ""},
		{"?limit=201", 400, 0, 0, 0, ""},
		{"?offset=-1", 400, 0, 0, 0, ""},
		{"?q=%zz", 400, 0, 0, 0, ""},
	} {
		t.Run(tc.query, func(t *testing.T) {
			backend := &queueCollectionBackend{fakeBackend: &fakeBackend{}}
			for i := 200; i >= 0; i-- {
				name := fmt.Sprintf("worker-%03d", i)
				observed := &jetstream.Consumer{Stream: "RJSQ_orders", Name: name, Mode: "pull"}
				if i == 200 {
					observed.Mode = "push"
					observed.FilterSubject = "last.subject"
				}
				backend.items = append(backend.items, jetstream.QueueConsumer{Stream: observed.Stream, Name: name, Observed: observed, Status: "present", Ownership: "unmarked"})
			}
			backend.items = append(backend.items, jetstream.QueueConsumer{Stream: "RJSQ_orders", Name: "worker-missing", Status: "missing", Ownership: "unknown", Expected: &topology.ConsumerPlan{Name: "worker-missing", Mode: "pull", FilterSubjects: []string{"expected.subject"}}})
			handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/queues/orders/consumers"+tc.query, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if tc.status == 400 {
				if backend.calls != 0 {
					t.Fatal("invalid query called backend")
				}
				return
			}
			var got struct {
				Items         []jetstream.QueueConsumer `json:"items"`
				Total, Offset int
				Revision      string `json:"declaration_revision"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Total != tc.total || got.Offset != tc.offset || len(got.Items) != tc.length || got.Revision != "\"9\"" {
				t.Fatalf("page=%+v", got)
			}
			if tc.first != "" && got.Items[0].Name != tc.first {
				t.Fatalf("first=%s", got.Items[0].Name)
			}
			if got.Items == nil {
				t.Fatal("empty list must be []")
			}
			if tc.first == "worker-missing" && got.Items[0].Observed != nil {
				t.Fatal("invented missing metrics")
			}
			if recorder.Header().Get("Cache-Control") != "no-store" || backend.calls != 1 {
				t.Fatal("read contract violation")
			}
		})
	}
}

func TestQueueConsumerBackendErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{jetstream.ErrNotFound, 404}, {jetstream.ErrConflict, 409}, {context.DeadlineExceeded, 503}} {
		backend := &queueCollectionBackend{fakeBackend: &fakeBackend{err: tc.err}}
		handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/queues/orders/consumers", nil))
		if recorder.Code != tc.status {
			t.Fatalf("status = %d", recorder.Code)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["error"] == nil || body["items"] != nil {
			t.Fatal("error hidden as list")
		}
	}
}
