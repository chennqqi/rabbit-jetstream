package topology

import (
	"reflect"
	"testing"
)

func TestComparePriorityPresenceAndValue(t *testing.T) {
	zero, two, max := 0, 2, 255
	for _, test := range []struct {
		name     string
		from, to *int
		changed  bool
	}{
		{"omitted", nil, nil, false}, {"enable-zero", nil, &zero, true}, {"disable-zero", &zero, nil, true},
		{"increase", &zero, &two, true}, {"decrease", &max, &two, true}, {"same-value", &two, &two, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := Queue{Metadata: Metadata{Name: "q"}, Spec: QueueSpec{MaxPriority: test.from}}
			desired := Queue{Metadata: Metadata{Name: "q"}, Spec: QueueSpec{MaxPriority: test.to}}
			diff := Compare(current, desired)
			if diff.HasChanges() != test.changed {
				t.Fatalf("unexpected diff: %#v", diff)
			}
			if test.changed && (len(diff.Changes) != 1 || diff.Changes[0].Path != "spec.maxPriority" || diff.Changes[0].Impact != "disruptive" || diff.Changes[0].From == diff.Changes[0].To) {
				t.Fatalf("priority difference lost: %#v", diff)
			}
		})
	}
}

func TestCompareLabelDelimiterCollision(t *testing.T) {
	current := Queue{Metadata: Metadata{Name: "q", Labels: map[string]string{"a": "b,c=d"}}}
	desired := Queue{Metadata: Metadata{Name: "q", Labels: map[string]string{"a": "b", "c": "d"}}}
	diff := Compare(current, desired)
	if len(diff.Changes) != 1 || diff.Changes[0].Path != "metadata.labels" || diff.Changes[0].From == diff.Changes[0].To {
		t.Fatalf("labels were conflated: %#v", diff)
	}
	if diff := Compare(Queue{}, Queue{Metadata: Metadata{Labels: map[string]string{}}}); diff.HasChanges() {
		t.Fatalf("nil and empty labels differ: %#v", diff)
	}
}

func TestCompareDoesNotMutateInputCollections(t *testing.T) {
	makeQueue := func() Queue {
		return Queue{Metadata: Metadata{Name: "q"}, Spec: QueueSpec{Subjects: []string{"z", "a"}, Bindings: []Binding{{Exchange: "z", Type: "topic", Keys: []string{"z", "a"}}, {Exchange: "a", Type: "fanout"}}}}
	}
	current, desired := makeQueue(), makeQueue()
	beforeCurrent, beforeDesired := makeQueue(), makeQueue()
	Compare(current, desired)
	if !reflect.DeepEqual(current, beforeCurrent) || !reflect.DeepEqual(desired, beforeDesired) {
		t.Fatalf("comparison mutated inputs: %#v %#v", current, desired)
	}
	if diff := Compare(Queue{}, Queue{Spec: QueueSpec{Subjects: []string{}, Bindings: []Binding{}}}); diff.HasChanges() {
		t.Fatalf("empty collections differ: %#v", diff)
	}
}
