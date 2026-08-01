package topology

import (
	"strings"
	"testing"
)

func TestCompareClassifiesAndSortsChanges(t *testing.T) {
	current, err := ParseQueue(strings.NewReader(validQueueYAML))
	if err != nil {
		t.Fatal(err)
	}
	desired, err := ParseQueue(strings.NewReader(strings.NewReplacer(
		"storage: file", "storage: memory",
		"maxBytes: 10GiB", "maxBytes: 1GiB",
		"maxDeliver: 7", "maxDeliver: 10",
		"orders.created, orders.*", "orders.created",
	).Replace(strings.Replace(validQueueYAML, "  retention:", "  storage: file\n  retention:", 1))))
	if err != nil {
		t.Fatal(err)
	}
	diff := Compare(*current, *desired)
	if !diff.HasChanges() || len(diff.Changes) != 4 {
		t.Fatalf("unexpected diff: %#v", diff)
	}
	impacts := make(map[string]string)
	for index, change := range diff.Changes {
		impacts[change.Path] = change.Impact
		if index > 0 && diff.Changes[index-1].Path > change.Path {
			t.Fatalf("changes are not sorted: %#v", diff.Changes)
		}
	}
	if impacts["spec.storage"] != "destructive" || impacts["spec.retention.maxBytes"] != "destructive" || impacts["spec.subjects"] != "disruptive" {
		t.Fatalf("unexpected impacts: %#v", impacts)
	}
}

func TestCompareIgnoresOrderingAndDefaults(t *testing.T) {
	first, _ := ParseQueue(strings.NewReader(validQueueYAML))
	second, _ := ParseQueue(strings.NewReader(strings.Replace(validQueueYAML, "orders.created, orders.*", "orders.*, orders.created", 1)))
	if diff := Compare(*first, *second); diff.HasChanges() {
		t.Fatalf("unexpected changes: %#v", diff)
	}
}
