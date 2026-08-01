package topology

import (
	"reflect"
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

func TestBuildPlanMapsDirectTopicAndFanoutBindings(t *testing.T) {
	input := `apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata: {name: routed}
spec:
  replicas: 1
  bindings:
    - {exchange: commands, type: direct, keys: [orders.create]}
    - {exchange: events, type: topic, keys: [orders.*, audit.#]}
    - {exchange: broadcasts, type: fanout}
`
	queue, err := ParseQueue(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rjs.q.routed.x.broadcasts.fanout", "rjs.q.routed.x.commands.direct.orders.create", "rjs.q.routed.x.events.topic.audit", "rjs.q.routed.x.events.topic.audit.>", "rjs.q.routed.x.events.topic.orders.*"}
	if !reflect.DeepEqual(plan.Stream.Subjects, want) || !reflect.DeepEqual(plan.Consumer.FilterSubjects, want) || len(plan.Routing) != 3 {
		t.Fatalf("routing plan=%#v", plan)
	}
}

func TestQueuePublishSubjectProtocol(t *testing.T) {
	for _, test := range []struct{ exchange, kind, key, want string }{
		{"commands", "direct", "orders.create", "rjs.q.target.x.commands.direct.orders.create"},
		{"events", "topic", "orders.created", "rjs.q.target.x.events.topic.orders.created"},
		{"events", "topic", "", "rjs.q.target.x.events.topic"},
		{"news", "fanout", "", "rjs.q.target.x.news.fanout"},
	} {
		got, err := QueuePublishSubject("target", test.exchange, test.kind, test.key)
		if err != nil || got != test.want {
			t.Fatalf("QueuePublishSubject=%q,%v want=%q", got, err, test.want)
		}
	}
	if _, err := QueuePublishSubject("target", "events", "direct", "orders.*"); err == nil {
		t.Fatal("wildcard publish key accepted")
	}
}
