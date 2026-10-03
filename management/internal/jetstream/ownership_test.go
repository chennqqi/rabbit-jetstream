package jetstream

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestOwnershipConflictBlocksPreviewAndApplyWithoutChangingResources(t *testing.T) {
	for _, target := range []string{"stream", "primary", "priority"} {
		for _, owner := range []string{"", "another_queue"} {
			t.Run(target+"/"+owner, func(t *testing.T) {
				ctx := context.Background()
				client, backend := testClient()
				document, err := topology.QueueDocument(testQueuePlan(t, "orders"))
				if err != nil {
					t.Fatal(err)
				}
				priority := 2
				document.Spec.MaxPriority = &priority
				plan, err := topology.BuildPlan(*document)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := client.Apply(ctx, plan); err != nil {
					t.Fatal(err)
				}
				stream := backend.streams[plan.Stream.Name]
				metadata := stream.info.Config.Metadata
				if target == "primary" {
					metadata = stream.consumers[plan.Consumer.Name].Config.Metadata
				}
				if target == "priority" {
					metadata = stream.consumers[plan.PriorityConsumers[0].Name].Config.Metadata
				}
				metadata["rabbit-jetstream.io/queue"] = owner
				before, _ := json.Marshal([]any{stream.info, stream.consumers})
				declaration, err := client.Declaration(ctx, plan.Queue)
				if err != nil {
					t.Fatal(err)
				}
				precondition := ApplyPrecondition{ExpectedRevision: &declaration.KVRevision}
				preview, err := client.Preview(ctx, plan, precondition)
				if err != nil {
					t.Fatal(err)
				}
				if !preview.Result.Blocked {
					t.Fatal("preview permits adoption")
				}
				result, err := client.ApplyConditional(ctx, plan, precondition)
				if err != nil {
					t.Fatal(err)
				}
				if !result.Blocked {
					t.Fatal("apply permits adoption")
				}
				actual := backend.streams[plan.Stream.Name]
				after, _ := json.Marshal([]any{actual.info, actual.consumers})
				if string(before) != string(after) {
					t.Fatal("blocked apply changed resources")
				}
				latest, err := client.Declaration(ctx, plan.Queue)
				if err != nil {
					t.Fatal(err)
				}
				if latest.KVRevision != declaration.KVRevision {
					t.Fatal("blocked apply persisted declaration")
				}
			})
		}
	}
}
