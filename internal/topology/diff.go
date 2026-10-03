package topology

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Change describes one normalized field change and its operational impact.
type Change struct {
	Path   string `json:"path" yaml:"path"`
	From   string `json:"from,omitempty" yaml:"from,omitempty"`
	To     string `json:"to,omitempty" yaml:"to,omitempty"`
	Impact string `json:"impact" yaml:"impact"`
}

type Diff struct {
	Queue   string   `json:"queue" yaml:"queue"`
	Changes []Change `json:"changes" yaml:"changes"`
}

func Compare(current, desired Queue) Diff {
	current = comparisonQueue(current)
	desired = comparisonQueue(desired)
	result := Diff{Queue: desired.Metadata.Name, Changes: []Change{}}
	add := func(path string, from, to any, impact string) {
		if reflect.DeepEqual(from, to) {
			return
		}
		result.Changes = append(result.Changes, Change{Path: path, From: valueString(from), To: valueString(to), Impact: impact})
	}
	add("metadata.name", current.Metadata.Name, desired.Metadata.Name, "destructive")
	add("metadata.labels", sortedLabels(current.Metadata.Labels), sortedLabels(desired.Metadata.Labels), "safe")
	add("spec.storage", current.Spec.Storage, desired.Spec.Storage, "destructive")
	add("spec.replicas", current.Spec.Replicas, desired.Spec.Replicas, "disruptive")
	add("spec.maxPriority", optionalPriority(current.Spec.MaxPriority), optionalPriority(desired.Spec.MaxPriority), "disruptive")
	add("spec.subjects", current.Spec.Subjects, desired.Spec.Subjects, subjectsImpact(current.Spec.Subjects, desired.Spec.Subjects))
	currentRouting, _, _ := routingPlan(current)
	desiredRouting, _, _ := routingPlan(desired)
	add("spec.bindings", current.Spec.Bindings, desired.Spec.Bindings, subjectsImpact(currentRouting, desiredRouting))
	add("spec.retention.maxAge", time.Duration(current.Spec.Retention.MaxAge), time.Duration(desired.Spec.Retention.MaxAge), limitImpact(int64(current.Spec.Retention.MaxAge), int64(desired.Spec.Retention.MaxAge)))
	add("spec.retention.maxBytes", int64(current.Spec.Retention.MaxBytes), int64(desired.Spec.Retention.MaxBytes), limitImpact(int64(current.Spec.Retention.MaxBytes), int64(desired.Spec.Retention.MaxBytes)))
	add("spec.retention.maxMessages", current.Spec.Retention.MaxMessages, desired.Spec.Retention.MaxMessages, limitImpact(current.Spec.Retention.MaxMessages, desired.Spec.Retention.MaxMessages))
	add("spec.delivery.ackWait", durationValue(current.Spec.Delivery.AckWait), durationValue(desired.Spec.Delivery.AckWait), "disruptive")
	add("spec.delivery.maxDeliver", intValue(current.Spec.Delivery.MaxDeliver), intValue(desired.Spec.Delivery.MaxDeliver), "disruptive")
	add("spec.deadLetter.queue", deadLetterName(current.Spec.DeadLetter), deadLetterName(desired.Spec.DeadLetter), "disruptive")
	sort.Slice(result.Changes, func(i, j int) bool { return result.Changes[i].Path < result.Changes[j].Path })
	return result
}

func (d Diff) HasChanges() bool { return len(d.Changes) > 0 }

// Default sorts collections in place. Copy every sortable slice so comparison
// cannot change a caller's draft, and normalize nil/empty collections equally.
func comparisonQueue(queue Queue) Queue {
	queue.Spec.Subjects = append([]string{}, queue.Spec.Subjects...)
	bindings := make([]Binding, len(queue.Spec.Bindings))
	for index, binding := range queue.Spec.Bindings {
		bindings[index] = binding
		bindings[index].Keys = append([]string{}, binding.Keys...)
	}
	queue.Spec.Bindings = bindings
	queue.Default()
	return queue
}

func optionalPriority(value *int) string {
	if value == nil {
		return "omitted"
	}
	return fmt.Sprint(*value)
}

func subjectsImpact(current, desired []string) string {
	desiredSet := make(map[string]struct{}, len(desired))
	for _, subject := range desired {
		desiredSet[subject] = struct{}{}
	}
	for _, subject := range current {
		if _, exists := desiredSet[subject]; !exists {
			return "disruptive"
		}
	}
	return "safe"
}

func limitImpact(current, desired int64) string {
	if desired > 0 && (current == 0 || desired < current) {
		return "destructive"
	}
	return "safe"
}

func deadLetterName(policy *DeadLetterPolicy) string {
	if policy == nil {
		return ""
	}
	return policy.Queue
}

func sortedLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "{}"
	}
	// JSON sorts map keys and escapes delimiters. Comma-joined key=value pairs
	// conflate distinct maps, e.g. {a: "b,c=d"} and {a: "b", c: "d"}.
	encoded, _ := json.Marshal(labels) // A map of strings cannot fail encoding.
	return string(encoded)
}

func valueString(value any) string {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, ",")
	case time.Duration:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func durationValue(value *Duration) time.Duration {
	if value == nil {
		return 0
	}
	return time.Duration(*value)
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
