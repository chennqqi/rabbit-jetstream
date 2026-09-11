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

type listQueryBackend struct {
	*fakeBackend
	calls int
}

func (b *listQueryBackend) ListDeclarations(ctx context.Context) ([]topology.Declaration, error) {
	b.calls++
	return b.fakeBackend.ListDeclarations(ctx)
}
func (b *listQueryBackend) ListStreams(ctx context.Context) ([]jetstream.Stream, error) {
	b.calls++
	return b.fakeBackend.ListStreams(ctx)
}

func TestGlobalListQueries(t *testing.T) {
	for _, route := range []string{"queues", "streams"} {
		for _, tc := range []struct {
			query                         string
			status, total, length, offset int
			first                         string
		}{
			{"", 200, 201, 50, 0, "resource-000"},
			{"?q=%20RESOURCE-200%20", 200, 1, 1, 0, "resource-200"},
			{"?offset=200&limit=1", 200, 201, 1, 200, "resource-200"},
			{"?q=resource-1&sort=name&order=desc&limit=1", 200, 100, 1, 0, "resource-199"},
			{"?q=unknown&offset=200", 200, 0, 0, 0, ""},
			{"?offset=99999999", 200, 201, 0, 201, ""},
			{"?sort=health", 400, 0, 0, 0, ""},
			{"?order=wrong", 400, 0, 0, 0, ""},
			{"?q=a&q=b", 400, 0, 0, 0, ""},
			{"?unknown=x", 400, 0, 0, 0, ""},
			{"?q=%zz", 400, 0, 0, 0, ""},
			{"?limit=201", 400, 0, 0, 0, ""},
		} {
			t.Run(route+tc.query, func(t *testing.T) {
				backend := &listQueryBackend{fakeBackend: &fakeBackend{}}
				for i := 200; i >= 0; i-- {
					name := fmt.Sprintf("resource-%03d", i)
					backend.streams = append(backend.streams, jetstream.Stream{Name: name})
					backend.declarations = append(backend.declarations, topology.Declaration{Queue: name})
				}
				handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/"+route+tc.query, nil))
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
					Items         []struct{ Name, Queue string }
					Total, Offset int
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Total != tc.total || got.Offset != tc.offset || len(got.Items) != tc.length || got.Items == nil {
					t.Fatalf("page=%+v", got)
				}
				if len(got.Items) > 0 && got.Items[0].Name != tc.first && got.Items[0].Queue != tc.first {
					t.Fatalf("first=%+v", got.Items[0])
				}
				if backend.streams[0].Name != "resource-200" || backend.declarations[0].Queue != "resource-200" {
					t.Fatal("query reordered backend-owned slice")
				}
				wantCalls := 1
				if route == "queues" {
					wantCalls = 2 // One declaration enumeration plus one Stream enumeration; never per row.
				}
				if backend.calls != wantCalls {
					t.Fatalf("backend calls=%d want=%d", backend.calls, wantCalls)
				}
			})
		}
	}
}

func TestGlobalListFailuresDoNotBecomeEmpty(t *testing.T) {
	for _, route := range []string{"queues", "streams"} {
		backend := &listQueryBackend{fakeBackend: &fakeBackend{err: context.DeadlineExceeded}}
		handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/"+route+"?q=unknown", nil))
		if recorder.Code != 503 {
			t.Fatalf("%s status=%d", route, recorder.Code)
		}
	}
}
