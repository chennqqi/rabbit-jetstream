package topology

import (
	"strings"
	"testing"
)

func TestReconcileCreatesMissingResources(t *testing.T) {
	plan := testPlan(t, false)
	result := Reconcile(plan, ObservedTopology{})
	if result.Status != "ready" || result.Blocked || result.Operations[0].Action != "create" || result.Operations[1].Action != "create" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReconcileNoopForMatchingResources(t *testing.T) {
	plan := testPlan(t, false)
	result := Reconcile(plan, observedFromPlan(plan))
	if result.Status != "noop" || result.Operations[0].Action != "noop" || result.Operations[1].Action != "noop" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReconcileBlocksStreamRecreation(t *testing.T) {
	plan := testPlan(t, false)
	observed := observedFromPlan(plan)
	observed.Stream.Storage = "memory"
	result := Reconcile(plan, observed)
	if !result.Blocked || result.Operations[0].Action != "recreate" || result.Operations[1].Action != "recreate" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReconcileAllowsConsumerUpdate(t *testing.T) {
	plan := testPlan(t, false)
	observed := observedFromPlan(plan)
	observed.Consumer.AckWaitNanos /= 2
	result := Reconcile(plan, observed)
	if result.Blocked || result.Operations[1].Action != "update" || result.Operations[1].Impact != "disruptive" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReconcileBlocksRetentionReduction(t *testing.T) {
	plan := testPlan(t, false)
	observed := observedFromPlan(plan)
	observed.Stream.MaxBytes = plan.Stream.MaxBytes * 2
	result := Reconcile(plan, observed)
	if !result.Blocked || result.Operations[0].Action != "update" || result.Operations[0].Impact != "destructive" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReconcileEnsuresDLQWorker(t *testing.T) {
	plan := testPlan(t, true)
	result := Reconcile(plan, ObservedTopology{})
	last := result.Operations[len(result.Operations)-1]
	if result.Status != "ready" || last.Action != "ensure" || last.Resource != "dead-letter-worker" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func testPlan(t *testing.T, dlq bool) Plan {
	t.Helper()
	queue, err := ParseQueue(strings.NewReader(validQueueYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !dlq {
		queue.Spec.DeadLetter = nil
	}
	plan, err := BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func observedFromPlan(plan Plan) ObservedTopology {
	return ObservedTopology{
		Stream: &ObservedStream{
			Name: plan.Stream.Name, Subjects: append([]string(nil), plan.Stream.Subjects...),
			Storage: plan.Stream.Storage, Replicas: plan.Stream.Replicas, Retention: plan.Stream.Retention,
			Discard: plan.Stream.Discard, MaxAgeNanos: plan.Stream.MaxAgeNanos, MaxBytes: plan.Stream.MaxBytes,
			MaxMessages: plan.Stream.MaxMessages, Metadata: cloneMap(plan.Stream.Metadata),
		},
		Consumer: &ObservedConsumer{
			Stream: plan.Consumer.Stream, Name: plan.Consumer.Name, Durable: plan.Consumer.Name,
			FilterSubjects: append([]string(nil), plan.Consumer.FilterSubjects...), Mode: plan.Consumer.Mode,
			DeliverPolicy: plan.Consumer.DeliverPolicy, AckPolicy: plan.Consumer.AckPolicy,
			AckWaitNanos: plan.Consumer.AckWaitNanos, MaxDeliver: plan.Consumer.MaxDeliver,
			ReplayPolicy: plan.Consumer.ReplayPolicy, Metadata: cloneMap(plan.Consumer.Metadata),
		},
	}
}
