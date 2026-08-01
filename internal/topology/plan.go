package topology

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	Routing      []RoutingPlan   `json:"routing,omitempty"`
}

type RoutingPlan struct {
	Exchange string   `json:"exchange"`
	Type     string   `json:"type"`
	Keys     []string `json:"keys"`
	Subjects []string `json:"subjects"`
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
	subjects, routing, err := routingPlan(queue)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{
		APIVersion: PlanAPIVersion, Queue: queue.Metadata.Name, Revision: revision,
		Stream: StreamPlan{
			Name: streamName, Subjects: subjects,
			Storage: queue.Spec.Storage, Replicas: queue.Spec.Replicas,
			Retention: "workqueue", Discard: "old",
			MaxAgeNanos: int64(queue.Spec.Retention.MaxAge), MaxBytes: int64(queue.Spec.Retention.MaxBytes),
			MaxMessages: queue.Spec.Retention.MaxMessages, Metadata: cloneMap(metadata),
		},
		Consumer: ConsumerPlan{
			Name: consumerName, Stream: streamName, Mode: "pull", FilterSubjects: append([]string(nil), subjects...),
			DeliverPolicy: "all",
			AckPolicy:     "explicit", AckWaitNanos: int64(durationValue(queue.Spec.Delivery.AckWait)),
			MaxDeliver: intValue(queue.Spec.Delivery.MaxDeliver), ReplayPolicy: "instant", Metadata: cloneMap(metadata),
		},
		Dependencies: []string{}, Warnings: []string{}, Routing: routing,
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

func routingPlan(queue Queue) ([]string, []RoutingPlan, error) {
	if len(queue.Spec.Bindings) == 0 {
		return append([]string(nil), queue.Spec.Subjects...), []RoutingPlan{}, nil
	}
	all := make(map[string]struct{})
	routing := make([]RoutingPlan, 0, len(queue.Spec.Bindings))
	for _, binding := range queue.Spec.Bindings {
		subjects, err := BindingSubjects(queue.Metadata.Name, binding)
		if err != nil {
			return nil, nil, err
		}
		for _, subject := range subjects {
			all[subject] = struct{}{}
		}
		routing = append(routing, RoutingPlan{Exchange: binding.Exchange, Type: binding.Type, Keys: append([]string(nil), binding.Keys...), Subjects: subjects})
	}
	subjects := make([]string, 0, len(all))
	for subject := range all {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)
	return subjects, routing, nil
}

func BindingSubjects(queueName string, binding Binding) ([]string, error) {
	if !queueNamePattern.MatchString(queueName) {
		return nil, fmt.Errorf("invalid queue name %q", queueName)
	}
	base := "rjs.q." + queueName + ".x." + binding.Exchange + "." + binding.Type
	var result []string
	switch binding.Type {
	case "direct":
		for _, key := range binding.Keys {
			result = append(result, base+"."+key)
		}
	case "topic":
		for _, key := range binding.Keys {
			tokens := strings.Split(key, ".")
			if tokens[len(tokens)-1] != "#" {
				result = append(result, base+"."+key)
				continue
			}
			stem := strings.Join(tokens[:len(tokens)-1], ".")
			if stem == "" {
				result = append(result, base, base+".>")
			} else {
				result = append(result, base+"."+stem, base+"."+stem+".>")
			}
		}
	case "fanout":
		result = append(result, base)
	default:
		return nil, fmt.Errorf("unsupported binding type %q", binding.Type)
	}
	set := make(map[string]struct{})
	unique := result[:0]
	for _, subject := range result {
		if _, exists := set[subject]; !exists {
			set[subject] = struct{}{}
			unique = append(unique, subject)
		}
	}
	sort.Strings(unique)
	return unique, nil
}

// QueuePublishSubject returns the queue-scoped target used after an SDK or
// protocol gateway has resolved exchange bindings. Queue scoping prevents
// subjects from overlapping across JetStream work-queue Streams.
func QueuePublishSubject(queueName, exchange, exchangeType, routingKey string) (string, error) {
	if !queueNamePattern.MatchString(queueName) {
		return "", fmt.Errorf("invalid queue name")
	}
	if !queueNamePattern.MatchString(exchange) {
		return "", fmt.Errorf("invalid exchange name")
	}
	base := "rjs.q." + queueName + ".x." + exchange + "." + exchangeType
	switch exchangeType {
	case "fanout":
		if routingKey != "" {
			return "", fmt.Errorf("fanout routing key must be empty")
		}
		return base, nil
	case "direct", "topic":
		if routingKey == "" {
			if exchangeType == "topic" {
				return base, nil
			}
			return "", fmt.Errorf("direct routing key cannot be empty")
		}
		if err := validateRoutingKey(routingKey, false); err != nil {
			return "", err
		}
		return base + "." + routingKey, nil
	default:
		return "", fmt.Errorf("unsupported exchange type %q", exchangeType)
	}
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
