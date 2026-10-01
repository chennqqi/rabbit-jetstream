package topology

import (
	"errors"
	"strings"
	"testing"
)

func TestValidationIssuesUseSubmittedOrder(t *testing.T) {
	for _, tc := range []struct{ spec, path, code string }{
		{`"subjects":["z.ok","a..bad"]`, "/spec/subjects/1", "invalid_subject"},
		{`"subjects":["z.ok","z.ok","a.ok"]`, "/spec/subjects/1", "duplicate"},
		{`"bindings":[{"exchange":"z","type":"fanout"},{"exchange":"a","type":"direct","keys":["z","a.*"]}]`, "/spec/bindings/1/keys/1", "invalid_routing_key"},
		{`"bindings":[{"exchange":"z","type":"topic","keys":["z","z","a"]}]`, "/spec/bindings/0/keys/1", "duplicate"},
	} {
		t.Run(tc.path+tc.code, func(t *testing.T) {
			_, err := ParseQueue(strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"q"},"spec":{"replicas":1,` + tc.spec + `}}`))
			var validation *ValidationError
			if !errors.As(err, &validation) || len(validation.Issues) != 1 {
				t.Fatalf("expected one semantic issue: %v", err)
			}
			if issue := validation.Issues[0]; issue.Path != tc.path || issue.Code != tc.code || !strings.Contains(err.Error(), issue.Message) {
				t.Fatalf("unexpected issue: %+v", issue)
			}
		})
	}
}

func TestValidationDecodeFailureHasNoGuessedPath(t *testing.T) {
	_, err := ParseQueue(strings.NewReader(`{"spec":{"replicas":"bad"}}`))
	var validation *ValidationError
	if err == nil || errors.As(err, &validation) {
		t.Fatalf("unexpected semantic error: %v", err)
	}
}

func TestValidationScalarAndGroupPaths(t *testing.T) {
	for _, tc := range []struct {
		path   string
		change func(*Queue)
	}{
		{"/apiVersion", func(q *Queue) { q.APIVersion = "future" }},
		{"/kind", func(q *Queue) { q.Kind = "other" }},
		{"/metadata/name", func(q *Queue) { q.Metadata.Name = "bad.name" }},
		{"/spec", func(q *Queue) { q.Spec.Subjects = nil }},
		{"/spec", func(q *Queue) { q.Spec.Bindings = []Binding{{Exchange: "e", Type: "fanout"}} }},
		{"/spec/replicas", func(q *Queue) { q.Spec.Replicas = 2 }},
		{"/spec/storage", func(q *Queue) { q.Spec.Storage = "other" }},
		{"/spec/retention", func(q *Queue) { q.Spec.Retention.MaxMessages = -1 }},
		{"/spec/maxPriority", func(q *Queue) { v := 256; q.Spec.MaxPriority = &v }},
		{"/spec/delivery/ackWait", func(q *Queue) { v := Duration(0); q.Spec.Delivery.AckWait = &v }},
		{"/spec/delivery/maxDeliver", func(q *Queue) { v := 0; q.Spec.Delivery.MaxDeliver = &v }},
		{"/spec/deadLetter/queue", func(q *Queue) { q.Spec.DeadLetter = &DeadLetterPolicy{Queue: "q"} }},
		{"/spec/deadLetter/queue", func(q *Queue) { q.Spec.DeadLetter = &DeadLetterPolicy{Queue: "bad.name"} }},
	} {
		q := Queue{APIVersion: QueueAPIVersion, Kind: QueueKind, Metadata: Metadata{Name: "q"}, Spec: QueueSpec{Subjects: []string{"q"}, Replicas: 1}}
		q.Default()
		tc.change(&q)
		var validation *ValidationError
		if err := q.Validate(); !errors.As(err, &validation) || len(validation.Issues) != 1 || validation.Issues[0].Path != tc.path {
			t.Fatalf("%s: %v", tc.path, err)
		}
	}
}
