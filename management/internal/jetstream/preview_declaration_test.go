package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

func TestPreviewDeclarationRejectsMismatchedBaseIdentity(t *testing.T) {
	for _, field := range []string{"version", "queue", "plan queue", "revision"} {
		t.Run(field, func(t *testing.T) {
			client, backend := testClient()
			ctx := context.Background()
			plan := testQueuePlan(t, "orders")
			if err := client.persistDeclaration(ctx, plan); err != nil {
				t.Fatal(err)
			}
			declaration, err := client.Declaration(ctx, plan.Queue)
			if err != nil {
				t.Fatal(err)
			}
			client.js = &previewReadJS{backend: backend, get: func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
				entry, err := backend.kv.Get(ctx, key)
				if err != nil {
					return nil, err
				}
				var altered topology.Declaration
				if err := json.Unmarshal(entry.Value(), &altered); err != nil {
					return nil, err
				}
				switch field {
				case "version":
					altered.APIVersion = "future"
				case "queue":
					altered.Queue = "other"
				case "plan queue":
					altered.Plan.Queue = "other"
				case "revision":
					altered.Revision = "other"
				}
				copy := *entry.(*fakeKVEntry)
				copy.value, err = json.Marshal(altered)
				return &copy, err
			}}
			preview, err := client.Preview(ctx, plan, ApplyPrecondition{ExpectedRevision: &declaration.KVRevision})
			if err != nil {
				t.Fatal(err)
			}
			review := preview.DeclarationReview
			if review.Status != "unavailable" || review.Reason != "base_identity_mismatch" || review.Diff != nil {
				t.Fatalf("identity mismatch compared: %#v", review)
			}
		})
	}
}

func TestPreviewDeclarationReview(t *testing.T) {
	for _, scenario := range []string{"create", "noop", "priority", "labels", "target unsupported", "base unsupported", "mid-read conflict", "final-read conflict"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend := testClient()
			ctx := context.Background()
			plan := testQueuePlan(t, "orders")
			precondition := ApplyPrecondition{CreateOnly: true}
			if scenario != "create" {
				base := plan
				if scenario == "base unsupported" {
					base.DeclarationSubjects = nil
				}
				if err := client.persistDeclaration(ctx, base); err != nil {
					t.Fatal(err)
				}
				declaration, err := client.Declaration(ctx, plan.Queue)
				if err != nil {
					t.Fatal(err)
				}
				precondition = ApplyPrecondition{ExpectedRevision: &declaration.KVRevision}
			}
			if scenario == "priority" || scenario == "labels" {
				document, err := topology.QueueDocument(plan)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "priority" {
					zero := 0
					document.Spec.MaxPriority = &zero
				} else {
					document.Metadata.Labels = map[string]string{"owner": "team"}
				}
				plan, err = topology.BuildPlan(*document)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "target unsupported" {
				plan.DeclarationSubjects = nil
			}
			readOnly := &previewReadJS{backend: backend}
			if scenario == "mid-read conflict" || scenario == "final-read conflict" {
				reads, conflictRead := 0, 2
				if scenario == "final-read conflict" {
					conflictRead = 3
				}
				readOnly.get = func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
					reads++
					entry, err := backend.kv.Get(ctx, key)
					if err == nil && reads == conflictRead {
						copy := *entry.(*fakeKVEntry)
						copy.revision++
						return &copy, nil
					}
					return entry, err
				}
			}
			client.js = readOnly // Any write/ensure path panics.
			preview, err := client.Preview(ctx, plan, precondition)
			if scenario == "mid-read conflict" || scenario == "final-read conflict" {
				if !errors.Is(err, ErrConflict) || preview != nil {
					t.Fatalf("conflict exposed preview: %#v %v", preview, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			review := preview.DeclarationReview
			if review == nil {
				t.Fatal("missing review")
			}
			switch scenario {
			case "create":
				if review.Status != "create" || review.Document == nil || review.Diff != nil {
					t.Fatalf("create=%#v", review)
				}
			case "target unsupported", "base unsupported":
				if review.Status != "unavailable" || review.Reason == "" || review.Diff != nil {
					t.Fatalf("unavailable=%#v", review)
				}
			default:
				if review.Status != "available" || review.Document == nil || review.Diff == nil || review.Diff.Queue != "orders" {
					t.Fatalf("review=%#v", review)
				}
				if scenario == "noop" && review.Diff.HasChanges() {
					t.Fatalf("unexpected diff: %#v", review.Diff)
				}
				if scenario != "noop" && len(review.Diff.Changes) != 1 {
					t.Fatalf("lost diff: %#v", review.Diff)
				}
			}
		})
	}
}
