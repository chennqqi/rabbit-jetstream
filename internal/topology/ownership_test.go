package topology

import "testing"

func TestReconcileBlocksUnmarkedAndForeignResources(t *testing.T) {
	for _, resource := range []string{"stream", "consumer"} {
		for _, owner := range []string{"", "foreign"} {
			t.Run(resource+"/"+owner, func(t *testing.T) {
				plan := testPlan(t, false)
				observed := observedFromPlan(plan)
				index, metadata := 0, observed.Stream.Metadata
				if resource == "consumer" {
					index, metadata = 1, observed.Consumer.Metadata
				}
				metadata["rabbit-jetstream.io/queue"] = owner
				result := Reconcile(plan, observed)
				operation := result.Operations[index]
				if result.Status != "blocked" || !result.Blocked || !operation.Blocked || operation.Impact != "unsupported" || operation.Reason == "" {
					t.Fatalf("ownership not protected: %#v", result)
				}
				if metadata["rabbit-jetstream.io/queue"] != owner {
					t.Fatal("observation mutated")
				}
			})
		}
	}
}
