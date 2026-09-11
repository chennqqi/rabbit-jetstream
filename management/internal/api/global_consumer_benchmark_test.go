package api

import (
	"context"
	"fmt"
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
	for index := range items {
		items[index] = jetstream.Consumer{Stream: stream, Name: fmt.Sprintf("consumer-%02d", index), Durable: fmt.Sprintf("consumer-%02d", index), Mode: "pull", DeliverPolicy: "all", AckPolicy: "explicit", AckWaitNanos: int64(30 * time.Second), MaxDeliver: 5, ReplayPolicy: "instant", FilterSubjects: []string{fmt.Sprintf("subject.%d", index)}, Metadata: map[string]string{"rabbit-jetstream.io/queue": source.declarations[0].Queue}, Pending: uint64(index), AckPending: index}
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
