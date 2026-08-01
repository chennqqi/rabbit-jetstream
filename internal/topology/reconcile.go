package topology

import (
	"reflect"
	"sort"
)

type ObservedTopology struct {
	Stream   *ObservedStream   `json:"stream,omitempty"`
	Consumer *ObservedConsumer `json:"consumer,omitempty"`
}

type ObservedStream struct {
	Name        string            `json:"name"`
	Subjects    []string          `json:"subjects"`
	Storage     string            `json:"storage"`
	Replicas    int               `json:"replicas"`
	Retention   string            `json:"retention"`
	Discard     string            `json:"discard"`
	MaxAgeNanos int64             `json:"max_age_nanos"`
	MaxBytes    int64             `json:"max_bytes"`
	MaxMessages int64             `json:"max_messages"`
	Metadata    map[string]string `json:"metadata"`
}

type ObservedConsumer struct {
	Stream         string            `json:"stream"`
	Name           string            `json:"name"`
	Durable        string            `json:"durable"`
	FilterSubject  string            `json:"filter_subject"`
	FilterSubjects []string          `json:"filter_subjects"`
	Mode           string            `json:"mode"`
	DeliverPolicy  string            `json:"deliver_policy"`
	AckPolicy      string            `json:"ack_policy"`
	AckWaitNanos   int64             `json:"ack_wait_nanos"`
	MaxDeliver     int               `json:"max_deliver"`
	ReplayPolicy   string            `json:"replay_policy"`
	Metadata       map[string]string `json:"metadata"`
}

type ReconcileResult struct {
	Queue      string      `json:"queue"`
	Revision   string      `json:"revision"`
	Status     string      `json:"status"`
	Blocked    bool        `json:"blocked"`
	Operations []Operation `json:"operations"`
}

type Operation struct {
	Resource string   `json:"resource"`
	Name     string   `json:"name"`
	Action   string   `json:"action"`
	Impact   string   `json:"impact"`
	Blocked  bool     `json:"blocked"`
	Reason   string   `json:"reason,omitempty"`
	Changes  []Change `json:"changes"`
}

func Reconcile(desired Plan, observed ObservedTopology) ReconcileResult {
	result := ReconcileResult{Queue: desired.Queue, Revision: desired.Revision, Operations: []Operation{}}
	streamOperation := reconcileStream(desired.Stream, observed.Stream)
	result.Operations = append(result.Operations, streamOperation)
	consumerOperation := reconcileConsumer(desired.Consumer, observed.Consumer, streamOperation.Action == "recreate")
	result.Operations = append(result.Operations, consumerOperation)
	if desired.DeadLetter != nil {
		result.Operations = append(result.Operations, Operation{
			Resource: "dead-letter-worker", Name: desired.DeadLetter.Worker, Action: "reject",
			Impact: "unsupported", Blocked: true, Changes: []Change{},
			Reason: "DLQ advisory republisher is not implemented",
		})
	}
	for _, operation := range result.Operations {
		if operation.Blocked {
			result.Blocked = true
		}
	}
	switch {
	case result.Blocked:
		result.Status = "blocked"
	case allNoop(result.Operations):
		result.Status = "noop"
	default:
		result.Status = "ready"
	}
	return result
}

