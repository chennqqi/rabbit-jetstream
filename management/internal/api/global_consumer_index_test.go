package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type blockingGlobalConsumerSource struct {
	*globalConsumerFixture
	entered chan struct{}
	release chan struct{}
}

func (source *blockingGlobalConsumerSource) ListStreams(ctx context.Context) ([]jetstream.Stream, error) {
	close(source.entered)
	select {
	case <-source.release:
		return source.streams, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type failingGlobalConsumerSource struct{ err error }

func (source failingGlobalConsumerSource) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	return nil, source.err
}
func (source failingGlobalConsumerSource) ListStreams(context.Context) ([]jetstream.Stream, error) {
	return nil, source.err
}
func (source failingGlobalConsumerSource) ListConsumers(context.Context, string) ([]jetstream.Consumer, error) {
	return nil, source.err
}

type globalConsumerFixture struct {
	declarations     [][]topology.Declaration
	streams          []jetstream.Stream
	consumers        map[string][]jetstream.Consumer
	failStream       string
	declarationCalls int
}

func (f *globalConsumerFixture) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	f.declarationCalls++
	index := f.declarationCalls - 1
	if index >= len(f.declarations) {
		index = len(f.declarations) - 1
	}
	return f.declarations[index], nil
}
func (f *globalConsumerFixture) ListStreams(context.Context) ([]jetstream.Stream, error) {
	return f.streams, nil
}
func (f *globalConsumerFixture) ListConsumers(_ context.Context, stream string) ([]jetstream.Consumer, error) {
	if stream == f.failStream {
		return nil, errors.New("offline secret")
	}
	return append([]jetstream.Consumer(nil), f.consumers[stream]...), nil
}

func TestCollectGlobalConsumersCompleteJoin(t *testing.T) {
	expected := topology.ConsumerPlan{Name: "C", Stream: "S", Mode: "pull", DeliverPolicy: "all", AckPolicy: "explicit", AckWaitNanos: 30, MaxDeliver: 5, ReplayPolicy: "instant", FilterSubjects: []string{"a"}}
	declaration := topology.Declaration{Queue: "q", KVRevision: 7, Plan: topology.Plan{Queue: "q", Stream: topology.StreamPlan{Name: "S"}, Consumer: expected}}
	f := &globalConsumerFixture{
		declarations: [][]topology.Declaration{{declaration}, {declaration}},
		streams:      []jetstream.Stream{{Name: "S", Metadata: map[string]string{"rabbit-jetstream.io/queue": "q"}}, {Name: "EXT"}},
		consumers: map[string][]jetstream.Consumer{
			"S":   {{Stream: "S", Name: "C", Durable: "durable-c", Mode: "pull", DeliverPolicy: "all", AckPolicy: "explicit", AckWaitNanos: 30, MaxDeliver: 5, ReplayPolicy: "instant", FilterSubjects: []string{"a"}, Metadata: map[string]string{"rabbit-jetstream.io/queue": "q"}, Pending: 0, AckPending: 0}, {Stream: "S", Name: "extra", Mode: "push"}},
			"EXT": {{Stream: "EXT", Name: "same", Durable: "same", Mode: "pull", Pending: 9, AckPending: 2}},
		},
	}
	now := time.Date(2026, 9, 11, 1, 2, 3, 0, time.UTC)
	generation, err := collectGlobalConsumers(context.Background(), f, func() time.Time { now = now.Add(time.Second); return now })
	if err != nil {
		t.Fatal(err)
	}
	if generation.ID == "" || len(generation.Rows) != 3 || f.declarationCalls != 2 {
		t.Fatalf("generation=%#v calls=%d", generation, f.declarationCalls)
	}
	managed := generation.Rows[1]
	if managed.Stream != "S" || managed.Name != "C" || managed.Queue != "q" || managed.Status != "present" || managed.Ownership != "matching" || managed.Pending == nil || *managed.Pending != 0 || managed.AckPending == nil || *managed.AckPending != 0 {
		t.Fatalf("managed=%#v", managed)
	}
	external := generation.Rows[0]
	if external.Stream != "EXT" || external.Queue != "" || external.Ownership != "external" {
		t.Fatalf("external=%#v", external)
	}
}

