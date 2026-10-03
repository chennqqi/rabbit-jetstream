package topology

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQueueDocumentRoundTrip(t *testing.T) {
	for _, spec := range []string{
		`subjects: [orders.events]`,
		`subjects: [orders.events]
  maxPriority: 0`,
		`subjects: [orders.events]
  maxPriority: 7`,
		`subjects: [orders.events]
  maxPriority: 255`,
		`bindings: [{exchange: direct, type: direct, keys: [z, a]}, {exchange: topic, type: topic, keys: ['orders.*', 'audit.#']}, {exchange: all, type: fanout}]`,
		`bindings: [{exchange: topic, type: topic, keys: ['orders.#']}]
  maxPriority: 7`,
		`subjects: [orders.events]
  deadLetter: {queue: dead}
  storage: memory
  retention: {maxAge: 1ns, maxBytes: 9223372036854775807, maxMessages: 9223372036854775807}
  delivery: {ackWait: 9223372036854775807ns, maxDeliver: 100000}`,
	} {
		t.Run(spec, func(t *testing.T) {
			input := "apiVersion: rabbit-jetstream.io/v1alpha1\nkind: Queue\nmetadata: {name: orders, labels: {owner: team, note: '中文'}}\nspec:\n  replicas: 3\n  " + spec
			queue, err := ParseQueue(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := BuildPlan(*queue)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(plan)
			document, err := QueueDocument(plan)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			reparsed, err := ParseQueue(strings.NewReader(string(encoded)))
			if err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(queue)
			got, _ := json.Marshal(reparsed)
			if string(want) != string(got) {
				t.Fatalf("document round-trip lost configuration:\nwant %s\ngot  %s", want, got)
			}
			after, _ := json.Marshal(plan)
			if string(before) != string(after) {
				t.Fatal("input plan mutated")
			}
			document.Metadata.Labels["owner"] = "changed"
			if plan.Stream.Metadata["rabbit-jetstream.io/label.owner"] != "team" {
				t.Fatal("document aliases plan metadata")
			}
			if document.Spec.MaxPriority != nil {
				*document.Spec.MaxPriority = 1
				if *plan.MaxPriority == 1 {
					t.Fatal("document aliases priority")
				}
			}
		})
	}
}

func TestQueueDocumentRejectsLossyPlans(t *testing.T) {
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.APIVersion = "future" },
		func(p *Plan) { p.DeclarationSubjects = nil },
		func(p *Plan) { p.Consumer.AckPolicy = "none" },
		func(p *Plan) { p.Stream.Name = "other" },
		func(p *Plan) { p.Consumer.Metadata["extra"] = "unsupported" },
		func(p *Plan) { p.Revision = "incorrect" },
		func(p *Plan) { p.Warnings = []string{"cannot preserve"} },
	} {
		queue, err := ParseQueue(strings.NewReader("apiVersion: rabbit-jetstream.io/v1alpha1\nkind: Queue\nmetadata: {name: orders}\nspec: {subjects: [orders.events], replicas: 1}"))
		if err != nil {
			t.Fatal(err)
		}
		plan, err := BuildPlan(*queue)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&plan)
		document, err := QueueDocument(plan)
		if err == nil || document != nil {
			t.Fatalf("lossy plan accepted: %+v", document)
		}
	}
}
