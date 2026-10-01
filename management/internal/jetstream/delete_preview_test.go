package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

func TestDeletePreviewReadOnly(t *testing.T) {
	for _, scenario := range []string{"empty", "messages", "consumers", "unmarked", "foreign", "missing stream", "missing declaration", "stale", "final conflict", "read failure", "canceled", "create only", "no revision", "invalid name", "identity version", "identity queue", "identity plan", "identity stream", "identity revision"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend := testClient()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			plan := testQueuePlan(t, "orders")
			if _, err := client.Apply(ctx, plan); err != nil {
				t.Fatal(err)
			}
			decl, err := client.Declaration(ctx, plan.Queue)
			if err != nil {
				t.Fatal(err)
			}
			condition := ApplyPrecondition{ExpectedRevision: &decl.KVRevision}
			readOnly := &previewReadJS{backend: backend}
			name := plan.Queue
			stream := backend.streams[plan.Stream.Name]
			stream.info.State.Consumers = 0
			var wantErr error
			switch scenario {
			case "messages":
				stream.info.State.Msgs = 18446744073709551615
			case "consumers":
				stream.info.State.Consumers = 3
			case "unmarked":
				delete(stream.info.Config.Metadata, "rabbit-jetstream.io/queue")
			case "foreign":
				stream.info.Config.Metadata["rabbit-jetstream.io/queue"] = "other"
			case "missing stream":
				delete(backend.streams, plan.Stream.Name)
			case "missing declaration":
				readOnly.get = func(context.Context, string) (jsapi.KeyValueEntry, error) { return nil, jsapi.ErrKeyNotFound }
				wantErr = ErrNotFound
			case "stale":
				decl.KVRevision++
				wantErr = ErrConflict
			case "final conflict":
				reads := 0
				readOnly.get = func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
					entry, err := backend.kv.Get(ctx, key)
					reads++
					if err == nil && reads == 2 {
						copy := *entry.(*fakeKVEntry)
						copy.revision++
						return &copy, nil
					}
					return entry, err
				}
				wantErr = ErrConflict
			case "read failure":
				readOnly.streamErr = context.DeadlineExceeded
				wantErr = context.DeadlineExceeded
			case "canceled":
				cancel()
				wantErr = context.Canceled
			case "create only":
				condition.CreateOnly = true
				wantErr = ErrConflict
			case "no revision":
				condition.ExpectedRevision = nil
				wantErr = ErrConflict
			case "invalid name":
				name = "bad/name"
				wantErr = ErrConflict
			case "identity version", "identity queue", "identity plan", "identity stream", "identity revision":
				readOnly.get = func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
					entry, err := backend.kv.Get(ctx, key)
					if err != nil {
						return nil, err
					}
					var altered topology.Declaration
					if err := json.Unmarshal(entry.Value(), &altered); err != nil {
						return nil, err
					}
					switch scenario {
					case "identity version":
						altered.APIVersion = "future"
					case "identity queue":
						altered.Queue = "other"
					case "identity plan":
						altered.Plan.Queue = "other"
					case "identity stream":
						altered.Plan.Stream.Name = "other"
					case "identity revision":
						altered.Revision = "other"
					}
					copy := *entry.(*fakeKVEntry)
					copy.value, err = json.Marshal(altered)
					return &copy, err
				}
				wantErr = ErrConflict
			}
			client.js = readOnly // A mutation/lock/ensure call panics through the nil SDK interface.
			preview, err := client.PreviewDelete(ctx, name, condition)
			if wantErr != nil {
				if !errors.Is(err, wantErr) || preview != nil {
					t.Fatalf("preview=%#v err=%v want=%v", preview, err, wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if preview.Queue != name || preview.Stream != plan.Stream.Name || preview.ObservedAt.IsZero() || preview.BaseRevision == "" {
				t.Fatalf("invalid evidence: %#v", preview)
			}
			if scenario == "missing stream" {
				if preview.StreamPresent || preview.Messages != nil || preview.Consumers != nil || preview.Ownership != "unobserved" || preview.Blocked {
					t.Fatalf("fabricated missing counts: %#v", preview)
				}
				return
			}
			if !preview.StreamPresent || preview.Messages == nil || preview.Consumers == nil || *preview.Messages != stream.info.State.Msgs || *preview.Consumers != stream.info.State.Consumers {
				t.Fatalf("lost observation: %#v", preview)
			}
			blocked := scenario == "messages" || scenario == "unmarked" || scenario == "foreign"
			if preview.Blocked != blocked || preview.RequiresForce != (scenario == "messages") || (blocked && preview.Reason == "") {
				t.Fatalf("wrong protection: %#v", preview)
			}
			wantOwnership := "matching"
			if scenario == "unmarked" {
				wantOwnership = "unmarked"
			}
			if scenario == "foreign" {
				wantOwnership = "different"
			}
			if preview.Ownership != wantOwnership {
				t.Fatalf("ownership=%s", preview.Ownership)
			}
		})
	}
}


func TestDeletePreviewReportsDeadLetterDependents(t *testing.T) {
	client, backend := testClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targetPlan := testQueuePlan(t, "orders")
	if _, err := client.Apply(ctx, targetPlan); err != nil {
		t.Fatal(err)
	}
	dependentPlan := testQueuePlan(t, "payments")
	dependentPlan.DeadLetter = &topology.DeadLetterPlan{Queue: "orders"}
	if _, err := client.Apply(ctx, dependentPlan); err != nil {
		t.Fatal(err)
	}
	decl, err := client.Declaration(ctx, targetPlan.Queue)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := client.PreviewDelete(ctx, targetPlan.Queue, ApplyPrecondition{ExpectedRevision: &decl.KVRevision})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.DeadLetterDependents) != 1 || preview.DeadLetterDependents[0] != "payments" {
		t.Fatalf("dependents=%v, want [payments]", preview.DeadLetterDependents)
	}
	// The dependent itself reports no dependents.
	dependentDecl, err := client.Declaration(ctx, dependentPlan.Queue)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = client.PreviewDelete(ctx, dependentPlan.Queue, ApplyPrecondition{ExpectedRevision: &dependentDecl.KVRevision})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.DeadLetterDependents) != 0 {
		t.Fatalf("dependents=%v, want none", preview.DeadLetterDependents)
	}
	_ = backend
}