func TestCollectGlobalConsumersIncludesMissingAndRejectsPartial(t *testing.T) {
	plan := topology.Plan{Queue: "q", Stream: topology.StreamPlan{Name: "S"}, Consumer: topology.ConsumerPlan{Name: "missing", Stream: "S", Mode: "pull"}}
	declaration := topology.Declaration{Queue: "q", KVRevision: 1, Plan: plan}
	f := &globalConsumerFixture{declarations: [][]topology.Declaration{{declaration}, {declaration}}, streams: []jetstream.Stream{{Name: "S"}, {Name: "broken"}}, consumers: map[string][]jetstream.Consumer{}, failStream: "broken"}
	if generation, err := collectGlobalConsumers(context.Background(), f, time.Now); err == nil || generation != nil {
		t.Fatalf("generation=%#v err=%v", generation, err)
	}
	f.failStream = ""
	generation, err := collectGlobalConsumers(context.Background(), f, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(generation.Rows) != 1 || generation.Rows[0].Status != "missing" || generation.Rows[0].Pending != nil {
		t.Fatalf("rows=%#v", generation.Rows)
	}
}

func TestCollectGlobalConsumersIncludesExpectedMembersWhenStreamIsMissing(t *testing.T) {
	plan := topology.Plan{Queue: "q", Stream: topology.StreamPlan{Name: "ABSENT"}, Consumer: topology.ConsumerPlan{Name: "C", Stream: "ABSENT", Mode: "pull"}}
	declaration := topology.Declaration{Queue: "q", KVRevision: 1, Plan: plan}
	f := &globalConsumerFixture{declarations: [][]topology.Declaration{{declaration}, {declaration}}, consumers: map[string][]jetstream.Consumer{}}
	generation, err := collectGlobalConsumers(context.Background(), f, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(generation.Rows) != 1 || generation.Rows[0].Stream != "ABSENT" || generation.Rows[0].Name != "C" || generation.Rows[0].Status != "missing" || generation.Rows[0].Pending != nil {
		t.Fatalf("rows=%#v", generation.Rows)
	}
}

func TestCollectGlobalConsumersRejectsDeclarationChurn(t *testing.T) {
	before := topology.Declaration{Queue: "q", KVRevision: 1, Plan: topology.Plan{Queue: "q", Stream: topology.StreamPlan{Name: "S"}, Consumer: topology.ConsumerPlan{Name: "C", Stream: "S"}}}
	after := before
	after.KVRevision = 2
	f := &globalConsumerFixture{declarations: [][]topology.Declaration{{before}, {after}}, streams: []jetstream.Stream{{Name: "S"}}, consumers: map[string][]jetstream.Consumer{"S": {}}}
	if generation, err := collectGlobalConsumers(context.Background(), f, time.Now); err == nil || generation != nil {
		t.Fatalf("generation=%#v err=%v", generation, err)
	}
}

func TestQueryGlobalConsumersFiltersBeforeStablePageOrder(t *testing.T) {
	rows := []globalConsumerRow{{Queue: "q2", Stream: "S2", Name: "same", Mode: "pull", Status: "present"}, {Queue: "q1", Stream: "S1", Name: "missing", Mode: "pull", Status: "missing"}, {Queue: "q1", Stream: "S1", Name: "same", Durable: "needle", Mode: "push", Status: "present"}}
	got := queryGlobalConsumers(rows, globalConsumerQuery{search: "needle", queue: "q1", mode: "push"})
	if len(got) != 1 || got[0].Stream != "S1" || got[0].Name != "same" {
		t.Fatalf("got=%#v", got)
	}
	got = queryGlobalConsumers(rows, globalConsumerQuery{descending: true})
	if len(got) != 3 || got[0].Stream != "S2" || got[1].Name != "same" || got[2].Name != "missing" {
		t.Fatalf("descending=%#v", got)
	}
}

func TestGlobalConsumerIndexCoalescesRefreshAndRetainsStaleGeneration(t *testing.T) {
	fixture := &globalConsumerFixture{declarations: [][]topology.Declaration{{}, {}}, streams: []jetstream.Stream{}, consumers: map[string][]jetstream.Consumer{}}
	source := &blockingGlobalConsumerSource{globalConsumerFixture: fixture, entered: make(chan struct{}), release: make(chan struct{})}
	index := &globalConsumerIndex{}
	completed := make(chan error, 1)
	go func() { _, _, err := index.refresh(context.Background(), source, time.Now); completed <- err }()
	<-source.entered
	if status := index.status(); status.State != "collecting" || status.Generation != nil {
		t.Fatalf("collecting=%#v", status)
	}
	if generation, busy, err := index.refresh(context.Background(), source, time.Now); err != nil || !busy || generation != nil {
		t.Fatalf("coalesced generation=%#v busy=%v err=%v", generation, busy, err)
	}
	close(source.release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	ready := index.status()
	if ready.State != "ready" || ready.Generation == nil {
		t.Fatalf("ready=%#v", ready)
	}
	original := ready.Generation.ID
	if generation, busy, err := index.refresh(context.Background(), failingGlobalConsumerSource{err: errors.New("secret backend unavailable")}, time.Now); err == nil || busy || generation != nil {
		t.Fatalf("failed generation=%#v busy=%v err=%v", generation, busy, err)
	}
	stale := index.status()
	if stale.State != "stale" || stale.Generation == nil || stale.Generation.ID != original || stale.FailedAt == nil {
		t.Fatalf("stale=%#v", stale)
	}
}
