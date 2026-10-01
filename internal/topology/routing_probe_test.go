package topology

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func probeQueue() Queue {
	return Queue{APIVersion: QueueAPIVersion, Kind: QueueKind, Metadata: Metadata{Name: "orders"}, Spec: QueueSpec{Replicas: 1, Subjects: []string{"orders.>"}}}
}

func TestProbeRouting_LiteralSemantics(t *testing.T) {
	for _, test := range []struct {
		pattern, subject string
		match            bool
	}{
		{"orders.>", "orders", false}, {"orders.>", "orders.created", true},
		{"orders.>", "orders.created.eu", true}, {"orders.*", "orders", false},
		{"orders.*", "orders.created", true}, {"orders.*", "orders.created.eu", false},
		{"orders.*.done", "orders.eu.done", true}, {"orders.*.done", "orders.eu.new", false},
		{"orders.created", "orders.created", true}, {"orders.created", "Orders.created", false},
		{">", "orders", true}, {"*", "orders", true}, {"*", "orders.created", false},
	} {
		t.Run(test.pattern+"/"+test.subject, func(t *testing.T) {
			queue := probeQueue()
			queue.Spec.Subjects = []string{test.pattern}
			result, err := ProbeRouting(queue, RoutingProbe{Subject: test.subject})
			if err != nil || (len(result.MatchedStreamSubjects) > 0) != test.match {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			if result.Queue != "orders" || result.Stream != StreamName("orders") || result.Revision == "" || len(result.Bindings) != 0 {
				t.Fatalf("invalid provenance: %+v", result)
			}
		})
	}
}

func TestProbeRouting_BindingTranslation(t *testing.T) {
	for _, test := range []struct {
		kind, key, routingKey string
		match                 bool
	}{
		{"direct", "created", "created", true}, {"direct", "created", "deleted", false},
		{"topic", "orders.#", "orders", true}, {"topic", "orders.#", "orders.eu.new", true},
		{"topic", "orders.*", "orders", false}, {"topic", "orders.*", "orders.eu", true},
		{"topic", "orders.*", "orders.eu.new", false}, {"topic", "#", "", true},
		{"topic", "#", "new.eu", true}, {"topic", "*.#", "eu", true},
		{"topic", "*.#", "", false}, {"fanout", "", "", true},
	} {
		t.Run(test.kind+"/"+test.key+"/"+test.routingKey, func(t *testing.T) {
			queue := probeQueue()
			queue.Spec.Subjects = nil
			binding := Binding{Exchange: "events", Type: test.kind}
			if test.key != "" {
				binding.Keys = []string{test.key}
			}
			queue.Spec.Bindings = []Binding{binding}
			probe := RoutingProbe{Exchange: "events", Type: test.kind, RoutingKey: test.routingKey}
			result, err := ProbeRouting(queue, probe)
			if err != nil || len(result.Bindings) != 1 || (len(result.MatchedStreamSubjects) > 0) != test.match || (len(result.Bindings[0].MatchedSubjects) > 0) != test.match {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			want, _ := QueuePublishSubject("orders", probe.Exchange, probe.Type, probe.RoutingKey)
			if result.Subject != want {
				t.Fatalf("subject=%q want=%q", result.Subject, want)
			}
			probe.Exchange = "other"
			other, err := ProbeRouting(queue, probe)
			if err != nil || len(other.MatchedStreamSubjects) != 0 || len(other.Bindings[0].MatchedSubjects) != 0 {
				t.Fatalf("cross-exchange match: %+v %v", other, err)
			}
		})
	}
}

func TestProbeRouting_RejectsInvalidInputs(t *testing.T) {
	for _, probe := range []RoutingProbe{
		{}, {Subject: "a", Exchange: "events"}, {Subject: "a", Type: "direct"}, {Subject: "a", RoutingKey: "b"},
		{Subject: "a.*"}, {Subject: "a.>"}, {Subject: "a..b"}, {Subject: "a b"},
		{Exchange: "events", Type: "headers"}, {Exchange: "events", Type: "direct"},
		{Exchange: "events", Type: "topic", RoutingKey: "a.#"}, {Exchange: "events", Type: "topic", RoutingKey: "a.*"},
		{Exchange: "events", Type: "fanout", RoutingKey: "a"}, {Exchange: "a.b", Type: "fanout"},
	} {
		result, err := ProbeRouting(probeQueue(), probe)
		if err == nil || !reflect.DeepEqual(result, RoutingProbeResult{}) {
			t.Fatalf("accepted %+v: %+v %v", probe, result, err)
		}
	}
	for _, binding := range []Binding{
		{Exchange: "events", Type: "topic", Keys: []string{"a.#.b"}},
		{Exchange: "events", Type: "headers"},
	} {
		queue := probeQueue()
		queue.Spec.Subjects = nil
		queue.Spec.Bindings = []Binding{binding}
		if _, err := ProbeRouting(queue, RoutingProbe{Subject: "a"}); err == nil {
			t.Fatalf("accepted invalid declaration: %+v", binding)
		}
	}
}

func TestProbeRouting_PriorityAndIngress(t *testing.T) {
	queue := probeQueue()
	ingress := QueueIngressSubject("orders")
	result, err := ProbeRouting(queue, RoutingProbe{Subject: ingress})
	if err != nil || !reflect.DeepEqual(result.MatchedStreamSubjects, []string{ingress}) {
		t.Fatalf("ordinary ingress: %+v %v", result, err)
	}
	priority := 2
	queue.Spec.MaxPriority = &priority
	for _, subject := range []string{ingress, "orders.created", "rjs.q.orders.p.0", "rjs.q.orders.p.2", "rjs.q.orders.p.3", "rjs.q.other.p.1"} {
		result, err := ProbeRouting(queue, RoutingProbe{Subject: subject})
		want := subject == "rjs.q.orders.p.0" || subject == "rjs.q.orders.p.2"
		if err != nil || (len(result.MatchedStreamSubjects) > 0) != want {
			t.Fatalf("priority %q: %+v %v", subject, result, err)
		}
	}
	queue.Spec.Subjects = nil
	queue.Spec.Bindings = []Binding{{Exchange: "events", Type: "fanout"}}
	subject, _ := QueuePublishSubject("orders", "events", "fanout", "")
	result, err = ProbeRouting(queue, RoutingProbe{Subject: subject})
	if err != nil || len(result.Bindings[0].MatchedSubjects) != 1 || len(result.MatchedStreamSubjects) != 0 {
		t.Fatalf("binding match must not imply priority Stream match: %+v %v", result, err)
	}
	if _, err := ProbeRouting(queue, RoutingProbe{Exchange: "events", Type: "fanout"}); err == nil {
		t.Fatal("silently invented exchange-to-priority translation")
	}
}

func TestProbeRouting_DeterministicAndDoesNotMutate(t *testing.T) {
	queue := probeQueue()
	queue.Spec.Subjects = nil
	queue.Spec.Bindings = []Binding{
		{Exchange: "z", Type: "direct", Keys: []string{"z", "a"}},
		{Exchange: "a", Type: "topic", Keys: []string{"z.#", "a.*"}},
	}
	before, _ := json.Marshal(queue)
	probe := RoutingProbe{Exchange: "z", Type: "direct", RoutingKey: "z"}
	first, err := ProbeRouting(queue, probe)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ProbeRouting(queue, probe)
	after, _ := json.Marshal(queue)
	if err != nil || !reflect.DeepEqual(first, second) || string(before) != string(after) {
		t.Fatalf("mutated or nondeterministic: before=%s after=%s error=%v", before, after, err)
	}
	first.Bindings[0].Keys[0] = "changed"
	first.Bindings[0].Subjects[0] = "changed"
	first.StreamSubjects[0] = "changed"
	after, _ = json.Marshal(queue)
	if string(before) != string(after) {
		t.Fatal("result aliases input")
	}
	queue = probeQueue()
	queue.Spec.Subjects = []string{"z.>", "a.*"}
	before, _ = json.Marshal(queue)
	if _, err := ProbeRouting(queue, RoutingProbe{Subject: "a.b"}); err != nil {
		t.Fatal(err)
	}
	after, _ = json.Marshal(queue)
	if string(before) != string(after) {
		t.Fatal("sorted caller subjects")
	}
	result, _ := ProbeRouting(queue, RoutingProbe{Subject: "none"})
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "null") {
		t.Fatalf("empty collections must be arrays: %s", encoded)
	}
}
