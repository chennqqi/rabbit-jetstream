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
	want := []string{"rjs.q.routed.ingress", "rjs.q.routed.x.broadcasts.fanout", "rjs.q.routed.x.commands.direct.orders.create", "rjs.q.routed.x.events.topic.audit", "rjs.q.routed.x.events.topic.audit.>", "rjs.q.routed.x.events.topic.orders.*"}
	if !reflect.DeepEqual(plan.Stream.Subjects, want) || !reflect.DeepEqual(plan.Consumer.FilterSubjects, want) || len(plan.Routing) != 3 {
		t.Fatalf("routing plan=%#v", plan)
	}
}

func TestPlanAlwaysIncludesQueueIngress(t *testing.T) {
	queue, err := ParseQueue(strings.NewReader(validQueueYAML))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.Stream.Subjects, QueueIngressSubject("orders")) {
		t.Fatalf("subjects %v do not include queue ingress", plan.Stream.Subjects)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

func TestBindingSubjectsCoversRootTopicAndRejectsInvalidInputs(t *testing.T) {
	subjects, err := BindingSubjects("target", Binding{Exchange: "events", Type: "topic", Keys: []string{"#"}})
	if err != nil || !reflect.DeepEqual(subjects, []string{"rjs.q.target.x.events.topic", "rjs.q.target.x.events.topic.>"}) {
		t.Fatalf("root topic subjects=%v err=%v", subjects, err)
	}
	subjects, err = BindingSubjects("target", Binding{Exchange: "commands", Type: "direct", Keys: []string{"orders.create", "orders.create"}})
	if err != nil || !reflect.DeepEqual(subjects, []string{"rjs.q.target.x.commands.direct.orders.create"}) {
		t.Fatalf("deduplicated direct subjects=%v err=%v", subjects, err)
	}
	for _, test := range []struct {
		queue   string
		binding Binding
	}{
		{"bad.queue", Binding{Exchange: "events", Type: "fanout"}},
		{"target", Binding{Exchange: "events", Type: "headers"}},
	} {
		if _, err := BindingSubjects(test.queue, test.binding); err == nil {
			t.Fatalf("BindingSubjects(%q,%+v) accepted invalid input", test.queue, test.binding)
		}
	}
}

func TestQueuePublishSubjectRejectsInvalidProtocolValues(t *testing.T) {
	for _, test := range []struct{ queue, exchange, kind, key string }{
		{"bad.queue", "events", "fanout", ""},
		{"target", "bad.exchange", "fanout", ""},
		{"target", "events", "fanout", "unexpected"},
		{"target", "events", "direct", ""},
		{"target", "events", "headers", "key"},
		{"target", "events", "topic", "bad..key"},
	} {
		if _, err := QueuePublishSubject(test.queue, test.exchange, test.kind, test.key); err == nil {
			t.Fatalf("QueuePublishSubject(%q,%q,%q,%q) accepted invalid input", test.queue, test.exchange, test.kind, test.key)
		}
	}
}

func TestPriorityResourceNames(t *testing.T) {
	subject, err := QueuePrioritySubject("orders", 255)
	if err != nil || subject != "rjs.q.orders.p.255" {
		t.Fatalf("QueuePrioritySubject() = %q, %v", subject, err)
	}
	consumer, err := PriorityConsumerName("orders", 0)
	if err != nil || consumer != "RJSQC_orders_P0" {
		t.Fatalf("PriorityConsumerName() = %q, %v", consumer, err)
	}
	for _, test := range []struct {
		queue    string
		priority int
	}{
		{"bad.name", 0},
		{"orders", -1},
		{"orders", 256},
	} {
		if _, err := QueuePrioritySubject(test.queue, test.priority); err == nil {
			t.Fatalf("QueuePrioritySubject(%q, %d) succeeded", test.queue, test.priority)
		}
		if _, err := PriorityConsumerName(test.queue, test.priority); err == nil {
			t.Fatalf("PriorityConsumerName(%q, %d) succeeded", test.queue, test.priority)
		}
	}
}

func TestBuildPlanProvisionsPrioritySubjectsAndConsumers(t *testing.T) {
	input := `apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata: {name: priority_orders}
spec:
  replicas: 1
  subjects: [orders.created]
  maxPriority: 2
`
	queue, err := ParseQueue(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	wantSubjects := []string{"rjs.q.priority_orders.p.0", "rjs.q.priority_orders.p.1", "rjs.q.priority_orders.p.2"}
	if !reflect.DeepEqual(plan.Stream.Subjects, wantSubjects) {
		t.Fatalf("priority subjects = %v, want %v", plan.Stream.Subjects, wantSubjects)
	}
	if plan.Consumer.Name != "RJSQC_priority_orders_P0" || !reflect.DeepEqual(plan.Consumer.FilterSubjects, wantSubjects[:1]) {
		t.Fatalf("priority zero consumer = %#v", plan.Consumer)
	}
	if len(plan.PriorityConsumers) != 2 || plan.PriorityConsumers[1].Name != "RJSQC_priority_orders_P2" || !reflect.DeepEqual(plan.PriorityConsumers[1].FilterSubjects, wantSubjects[2:]) {
		t.Fatalf("priority consumers = %#v", plan.PriorityConsumers)
	}
}
