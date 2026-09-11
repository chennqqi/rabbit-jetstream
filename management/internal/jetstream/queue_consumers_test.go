package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

type queueReadJS struct {
	jsapi.JetStream
	kv          *memoryKV
	stream      *queueReadStream
	err         error
	streamCalls int
	afterRead   func()
}

func (f *queueReadJS) KeyValue(context.Context, string) (jsapi.KeyValue, error) { return f.kv, nil }
func (f *queueReadJS) Stream(_ context.Context, name string) (jsapi.Stream, error) {
	f.streamCalls++
	if name != "RJSQ_orders" {
		panic("wrong Stream read")
	}
	if f.afterRead != nil {
		f.afterRead()
	}
	return f.stream, f.err
}

type queueReadStream struct {
	jsapi.Stream
	info   *jsapi.StreamInfo
	lister *fakeConsumerLister
	ctx    context.Context
}

func (s *queueReadStream) CachedInfo() *jsapi.StreamInfo { return s.info }
func (s *queueReadStream) ListConsumers(ctx context.Context) jsapi.ConsumerInfoLister {
	s.ctx = ctx
	return s.lister
}

func queueReadFixture(t *testing.T, priority *int) (*Client, *queueReadJS, topology.Plan) {
	t.Helper()
	queue := topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind, Metadata: topology.Metadata{Name: "orders"}, Spec: topology.QueueSpec{Subjects: []string{"orders.events"}, Replicas: 1, MaxPriority: priority}}
	plan, err := topology.BuildPlan(queue)
	if err != nil {
		t.Fatal(err)
	}
	backend := &queueReadJS{kv: newMemoryKV(), stream: &queueReadStream{info: &jsapi.StreamInfo{Config: jsapi.StreamConfig{Name: plan.Stream.Name, Metadata: plan.Stream.Metadata}}, lister: &fakeConsumerLister{}}}
	client := &Client{js: backend, metadataBucket: "META"}
	if err := client.persistDeclaration(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	return client, backend, plan
}

