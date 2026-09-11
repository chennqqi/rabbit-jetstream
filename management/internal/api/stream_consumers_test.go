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

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type streamConsumerBackend struct {
	*fakeBackend
	calls int
}

func (b *streamConsumerBackend) ListConsumers(ctx context.Context, name string) ([]jetstream.Consumer, error) {
	b.calls++
	return b.fakeBackend.ListConsumers(ctx, name)
}

func TestStreamConsumerQueries(t *testing.T) {
	for _, tc := range []struct {
		query                         string
		status, total, offset, length int
		first                         string
	}{
		{"", 200, 201, 0, 50, "c-000"},
		{"?q=%20C-200%20", 200, 1, 0, 1, "c-200"},
		{"?q=BILLING&mode=push&order=desc&limit=1&offset=1", 200, 100, 1, 1, "c-197"},
		{"?q=legacy", 200, 1, 0, 1, "c-200"},
		{"?q=ignored", 200, 0, 0, 0, ""},
		{"?mode=pull&order=desc&limit=1", 200, 101, 0, 1, "c-200"},
		{"?q=missing&offset=200", 200, 0, 0, 0, ""},
		{"?offset=999", 200, 201, 201, 0, ""},
		{"?mode=invalid", 400, 0, 0, 0, ""},
		{"?mode=pull&mode=push", 400, 0, 0, 0, ""},
		{"?q=a&q=b", 400, 0, 0, 0, ""},
		{"?unknown=x", 400, 0, 0, 0, ""},
		{"?sort=pending", 400, 0, 0, 0, ""},
		{"?order=invalid", 400, 0, 0, 0, ""},
		{"?q=%zz", 400, 0, 0, 0, ""},
		{"?q=" + strings.Repeat("a", 257), 400, 0, 0, 0, ""},
		{"?limit=201", 400, 0, 0, 0, ""},
	} {
		t.Run(tc.query, func(t *testing.T) {
			backend := &streamConsumerBackend{fakeBackend: &fakeBackend{}}
			for i := 200; i >= 0; i-- {
				mode := "pull"
				if i%2 == 1 {
					mode = "push"
				}
				row := jetstream.Consumer{Stream: "s", Name: fmt.Sprintf("c-%03d", i), Mode: mode, FilterSubjects: []string{"billing.events"}, FilterSubject: "ignored"}
				if i == 200 {
					row.FilterSubjects = nil
					row.FilterSubject = "legacy.events"
				}
				backend.consumers = append(backend.consumers, row)
			}
			handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/streams/s/consumers"+tc.query, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if tc.status == 400 {
				if backend.calls != 0 {
					t.Fatal("invalid query enumerated backend")
				}
				return
			}
			var got struct {
				Items         []jetstream.Consumer
				Total, Offset int
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Total != tc.total || got.Offset != tc.offset || len(got.Items) != tc.length || got.Items == nil {
				t.Fatalf("page=%+v", got)
			}
			if len(got.Items) > 0 && got.Items[0].Name != tc.first {
				t.Fatalf("first=%s", got.Items[0].Name)
			}
			if backend.consumers[0].Name != "c-200" {
				t.Fatal("backend slice reordered")
			}
		})
	}
}

func TestStreamConsumerQueryFailure(t *testing.T) {
	backend := &streamConsumerBackend{fakeBackend: &fakeBackend{err: context.DeadlineExceeded}}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/streams/s/consumers?q=missing", nil))
	if recorder.Code != 503 {
		t.Fatalf("status=%d", recorder.Code)
	}
}
