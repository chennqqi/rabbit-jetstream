package topology

import (
	"strings"
	"testing"
)

func TestBuildPlanMapsQueueResources(t *testing.T) {
	queue, err := ParseQueue(strings.NewReader(validQueueYAML))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stream.Name != "RJSQ_orders" || plan.Consumer.Name != "RJSQC_orders" {
		t.Fatalf("unexpected resource names: %#v", plan)
	}
	if plan.Stream.Retention != "workqueue" || plan.Consumer.Mode != "pull" || plan.Consumer.DeliverPolicy != "all" || plan.Consumer.AckPolicy != "explicit" || plan.Consumer.MaxDeliver != 7 {
		t.Fatalf("unexpected semantics: %#v", plan)
	}
	if plan.Stream.MaxBytes != 10<<30 || plan.Stream.Replicas != 3 || plan.Consumer.Stream != plan.Stream.Name {
		t.Fatalf("unexpected stream mapping: %#v", plan)
	}
	if plan.DeadLetter == nil || plan.DeadLetter.Stream != "RJSQ_orders_dlq" || len(plan.Warnings) != 1 {
		t.Fatalf("unexpected DLQ mapping: %#v", plan)
	}
	if plan.Stream.Metadata["rabbit-jetstream.io/revision"] != plan.Revision || len(plan.Revision) != 32 {
		t.Fatalf("unexpected revision metadata: %#v", plan.Stream.Metadata)
	}
}

func TestBuildPlanIsDeterministic(t *testing.T) {
	first, _ := ParseQueue(strings.NewReader(validQueueYAML))
	second, _ := ParseQueue(strings.NewReader(strings.Replace(validQueueYAML, "orders.created, orders.*", "orders.*, orders.created", 1)))
	firstPlan, err := BuildPlan(*first)
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := BuildPlan(*second)
	if err != nil {
		t.Fatal(err)
	}
	if firstPlan.Revision != secondPlan.Revision {
		t.Fatalf("revisions differ: %s != %s", firstPlan.Revision, secondPlan.Revision)
	}
}

func TestResourceNamesPreserveCaseToAvoidCollisions(t *testing.T) {
	if StreamName("orders") == StreamName("ORDERS") {
		t.Fatal("case-sensitive queue names collided")
	}
}
