package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

const (
	benchmarkGlobalStreams      = 10000
	benchmarkConsumersPerStream = 10
)

type benchmarkGlobalConsumerSource struct {
	streams      []jetstream.Stream
	declarations []topology.Declaration
}

func newBenchmarkGlobalConsumerSource() *benchmarkGlobalConsumerSource {
	source := &benchmarkGlobalConsumerSource{streams: make([]jetstream.Stream, benchmarkGlobalStreams), declarations: make([]topology.Declaration, benchmarkGlobalStreams)}
	for index := 0; index < benchmarkGlobalStreams; index++ {
		queue, stream := fmt.Sprintf("queue-%05d", index), fmt.Sprintf("stream-%05d", index)
		source.streams[index] = jetstream.Stream{Name: stream, Metadata: map[string]string{"rabbit-jetstream.io/queue": queue}}
		plans := make([]topology.ConsumerPlan, benchmarkConsumersPerStream)
		for member := range plans {
			plans[member] = topology.ConsumerPlan{Name: fmt.Sprintf("consumer-%02d", member), Stream: stream, Mode: "pull", DeliverPolicy: "all", AckPolicy: "explicit", AckWaitNanos: int64(30 * time.Second), MaxDeliver: 5, ReplayPolicy: "instant", FilterSubjects: []string{fmt.Sprintf("subject.%d", member)}}
		}
		source.declarations[index] = topology.Declaration{Queue: queue, KVRevision: uint64(index + 1), Plan: topology.Plan{Queue: queue, Stream: topology.StreamPlan{Name: stream}, Consumer: plans[0], PriorityConsumers: plans[1:]}}
	}
	return source
}

func (source *benchmarkGlobalConsumerSource) ListStreams(context.Context) ([]jetstream.Stream, error) {
	return source.streams, nil
}
func (source *benchmarkGlobalConsumerSource) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	return source.declarations, nil
}
func (source *benchmarkGlobalConsumerSource) ListConsumers(_ context.Context, stream string) ([]jetstream.Consumer, error) {
	items := make([]jetstream.Consumer, benchmarkConsumersPerStream)
	queue := "queue-" + strings.TrimPrefix(stream, "stream-")
	for index := range items {
		items[index] = jetstream.Consumer{Stream: stream, Name: fmt.Sprintf("consumer-%02d", index), Durable: fmt.Sprintf("consumer-%02d", index), Mode: "pull", DeliverPolicy: "all", AckPolicy: "explicit", AckWaitNanos: int64(30 * time.Second), MaxDeliver: 5, ReplayPolicy: "instant", FilterSubjects: []string{fmt.Sprintf("subject.%d", index)}, Metadata: map[string]string{"rabbit-jetstream.io/queue": queue}, Pending: uint64(index), AckPending: index}
	}
	return items, nil
}

func BenchmarkCollectGlobalConsumers100k(b *testing.B) {
	source := newBenchmarkGlobalConsumerSource()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		generation, err := collectGlobalConsumers(context.Background(), source, time.Now)
		if err != nil {
			b.Fatal(err)
		}
		if len(generation.Rows) != benchmarkGlobalStreams*benchmarkConsumersPerStream {
			b.Fatalf("rows=%d", len(generation.Rows))
		}
		for _, row := range generation.Rows {
			if row.Status != "present" || row.Ownership != "matching" {
				b.Fatalf("unexpected row semantics: status=%s ownership=%s", row.Status, row.Ownership)
			}
		}
	}
}

func BenchmarkQueryGlobalConsumers100k(b *testing.B) {
	source := newBenchmarkGlobalConsumerSource()
	generation, err := collectGlobalConsumers(context.Background(), source, time.Now)
	if err != nil {
		b.Fatal(err)
	}
	query := globalConsumerQuery{search: "consumer-09", mode: "pull"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		rows := queryGlobalConsumers(generation.Rows, query)
		if len(rows) != benchmarkGlobalStreams {
			b.Fatalf("rows=%d", len(rows))
		}
	}
}

func BenchmarkHTTPGlobalConsumers100k(b *testing.B) {
	source := newBenchmarkGlobalConsumerSource()
	byStream := make(map[string][]jetstream.Consumer, len(source.streams))
	for _, stream := range source.streams {
		byStream[stream.Name], _ = source.ListConsumers(context.Background(), stream.Name)
	}
	backend := &globalHTTPBackend{
		fakeBackend: &fakeBackend{streams: source.streams, declarations: source.declarations},
		byStream:    byStream,
	}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "benchmark", "dev", nil, nil, AuthConfig{
		RequireReadAuth: true,
		OperatorTokens:  []string{"operator"},
		AuditorTokens:   []string{"auditor"},
	})
	refresh := httptest.NewRequest(http.MethodPost, "/api/v1/consumers/refresh", nil)
	refresh.Header.Set("Authorization", "Bearer operator")
	refreshResponse := httptest.NewRecorder()
	handler.ServeHTTP(refreshResponse, refresh)
	if refreshResponse.Code != http.StatusOK {
		b.Fatalf("refresh status=%d body=%s", refreshResponse.Code, refreshResponse.Body.String())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/consumers?q=consumer-09&mode=pull&limit=200", nil)
		request.Header.Set("Authorization", "Bearer auditor")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatalf("query status=%d body=%s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), `"ownership":"different"`) || !strings.Contains(response.Body.String(), `"total":10000`) {
			b.Fatalf("query returned incorrect ownership or total")
		}
		b.ReportMetric(float64(response.Body.Len()), "response-B")
	}
}
