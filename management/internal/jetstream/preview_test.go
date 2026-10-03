package jetstream

import (
	"context"
	"errors"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

// Only reads are implemented. Any write/lock/ensure path panics via the nil
// embedded SDK interface, including writes that would leave no final diff.
type previewReadJS struct {
	jsapi.JetStream
	backend   *fakeJS
	streamErr error
	get       func(context.Context, string) (jsapi.KeyValueEntry, error)
}
type previewReadKV struct {
	jsapi.KeyValue
	get  func(context.Context, string) (jsapi.KeyValueEntry, error)
	keys func(context.Context) ([]string, error)
}

func (kv previewReadKV) Get(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
	return kv.get(ctx, key)
}

func (kv previewReadKV) Keys(ctx context.Context, _ ...jsapi.WatchOpt) ([]string, error) {
	return kv.keys(ctx)
}
func (f *previewReadJS) KeyValue(context.Context, string) (jsapi.KeyValue, error) {
	if f.backend.kv == nil {
		return nil, jsapi.ErrBucketNotFound
	}
	get := f.get
	if get == nil {
		get = f.backend.kv.Get
	}
	// Keys backs the read-only DLQ-dependents scan in the delete preview;
	// it stays a pure read and degrades to no dependents when unavailable.
	keys := f.backend.kv.Keys
	return previewReadKV{get: get, keys: func(ctx context.Context) ([]string, error) {
		return keys(ctx)
	}}, nil
}
func (f *previewReadJS) Stream(ctx context.Context, name string) (jsapi.Stream, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.backend.Stream(ctx, name)
}

func TestPreviewReadOnly(t *testing.T) {
	for _, scenario := range []string{"create", "noop", "blocked", "conflict", "create collision", "missing declaration", "read failure", "canceled", "missing DLQ", "valid DLQ", "DLQ cycle", "concurrent change"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend := testClient()
			plan := testQueuePlan(t, "orders")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			precondition := ApplyPrecondition{CreateOnly: true}
			if scenario != "create" && scenario != "missing declaration" {
				if _, err := client.Apply(ctx, plan); err != nil {
					t.Fatal(err)
				}
				declaration, err := client.Declaration(ctx, plan.Queue)
				if err != nil {
					t.Fatal(err)
				}
				precondition = ApplyPrecondition{ExpectedRevision: &declaration.KVRevision}
			}
			readOnly := &previewReadJS{backend: backend}
			wantError := false
			switch scenario {
			case "blocked":
				plan.Stream.Storage = "memory"
			case "conflict":
				revision := uint64(9999)
				precondition.ExpectedRevision = &revision
				wantError = true
			case "create collision":
				precondition = ApplyPrecondition{CreateOnly: true}
				wantError = true
			case "missing declaration":
				revision := uint64(1)
				precondition = ApplyPrecondition{ExpectedRevision: &revision}
				wantError = true
			case "read failure":
				readOnly.streamErr = context.DeadlineExceeded
				wantError = true
			case "canceled":
				cancel()
				wantError = true
			case "missing DLQ", "valid DLQ", "DLQ cycle":
				plan.DeadLetter = &topology.DeadLetterPlan{Queue: "dead", Stream: "RJSQ_dead", Worker: "worker"}
				wantError = scenario != "valid DLQ"
				if scenario != "missing DLQ" {
					target := testQueuePlan(t, "dead")
					if scenario == "DLQ cycle" {
						target.DeadLetter = &topology.DeadLetterPlan{Queue: plan.Queue}
					}
					if err := client.persistDeclaration(ctx, target); err != nil {
						t.Fatal(err)
					}
					if scenario == "valid DLQ" {
						if _, err := client.Apply(ctx, target); err != nil {
							t.Fatal(err)
						}
					}
				}
			case "concurrent change":
				wantError = true
				reads := 0
				readOnly.get = func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
					reads++
					entry, err := backend.kv.Get(ctx, key)
					if reads == 2 && err == nil {
						altered := entry.(*fakeKVEntry)
						altered.revision++
						return altered, nil
					}
					return entry, err
				}
			}
			client.js = readOnly
			got, err := client.Preview(ctx, plan, precondition)
			if (err != nil) != wantError {
				t.Fatalf("preview=%+v error=%v", got, err)
			}
			if wantError {
				if got != nil {
					t.Fatal("error returned valid preview")
				}
				return
			}
			if got.ObservedAt.IsZero() || got.Plan.Queue != plan.Queue {
				t.Fatalf("missing preview identity/time: %+v", got)
			}
			if scenario == "create" && (!got.CreateOnly || got.BaseRevision != "" || got.Result.Status != "ready") {
				t.Fatalf("create=%+v", got)
			}
			if scenario == "noop" && (got.Result.Status != "noop" || got.BaseRevision == "") {
				t.Fatalf("noop=%+v", got)
			}
			if scenario == "blocked" && (!got.Result.Blocked || got.Result.Status != "blocked") {
				t.Fatalf("unsafe transition hidden: %+v", got)
			}
		})
	}
}

func TestPreviewThenApplyRechecksRevision(t *testing.T) {
	client, _ := testClient()
	plan := testQueuePlan(t, "orders")
	ctx := context.Background()
	if _, err := client.Apply(ctx, plan); err != nil {
		t.Fatal(err)
	}
	declaration, err := client.Declaration(ctx, plan.Queue)
	if err != nil {
		t.Fatal(err)
	}
	precondition := ApplyPrecondition{ExpectedRevision: &declaration.KVRevision}
	if _, err := client.Preview(ctx, plan, precondition); err != nil {
		t.Fatal(err)
	}
	changed := plan
	changed.Revision = "another-writer"
	if err := client.persistDeclaration(ctx, changed); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ApplyConditional(ctx, plan, precondition); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale preview authorized apply: %v", err)
	}
}