func TestQueueConsumersPriorityAndOwnership(t *testing.T) {
	priority := 7
	client, backend, plan := queueReadFixture(t, &priority)
	backend.stream.lister.items = []*jsapi.ConsumerInfo{
		{Stream: plan.Stream.Name, Name: plan.Consumer.Name, Config: jsapi.ConsumerConfig{Metadata: plan.Consumer.Metadata}, NumPending: 42},
		{Stream: plan.Stream.Name, Name: plan.PriorityConsumers[0].Name, Config: jsapi.ConsumerConfig{Metadata: map[string]string{"rabbit-jetstream.io/queue": "another"}}},
		{Stream: plan.Stream.Name, Name: "external", Config: jsapi.ConsumerConfig{DeliverSubject: "external.delivery"}},
	}
	got, err := client.QueueConsumers(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 9 || got.StreamOwnership != "matching" || got.StreamStatus != "present" || got.DeclarationRevision != "\"1\"" {
		t.Fatalf("collection = %+v", got)
	}
	expected, missing := 0, 0
	for _, row := range got.Items {
		if row.Stream != plan.Stream.Name {
			t.Fatal("cross-Stream member")
		}
		if row.Expected != nil {
			expected++
		}
		if row.Status == "missing" {
			missing++
			if row.Observed != nil || row.Expected == nil || row.Ownership != "unknown" {
				t.Fatalf("fabricated observation: %+v", row)
			}
		}
		if row.Name == plan.Consumer.Name && (row.Ownership != "matching" || row.Observed.Pending != 42) {
			t.Fatalf("primary = %+v", row)
		}
		if row.Name == plan.PriorityConsumers[0].Name && row.Ownership != "different" {
			t.Fatal("ownership mismatch hidden")
		}
		if row.Name == "external" && (row.Expected != nil || row.Ownership != "unmarked" || row.Observed.Mode != "push") {
			t.Fatalf("external = %+v", row)
		}
	}
	if expected != 8 || missing != 6 || backend.streamCalls != 1 {
		t.Fatalf("expected=%d missing=%d calls=%d", expected, missing, backend.streamCalls)
	}
}

func TestQueueConsumersReadFailures(t *testing.T) {
	for _, scenario := range []string{"missing declaration", "missing stream", "stream unavailable", "list failure", "limit", "duplicate", "wrong stream", "changed declaration", "removed declaration", "canceled", "explicit zero"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend, plan := queueReadFixture(t, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantErr := true
			switch scenario {
			case "missing declaration":
				_ = backend.kv.Delete(ctx, "queues.orders")
			case "missing stream":
				backend.err = jsapi.ErrStreamNotFound
				wantErr = false
			case "stream unavailable":
				backend.err = context.DeadlineExceeded
			case "list failure":
				backend.stream.lister.err = errors.New("enumeration incomplete")
			case "limit":
				for i := 0; i <= QueueConsumerReadLimit; i++ {
					backend.stream.lister.items = append(backend.stream.lister.items, &jsapi.ConsumerInfo{Stream: plan.Stream.Name, Name: fmt.Sprintf("worker-%04d", i)})
				}
			case "duplicate":
				backend.stream.lister.items = []*jsapi.ConsumerInfo{{Stream: plan.Stream.Name, Name: "same"}, {Stream: plan.Stream.Name, Name: "same"}}
			case "wrong stream":
				backend.stream.lister.items = []*jsapi.ConsumerInfo{{Stream: "wrong", Name: "same"}}
			case "changed declaration":
				backend.afterRead = func() {
					entry, _ := backend.kv.Get(ctx, "queues.orders")
					_, _ = backend.kv.Put(ctx, "queues.orders", entry.Value())
				}
			case "removed declaration":
				backend.afterRead = func() { _ = backend.kv.Delete(ctx, "queues.orders") }
			case "canceled":
				cancel()
			case "explicit zero":
				zero := 0
				client, backend, plan = queueReadFixture(t, &zero)
				wantErr = false
			}
			got, err := client.QueueConsumers(ctx, "orders")
			if (err != nil) != wantErr {
				t.Fatalf("result=%+v error=%v", got, err)
			}
			if wantErr && got != nil {
				t.Fatal("partial result presented as complete")
			}
			if !wantErr && (len(got.Items) != 1 || got.Items[0].Expected.Name != plan.Consumer.Name || got.Items[0].Observed != nil) {
				t.Fatalf("missing expected identity lost: %+v", got)
			}
			if (scenario == "changed declaration" || scenario == "removed declaration") && !errors.Is(err, ErrConflict) {
				t.Fatalf("conflict = %v", err)
			}
			if scenario == "missing declaration" && (!errors.Is(err, ErrNotFound) || backend.streamCalls != 0) {
				t.Fatalf("missing declaration should stop before broker reads: %v", err)
			}
			if scenario == "missing stream" && (got.StreamStatus != "missing" || got.StreamOwnership != "unknown") {
				t.Fatalf("missing stream = %+v", got)
			}
			if backend.stream.ctx != nil && backend.stream.ctx.Err() == nil {
				t.Fatal("enumeration context not canceled after return")
			}
		})
	}
}

func TestQueueConsumersLimitBoundaryAndStreamOwnership(t *testing.T) {
	client, backend, plan := queueReadFixture(t, nil)
	backend.stream.info.Config.Metadata = map[string]string{"rabbit-jetstream.io/queue": "different"}
	for i := 0; i < QueueConsumerReadLimit; i++ {
		backend.stream.lister.items = append(backend.stream.lister.items, &jsapi.ConsumerInfo{Stream: plan.Stream.Name, Name: fmt.Sprintf("extra-%04d", i)})
	}
	got, err := client.QueueConsumers(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != QueueConsumerReadLimit+1 || got.StreamOwnership != "different" {
		t.Fatalf("boundary/ownership = %+v", got)
	}
}

func TestQueueConsumersRejectCorruptPlan(t *testing.T) {
	for _, scenario := range []string{"queue", "stream", "duplicate", "name", "priority count"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend, plan := queueReadFixture(t, nil)
			switch scenario {
			case "queue":
				plan.Queue = "wrong"
			case "stream":
				plan.Consumer.Stream = "wrong"
			case "duplicate":
				plan.PriorityConsumers = []topology.ConsumerPlan{plan.Consumer}
			case "name":
				plan.Consumer.Name = ""
			case "priority count":
				plan.PriorityConsumers = make([]topology.ConsumerPlan, topology.MaximumPriority+1)
			}
			record := topology.Declaration{Queue: "orders", Plan: plan}
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.kv.Put(context.Background(), "queues.orders", encoded); err != nil {
				t.Fatal(err)
			}
			got, err := client.QueueConsumers(context.Background(), "orders")
			if err == nil || got != nil || backend.streamCalls != 0 {
				t.Fatalf("corrupt declaration reached broker: result=%+v error=%v", got, err)
			}
		})
	}
}
