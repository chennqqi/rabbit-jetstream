package topology

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Versioned Queue document identifiers.
const (
	QueueAPIVersion = "rabbit-jetstream.io/v1alpha1"
	QueueKind       = "Queue"
)

var queueNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ValidQueueName(name string) bool { return queueNamePattern.MatchString(name) }

type Queue struct {
	APIVersion string    `yaml:"apiVersion" json:"apiVersion"`
	Kind       string    `yaml:"kind" json:"kind"`
	Metadata   Metadata  `yaml:"metadata" json:"metadata"`
	Spec       QueueSpec `yaml:"spec" json:"spec"`
}

type Metadata struct {
	Name   string            `yaml:"name" json:"name"`
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type QueueSpec struct {
	Subjects    []string          `yaml:"subjects" json:"subjects"`
	Bindings    []Binding         `yaml:"bindings,omitempty" json:"bindings,omitempty"`
	MaxPriority *int              `yaml:"maxPriority,omitempty" json:"maxPriority,omitempty"`
	Replicas    int               `yaml:"replicas" json:"replicas"`
	Storage     string            `yaml:"storage" json:"storage"`
	Retention   RetentionPolicy   `yaml:"retention,omitempty" json:"retention,omitempty"`
	Delivery    DeliveryPolicy    `yaml:"delivery,omitempty" json:"delivery,omitempty"`
	DeadLetter  *DeadLetterPolicy `yaml:"deadLetter,omitempty" json:"deadLetter,omitempty"`
}

type Binding struct {
	Exchange string   `yaml:"exchange" json:"exchange"`
	Type     string   `yaml:"type" json:"type"`
	Keys     []string `yaml:"keys,omitempty" json:"keys,omitempty"`
}

type RetentionPolicy struct {
	MaxAge      Duration `yaml:"maxAge,omitempty" json:"maxAge,omitempty"`
	MaxBytes    ByteSize `yaml:"maxBytes,omitempty" json:"maxBytes,omitempty"`
	MaxMessages int64    `yaml:"maxMessages,omitempty" json:"maxMessages,omitempty"`
}

type DeliveryPolicy struct {
	AckWait    *Duration `yaml:"ackWait,omitempty" json:"ackWait,omitempty"`
	MaxDeliver *int      `yaml:"maxDeliver,omitempty" json:"maxDeliver,omitempty"`
}

type DeadLetterPolicy struct {
	Queue string `yaml:"queue" json:"queue"`
}

type Duration time.Duration

func (d *Duration) UnmarshalText(value []byte) error {
	parsed, err := time.ParseDuration(string(value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(time.Duration(d).String()), nil }

type ByteSize int64

func (b *ByteSize) UnmarshalText(value []byte) error {
	parsed, err := parseByteSize(string(value))
	if err != nil {
		return err
	}
	*b = ByteSize(parsed)
	return nil
}

func (b ByteSize) MarshalText() ([]byte, error) { return []byte(strconv.FormatInt(int64(b), 10)), nil }

func ParseQueue(reader io.Reader) (*Queue, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var queue Queue
	if err := decoder.Decode(&queue); err != nil {
		return nil, fmt.Errorf("decode queue: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("decode queue: multiple YAML documents are not allowed")
		}
		return nil, fmt.Errorf("decode queue: %w", err)
	}
	queue.Default()
	if err := queue.Validate(); err != nil {
		return nil, err
	}
	return &queue, nil
}

func (q *Queue) Default() {
	if q.Spec.Storage == "" {
		q.Spec.Storage = "file"
	}
	if q.Spec.Delivery.AckWait == nil {
		value := Duration(30 * time.Second)
		q.Spec.Delivery.AckWait = &value
	}
	if q.Spec.Delivery.MaxDeliver == nil {
		value := 5
		q.Spec.Delivery.MaxDeliver = &value
	}
	sort.Strings(q.Spec.Subjects)
	for index := range q.Spec.Bindings {
		sort.Strings(q.Spec.Bindings[index].Keys)
	}
	sort.Slice(q.Spec.Bindings, func(i, j int) bool {
		left, right := q.Spec.Bindings[i], q.Spec.Bindings[j]
		if left.Exchange != right.Exchange {
			return left.Exchange < right.Exchange
		}
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		return strings.Join(left.Keys, "\x00") < strings.Join(right.Keys, "\x00")
	})
}

func (q Queue) Validate() error {
	var problems []string
	if q.APIVersion != QueueAPIVersion {
		problems = append(problems, "apiVersion must be "+QueueAPIVersion)
	}
	if q.Kind != QueueKind {
		problems = append(problems, "kind must be Queue")
	}
	if !queueNamePattern.MatchString(q.Metadata.Name) {
		problems = append(problems, "metadata.name must contain only letters, digits, '_' or '-'")
	}
	if len(q.Spec.Subjects) == 0 && len(q.Spec.Bindings) == 0 {
		problems = append(problems, "spec.subjects or spec.bindings must contain at least one entry")
	}
	if len(q.Spec.Subjects) > 0 && len(q.Spec.Bindings) > 0 {
		problems = append(problems, "spec.subjects and spec.bindings are mutually exclusive")
	}
	for index, binding := range q.Spec.Bindings {
		prefix := fmt.Sprintf("spec.bindings[%d]", index)
		if !queueNamePattern.MatchString(binding.Exchange) {
			problems = append(problems, prefix+".exchange is invalid")
		}
		switch binding.Type {
		case "direct":
			if len(binding.Keys) == 0 {
				problems = append(problems, prefix+".keys must not be empty for direct")
			}
			for _, key := range binding.Keys {
				if err := validateRoutingKey(key, false); err != nil {
					problems = append(problems, prefix+".keys: "+err.Error())
				}
			}
		case "topic":
			if len(binding.Keys) == 0 {
				problems = append(problems, prefix+".keys must not be empty for topic")
			}
			for _, key := range binding.Keys {
				if err := validateRoutingKey(key, true); err != nil {
					problems = append(problems, prefix+".keys: "+err.Error())
				}
			}
		case "fanout":
			if len(binding.Keys) != 0 {
				problems = append(problems, prefix+".keys must be empty for fanout")
			}
		default:
			problems = append(problems, prefix+".type must be direct, topic, or fanout")
		}
		bindingKeys := make(map[string]struct{}, len(binding.Keys))
		for _, key := range binding.Keys {
			if _, exists := bindingKeys[key]; exists {
				problems = append(problems, prefix+".keys contains duplicate "+key)
			}
			bindingKeys[key] = struct{}{}
		}
	}
	seen := make(map[string]struct{})
	for _, subject := range q.Spec.Subjects {
		if err := validateSubject(subject); err != nil {
			problems = append(problems, "spec.subjects: "+err.Error())
		}
		if _, exists := seen[subject]; exists {
			problems = append(problems, "spec.subjects contains duplicate "+subject)
		}
		seen[subject] = struct{}{}
	}
	if q.Spec.Replicas != 1 && q.Spec.Replicas != 3 && q.Spec.Replicas != 5 {
		problems = append(problems, "spec.replicas must be 1, 3, or 5")
	}
	if q.Spec.Storage != "file" && q.Spec.Storage != "memory" {
		problems = append(problems, "spec.storage must be file or memory")
	}
	if q.Spec.Retention.MaxAge < 0 || q.Spec.Retention.MaxBytes < 0 || q.Spec.Retention.MaxMessages < 0 {
		problems = append(problems, "spec.retention limits cannot be negative")
	}
	if q.Spec.MaxPriority != nil && (*q.Spec.MaxPriority < MinimumPriority || *q.Spec.MaxPriority > MaximumPriority) {
		problems = append(problems, fmt.Sprintf("spec.maxPriority must be between %d and %d", MinimumPriority, MaximumPriority))
	}
	if q.Spec.Delivery.AckWait == nil || *q.Spec.Delivery.AckWait <= 0 {
		problems = append(problems, "spec.delivery.ackWait must be positive")
	}
	if q.Spec.Delivery.MaxDeliver == nil || *q.Spec.Delivery.MaxDeliver < 1 {
		problems = append(problems, "spec.delivery.maxDeliver must be at least 1")
	}
	if q.Spec.DeadLetter != nil {
		if !queueNamePattern.MatchString(q.Spec.DeadLetter.Queue) {
			problems = append(problems, "spec.deadLetter.queue is invalid")
		} else if q.Spec.DeadLetter.Queue == q.Metadata.Name {
			problems = append(problems, "spec.deadLetter.queue cannot reference itself")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid Queue: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateRoutingKey(value string, pattern bool) error {
	if value == "" {
		return errors.New("routing key cannot be empty")
	}
	tokens := strings.Split(value, ".")
	for index, token := range tokens {
		if token == "" || strings.ContainsAny(token, " \t\r\n>") {
			return fmt.Errorf("routing key %q has an invalid token", value)
		}
		if token == "*" && pattern {
			continue
		}
		if token == "#" && pattern && index == len(tokens)-1 {
			continue
		}
		if strings.ContainsAny(token, "*#") {
			return fmt.Errorf("routing key %q has an invalid wildcard", value)
		}
	}
	return nil
}

func validateSubject(subject string) error {
	if subject == "" || strings.ContainsAny(subject, " \t\r\n") {
		return fmt.Errorf("subject %q is empty or contains whitespace", subject)
	}
	tokens := strings.Split(subject, ".")
	for index, token := range tokens {
		if token == "" {
			return fmt.Errorf("subject %q contains an empty token", subject)
		}
		if strings.Contains(token, ">") && (token != ">" || index != len(tokens)-1) {
			return fmt.Errorf("subject %q uses '>' outside the final token", subject)
		}
		if strings.Contains(token, "*") && token != "*" {
			return fmt.Errorf("subject %q uses '*' as a partial token", subject)
		}
	}
	return nil
}

func parseByteSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("byte size is empty")
	}
	units := []struct {
		suffix string
		factor int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1_000_000_000}, {"MB", 1_000_000}, {"KB", 1_000}, {"B", 1}}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			parsed, err := strconv.ParseInt(number, 10, 64)
			if err != nil || parsed < 0 || (parsed > 0 && parsed > (1<<63-1)/unit.factor) {
				return 0, fmt.Errorf("invalid byte size %q", value)
			}
			return parsed * unit.factor, nil
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid byte size %q", value)
	}
	return parsed, nil
}
