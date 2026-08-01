package topology

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const PlanAPIVersion = "rabbit-jetstream.io/plan/v1alpha1"

type Plan struct {
	APIVersion   string          `json:"apiVersion"`
	Queue        string          `json:"queue"`
	Revision     string          `json:"revision"`
	Stream       StreamPlan      `json:"stream"`
	Consumer     ConsumerPlan    `json:"consumer"`
	DeadLetter   *DeadLetterPlan `json:"deadLetter,omitempty"`
	Dependencies []string        `json:"dependencies"`
	Warnings     []string        `json:"warnings"`
}

type StreamPlan struct {
	Name        string            `json:"name"`
	Subjects    []string          `json:"subjects"`
	Storage     string            `json:"storage"`
	Replicas    int               `json:"replicas"`
	Retention   string            `json:"retention"`
	Discard     string            `json:"discard"`
	MaxAgeNanos int64             `json:"maxAgeNanos"`
	MaxBytes    int64             `json:"maxBytes"`
	MaxMessages int64             `json:"maxMessages"`
	Metadata    map[string]string `json:"metadata"`
}

type ConsumerPlan struct {
	Name           string            `json:"name"`
	Stream         string            `json:"stream"`
	Mode           string            `json:"mode"`
	FilterSubjects []string          `json:"filterSubjects"`
	DeliverPolicy  string            `json:"deliverPolicy"`
	AckPolicy      string            `json:"ackPolicy"`
	AckWaitNanos   int64             `json:"ackWaitNanos"`
	MaxDeliver     int               `json:"maxDeliver"`
	ReplayPolicy   string            `json:"replayPolicy"`
	Metadata       map[string]string `json:"metadata"`
}

type DeadLetterPlan struct {
	Queue     string `json:"queue"`
	Stream    string `json:"stream"`
	Worker    string `json:"worker"`
	Mechanism string `json:"mechanism"`
}

func BuildPlan(queue Queue) (Plan, error) {
	queue.Default()
	if err := queue.Validate(); err != nil {
		return Plan{}, err
	}
	revision, err := queueRevision(queue)
	if err != nil {
		return Plan{}, err
	}
	streamName := StreamName(queue.Metadata.Name)
	consumerName := ConsumerName(queue.Metadata.Name)
	metadata := resourceMetadata(queue, revision)
	plan := Plan{
		APIVersion: PlanAPIVersion, Queue: queue.Metadata.Name, Revision: revision,
		Stream: StreamPlan{
			Name: streamName, Subjects: append([]string(nil), queue.Spec.Subjects...),
			Storage: queue.Spec.Storage, Replicas: queue.Spec.Replicas,
			Retention: "workqueue", Discard: "old",
			MaxAgeNanos: int64(queue.Spec.Retention.MaxAge), MaxBytes: int64(queue.Spec.Retention.MaxBytes),
			MaxMessages: queue.Spec.Retention.MaxMessages, Metadata: cloneMap(metadata),
		},
		Consumer: ConsumerPlan{
			Name: consumerName, Stream: streamName, Mode: "pull", FilterSubjects: append([]string(nil), queue.Spec.Subjects...),
			DeliverPolicy: "all",
			AckPolicy:     "explicit", AckWaitNanos: int64(durationValue(queue.Spec.Delivery.AckWait)),
			MaxDeliver: intValue(queue.Spec.Delivery.MaxDeliver), ReplayPolicy: "instant", Metadata: cloneMap(metadata),
		},
		Dependencies: []string{}, Warnings: []string{},
	}
	if queue.Spec.DeadLetter != nil {
		plan.Dependencies = append(plan.Dependencies, queue.Spec.DeadLetter.Queue)
		plan.DeadLetter = &DeadLetterPlan{
			Queue: queue.Spec.DeadLetter.Queue, Stream: StreamName(queue.Spec.DeadLetter.Queue),
			Worker: DeadLetterWorkerName(queue.Metadata.Name), Mechanism: "advisory-republish",
		}
		plan.Warnings = append(plan.Warnings, "DLQ transfer requires the server-side advisory republisher; JetStream MaxDeliver alone does not move messages")
	}
	sort.Strings(plan.Dependencies)
	return plan, nil
}

func StreamName(queueName string) string { return "RJSQ_" + queueName }

func ConsumerName(queueName string) string { return "RJSQC_" + queueName }

func DeadLetterWorkerName(queueName string) string { return "RJSDLQ_" + queueName }

func queueRevision(queue Queue) (string, error) {
	encoded, err := json.Marshal(queue)
	if err != nil {
		return "", fmt.Errorf("encode Queue revision: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:16]), nil
}

func resourceMetadata(queue Queue, revision string) map[string]string {
	metadata := map[string]string{
		"rabbit-jetstream.io/api-version": queue.APIVersion,
		"rabbit-jetstream.io/queue":       queue.Metadata.Name,
		"rabbit-jetstream.io/revision":    revision,
	}
	for key, value := range queue.Metadata.Labels {
		metadata["rabbit-jetstream.io/label."+key] = value
	}
	return metadata
}

func cloneMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
