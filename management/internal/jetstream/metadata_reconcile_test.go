package jetstream

import (
	"context"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestApplyMetadataMatchesPreviewAndPreservesExternalKeys(t *testing.T) {
	ctx := context.Background()
	client, backend := testClient()
	document, err := topology.QueueDocument(testQueuePlan(t, "orders"))
	if err != nil {
		t.Fatal(err)
	}
	priority := 2
	document.Spec.MaxPriority = &priority
	document.Metadata.Labels = map[string]string{"empty": ""}
	plan, err := topology.BuildPlan(*document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Apply(ctx, plan); err != nil {
		t.Fatal(err)
	}
	stream := backend.streams[plan.Stream.Name]
	alter := func(metadata map[string]string) {
		metadata["external/key"] = "keep"
		metadata["rabbit-jetstream.io/label.removed"] = ""
		delete(metadata, "rabbit-jetstream.io/label.empty")
	}
	alter(stream.info.Config.Metadata)
	for _, consumer := range stream.consumers {
		alter(consumer.Config.Metadata)
	}
	declaration, err := client.Declaration(ctx, plan.Queue)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := client.Preview(ctx, plan, ApplyPrecondition{ExpectedRevision: &declaration.KVRevision})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Result.Status != "ready" || preview.Result.Blocked {
		t.Fatalf("drift hidden: %#v", preview.Result)
	}
	for _, operation := range preview.Result.Operations {
		if operation.Action != "update" || len(operation.Changes) != 2 {
			t.Fatalf("metadata drift missing: %#v", operation)
		}
	}
	if _, err := client.ApplyConditional(ctx, plan, ApplyPrecondition{ExpectedRevision: &declaration.KVRevision}); err != nil {
		t.Fatal(err)
	}
	stream = backend.streams[plan.Stream.Name]
	verify := func(metadata map[string]string) {
		t.Helper()
		if metadata["external/key"] != "keep" {
			t.Fatal("external metadata lost")
		}
		if _, ok := metadata["rabbit-jetstream.io/label.removed"]; ok {
			t.Fatal("deleted label retained")
		}
		if value, ok := metadata["rabbit-jetstream.io/label.empty"]; !ok || value != "" {
			t.Fatal("empty label not restored")
		}
	}
	verify(stream.info.Config.Metadata)
	for _, consumer := range stream.consumers {
		verify(consumer.Config.Metadata)
	}
	observed, err := client.observedTopology(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result := topology.Reconcile(plan, observed); result.Status != "noop" {
		t.Fatalf("not converged: %#v", result)
	}
}