func reconcileStream(desired StreamPlan, observed *ObservedStream) Operation {
	operation := Operation{Resource: "stream", Name: desired.Name, Impact: "safe", Changes: []Change{}}
	if observed == nil {
		operation.Action = "create"
		return operation
	}
	addChange(&operation, "storage", observed.Storage, desired.Storage, "destructive")
	addChange(&operation, "subjects", sortedStrings(observed.Subjects), sortedStrings(desired.Subjects), subjectsImpact(observed.Subjects, desired.Subjects))
	addChange(&operation, "replicas", observed.Replicas, desired.Replicas, "disruptive")
	addChange(&operation, "retention", observed.Retention, desired.Retention, "destructive")
	addChange(&operation, "discard", observed.Discard, desired.Discard, "disruptive")
	addChange(&operation, "maxAgeNanos", normalizeLimit(observed.MaxAgeNanos), normalizeLimit(desired.MaxAgeNanos), limitImpact(observed.MaxAgeNanos, desired.MaxAgeNanos))
	addChange(&operation, "maxBytes", normalizeLimit(observed.MaxBytes), normalizeLimit(desired.MaxBytes), limitImpact(observed.MaxBytes, desired.MaxBytes))
	addChange(&operation, "maxMessages", normalizeLimit(observed.MaxMessages), normalizeLimit(desired.MaxMessages), limitImpact(observed.MaxMessages, desired.MaxMessages))
	addMetadataChanges(&operation, observed.Metadata, desired.Metadata)
	if len(operation.Changes) == 0 {
		operation.Action, operation.Impact = "noop", "none"
		return operation
	}
	if hasPath(operation.Changes, "storage") || hasPath(operation.Changes, "retention") {
		operation.Action, operation.Blocked = "recreate", true
		operation.Reason = "changing storage or retention requires Stream recreation"
		return operation
	}
	operation.Action = "update"
	operation.Blocked = hasImpact(operation.Changes, "destructive")
	if operation.Blocked {
		operation.Reason = "retention reduction requires explicit destructive-change approval"
	}
	return operation
}

func reconcileConsumer(desired ConsumerPlan, observed *ObservedConsumer, streamRecreate bool) Operation {
	operation := Operation{Resource: "consumer", Name: desired.Name, Impact: "safe", Changes: []Change{}}
	if observed == nil {
		operation.Action = "create"
		return operation
	}
	filters := observed.FilterSubjects
	if len(filters) == 0 && observed.FilterSubject != "" {
		filters = []string{observed.FilterSubject}
	}
	addChange(&operation, "stream", observed.Stream, desired.Stream, "destructive")
	addChange(&operation, "mode", observed.Mode, desired.Mode, "destructive")
	addChange(&operation, "filterSubjects", sortedStrings(filters), sortedStrings(desired.FilterSubjects), "disruptive")
	addChange(&operation, "deliverPolicy", observed.DeliverPolicy, desired.DeliverPolicy, "destructive")
	addChange(&operation, "ackPolicy", observed.AckPolicy, desired.AckPolicy, "disruptive")
	addChange(&operation, "ackWaitNanos", observed.AckWaitNanos, desired.AckWaitNanos, "disruptive")
	addChange(&operation, "maxDeliver", observed.MaxDeliver, desired.MaxDeliver, "disruptive")
	addChange(&operation, "replayPolicy", observed.ReplayPolicy, desired.ReplayPolicy, "destructive")
	addMetadataChanges(&operation, observed.Metadata, desired.Metadata)
	if len(operation.Changes) == 0 && !streamRecreate {
		operation.Action, operation.Impact = "noop", "none"
		return operation
	}
	if streamRecreate || hasImpact(operation.Changes, "destructive") {
		operation.Action, operation.Impact, operation.Blocked = "recreate", "destructive", true
		operation.Reason = "consumer has immutable changes or depends on Stream recreation"
		return operation
	}
	operation.Action = "update"
	return operation
}

func addChange(operation *Operation, path string, from, to any, impact string) {
	if reflect.DeepEqual(from, to) {
		return
	}
	operation.Changes = append(operation.Changes, Change{Path: path, From: valueString(from), To: valueString(to), Impact: impact})
	operation.Impact = maxImpact(operation.Impact, impact)
}

func addMetadataChanges(operation *Operation, observed, desired map[string]string) {
	keys := make([]string, 0, len(desired))
	for key := range desired {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		addChange(operation, "metadata."+key, observed[key], desired[key], "safe")
	}
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func normalizeLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func maxImpact(current, candidate string) string {
	rank := map[string]int{"none": 0, "safe": 1, "disruptive": 2, "destructive": 3, "unsupported": 4}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func hasPath(changes []Change, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

func hasImpact(changes []Change, impact string) bool {
	for _, change := range changes {
		if change.Impact == impact {
			return true
		}
	}
	return false
}

func allNoop(operations []Operation) bool {
	for _, operation := range operations {
		if operation.Action != "noop" {
			return false
		}
	}
	return true
}
