package topology

import (
	"strings"
	"testing"
	"time"
)

const validQueueYAML = `apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata:
  name: orders
  labels:
    team: commerce
spec:
  subjects: [orders.created, orders.*]
  replicas: 3
  retention:
    maxAge: 168h
    maxBytes: 10GiB
    maxMessages: 1000000
  delivery:
    maxDeliver: 7
  deadLetter:
    queue: orders_dlq
`

func TestParseQueueDefaultsAndNormalizes(t *testing.T) {
	queue, err := ParseQueue(strings.NewReader(validQueueYAML))
	if err != nil {
		t.Fatal(err)
	}
	if queue.Spec.Storage != "file" || queue.Spec.Delivery.AckWait == nil || time.Duration(*queue.Spec.Delivery.AckWait) != 30*time.Second {
		t.Fatalf("defaults not applied: %#v", queue.Spec)
	}
	if queue.Spec.Retention.MaxBytes != ByteSize(10<<30) {
		t.Fatalf("max bytes = %d", queue.Spec.Retention.MaxBytes)
	}
	if queue.Spec.Subjects[0] != "orders.*" || queue.Spec.Subjects[1] != "orders.created" {
		t.Fatalf("subjects not normalized: %#v", queue.Spec.Subjects)
	}
}

func TestParseQueueRejectsUnknownAndMultipleDocuments(t *testing.T) {
	tests := []string{
		strings.Replace(validQueueYAML, "  replicas: 3", "  replicas: 3\n  unexpected: true", 1),
		validQueueYAML + "---\n" + validQueueYAML,
	}
	for _, input := range tests {
		if _, err := ParseQueue(strings.NewReader(input)); err == nil {
			t.Fatalf("expected error for input:\n%s", input)
		}
	}
}

func TestQueueValidationFailures(t *testing.T) {
	tests := []struct {
		name    string
		replace string
		with    string
		want    string
	}{
		{"version", QueueAPIVersion, "v1", "apiVersion"},
		{"name", "name: orders", "name: orders.prod", "metadata.name"},
		{"replicas", "replicas: 3", "replicas: 2", "spec.replicas"},
		{"partial wildcard", "orders.created", "orders.cre*", "partial token"},
		{"self dlq", "queue: orders_dlq", "queue: orders", "cannot reference itself"},
		{"negative retention", "maxMessages: 1000000", "maxMessages: -1", "cannot be negative"},
		{"zero max deliver", "maxDeliver: 7", "maxDeliver: 0", "maxDeliver must be at least 1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := strings.Replace(validQueueYAML, test.replace, test.with, 1)
			_, err := ParseQueue(strings.NewReader(input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseByteSize(t *testing.T) {
	tests := map[string]int64{"1KiB": 1024, "2MB": 2_000_000, "99": 99, "0B": 0}
	for input, expected := range tests {
		actual, err := parseByteSize(input)
		if err != nil || actual != expected {
			t.Fatalf("parseByteSize(%q) = %d, %v", input, actual, err)
		}
	}
	for _, input := range []string{"-1", "1.5GiB", "invalid", "9223372036854775807GiB"} {
		if _, err := parseByteSize(input); err == nil {
			t.Fatalf("parseByteSize(%q) succeeded", input)
		}
	}
}
