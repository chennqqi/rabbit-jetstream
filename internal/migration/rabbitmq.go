package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

type RabbitDefinitions struct {
	Queues    []RabbitQueue    `json:"queues"`
	Exchanges []RabbitExchange `json:"exchanges"`
	Bindings  []RabbitBinding  `json:"bindings"`
	Policies  []RabbitPolicy   `json:"policies"`
}

type RabbitQueue struct {
	Name       string         `json:"name"`
	VHost      string         `json:"vhost"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Arguments  map[string]any `json:"arguments"`
}

type RabbitExchange struct {
	Name       string         `json:"name"`
	VHost      string         `json:"vhost"`
	Type       string         `json:"type"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Internal   bool           `json:"internal"`
	Arguments  map[string]any `json:"arguments"`
}

type RabbitBinding struct {
	Source          string         `json:"source"`
	VHost           string         `json:"vhost"`
	Destination     string         `json:"destination"`
	DestinationType string         `json:"destination_type"`
	RoutingKey      string         `json:"routing_key"`
	Arguments       map[string]any `json:"arguments"`
}

type RabbitPolicy struct {
	Name  string `json:"name"`
	VHost string `json:"vhost"`
}

type Issue struct {
	Severity string `json:"severity"`
	Resource string `json:"resource"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type Report struct {
	Schema     string  `json:"schema"`
	VHost      string  `json:"vhost"`
	Compatible bool    `json:"compatible"`
	Queues     int     `json:"queues"`
	Converted  int     `json:"converted"`
	Issues     []Issue `json:"issues"`
}

type Result struct {
	Queues []topology.Queue
	Report Report
}

func ConvertRabbitMQ(reader io.Reader, vhost string, replicas int) (Result, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var definitions RabbitDefinitions
	if err := decoder.Decode(&definitions); err != nil {
		return Result{}, fmt.Errorf("decode RabbitMQ definitions: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Result{}, errors.New("decode RabbitMQ definitions: trailing JSON value")
	}
	if vhost == "" {
		vhost = "/"
	}
	result := Result{Report: Report{Schema: "rabbit-jetstream.io/rabbitmq-migration/v1alpha1", VHost: vhost, Compatible: true, Issues: []Issue{}}}
	if replicas != 1 && replicas != 3 && replicas != 5 {
		return Result{}, errors.New("replicas must be 1, 3, or 5")
	}
	exchanges := make(map[string]RabbitExchange)
	for _, exchange := range definitions.Exchanges {
		if exchange.VHost == vhost {
			exchanges[exchange.Name] = exchange
		}
	}
	bindingsByQueue := make(map[string][]RabbitBinding)
	for _, binding := range definitions.Bindings {
		if binding.VHost != vhost {
			continue
		}
		if binding.DestinationType != "queue" {
			result.issue("error", "binding:"+binding.Source+"->"+binding.Destination, "exchange_binding_unsupported", "exchange-to-exchange bindings cannot be mapped safely")
			continue
		}
		bindingsByQueue[binding.Destination] = append(bindingsByQueue[binding.Destination], binding)
	}
	for _, policy := range definitions.Policies {
		if policy.VHost == vhost {
			result.issue("error", "policy:"+policy.Name, "policies_unsupported", "RabbitMQ policies may override queue arguments; flatten policies before conversion")
		}
	}
	for _, source := range definitions.Queues {
		if source.VHost != vhost {
			continue
		}
		result.Report.Queues++
		queue, issues := convertQueue(source, bindingsByQueue[source.Name], exchanges, bindingsByQueue, replicas)
		result.Report.Issues = append(result.Report.Issues, issues...)
		if queue != nil {
			result.Queues = append(result.Queues, *queue)
		}
	}
	result.pruneInvalidDependencies()
	result.Report.Converted = len(result.Queues)
	if result.Report.Queues == 0 {
		result.issue("error", "vhost:"+vhost, "no_queues", "definitions contain no Queues for the selected virtual host")
	}
	sort.Slice(result.Queues, func(i, j int) bool { return result.Queues[i].Metadata.Name < result.Queues[j].Metadata.Name })
	sort.Slice(result.Report.Issues, func(i, j int) bool {
		left, right := result.Report.Issues[i], result.Report.Issues[j]
		if left.Resource != right.Resource {
			return left.Resource < right.Resource
		}
		return left.Code < right.Code
	})
	for _, issue := range result.Report.Issues {
		if issue.Severity == "error" {
			result.Report.Compatible = false
			break
		}
	}
	return result, nil
}

func convertQueue(source RabbitQueue, bindings []RabbitBinding, exchanges map[string]RabbitExchange, allBindings map[string][]RabbitBinding, replicas int) (*topology.Queue, []Issue) {
	resource := "queue:" + source.Name
	issues := make([]Issue, 0)
	errorIssue := func(code, message string) {
		issues = append(issues, Issue{Severity: "error", Resource: resource, Code: code, Message: message})
	}
	warn := func(code, message string) {
		issues = append(issues, Issue{Severity: "warning", Resource: resource, Code: code, Message: message})
	}
	if !topology.ValidQueueName(source.Name) {
		errorIssue("invalid_name", "queue name cannot be represented by the Queue schema")
	}
	if !source.Durable || source.AutoDelete {
		errorIssue("transient_queue_unsupported", "non-durable or auto-delete queues have no production-safe mapping")
	}
	queue := &topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind, Metadata: topology.Metadata{Name: source.Name, Labels: map[string]string{"migration.rabbitmq.io/vhost": source.VHost}}, Spec: topology.QueueSpec{Replicas: replicas, Storage: "file"}}
	grouped := make(map[string]*topology.Binding)
	for _, binding := range bindings {
		exchange, exists := exchanges[binding.Source]
		if !exists && binding.Source != "" {
			errorIssue("exchange_missing", "binding source exchange "+binding.Source+" is not present in definitions")
			continue
		}
		if binding.Source == "" {
			warn("default_exchange", "default exchange binding is represented by native queue ingress")
			continue
		}
		if !exchange.Durable || exchange.AutoDelete || exchange.Internal {
			errorIssue("exchange_lifecycle_unsupported", "exchange "+exchange.Name+" is transient, auto-delete, or internal")
			continue
		}
		if exchange.Type != "direct" && exchange.Type != "topic" && exchange.Type != "fanout" {
			errorIssue("exchange_type_unsupported", "exchange "+exchange.Name+" uses unsupported type "+exchange.Type)
			continue
		}
		if len(exchange.Arguments) > 0 || len(binding.Arguments) > 0 {
			errorIssue("routing_arguments_unsupported", "exchange or binding arguments require manual migration")
			continue
		}
		key := exchange.Name + "\x00" + exchange.Type
		mapped := grouped[key]
		if mapped == nil {
			mapped = &topology.Binding{Exchange: exchange.Name, Type: exchange.Type}
			grouped[key] = mapped
		}
		if exchange.Type != "fanout" {
			if binding.RoutingKey == "" {
				errorIssue("empty_routing_key_unsupported", "empty direct/topic routing keys cannot be represented")
				continue
			}
			mapped.Keys = append(mapped.Keys, binding.RoutingKey)
		}
	}
	for _, binding := range grouped {
		queue.Spec.Bindings = append(queue.Spec.Bindings, *binding)
	}
	if len(queue.Spec.Bindings) == 0 {
		queue.Spec.Subjects = []string{"rjs.migrated." + source.Name}
		warn("native_ingress_fallback", "Queue has no safely mapped exchange binding; publishers must use the generated native ingress subject")
	}
	applyQueueArguments(queue, source.Arguments, exchanges, allBindings, &issues)
	queue.Default()
	if err := queue.Validate(); err != nil {
		errorIssue("queue_validation_failed", err.Error())
	}
	for _, issue := range issues {
		if issue.Severity == "error" {
			return nil, issues
		}
	}
	return queue, issues
}

func applyQueueArguments(queue *topology.Queue, arguments map[string]any, exchanges map[string]RabbitExchange, allBindings map[string][]RabbitBinding, issues *[]Issue) {
	resource := "queue:" + queue.Metadata.Name
	known := map[string]bool{"x-message-ttl": true, "x-max-length": true, "x-max-length-bytes": true, "x-dead-letter-exchange": true, "x-dead-letter-routing-key": true, "x-queue-type": true}
	if value, ok := integer(arguments["x-message-ttl"]); ok && value <= math.MaxInt64/int64(time.Millisecond) {
		queue.Spec.Retention.MaxAge = topology.Duration(time.Duration(value) * time.Millisecond)
	} else if arguments["x-message-ttl"] != nil {
		*issues = append(*issues, Issue{"error", resource, "invalid_message_ttl", "x-message-ttl must be a non-negative integer"})
	}
	if value, ok := integer(arguments["x-max-length"]); ok {
		queue.Spec.Retention.MaxMessages = value
	} else if arguments["x-max-length"] != nil {
		*issues = append(*issues, Issue{"error", resource, "invalid_max_length", "x-max-length must be a non-negative integer"})
	}
	if value, ok := integer(arguments["x-max-length-bytes"]); ok {
		queue.Spec.Retention.MaxBytes = topology.ByteSize(value)
	} else if arguments["x-max-length-bytes"] != nil {
		*issues = append(*issues, Issue{"error", resource, "invalid_max_length_bytes", "x-max-length-bytes must be a non-negative integer"})
	}
	queueType, queueTypeIsString := arguments["x-queue-type"].(string)
	if arguments["x-queue-type"] != nil && !queueTypeIsString {
		*issues = append(*issues, Issue{"error", resource, "queue_type_unsupported", "x-queue-type must be a string"})
	} else if queueType == "stream" {
		*issues = append(*issues, Issue{"error", resource, "rabbit_stream_unsupported", "RabbitMQ stream queues require a separate migration path"})
	} else if queueType != "" && queueType != "classic" && queueType != "quorum" {
		*issues = append(*issues, Issue{"error", resource, "queue_type_unsupported", "unknown x-queue-type " + queueType})
	}
	if dlx, ok := arguments["x-dead-letter-exchange"].(string); ok {
		targets := deadLetterTargets(dlx, stringValue(arguments["x-dead-letter-routing-key"]), exchanges, allBindings)
		if len(targets) == 1 && targets[0] != queue.Metadata.Name {
			queue.Spec.DeadLetter = &topology.DeadLetterPolicy{Queue: targets[0]}
		} else {
			*issues = append(*issues, Issue{"error", resource, "dead_letter_ambiguous", "dead-letter exchange must resolve to exactly one different Queue"})
		}
	} else if arguments["x-dead-letter-exchange"] != nil {
		*issues = append(*issues, Issue{"error", resource, "dead_letter_invalid", "x-dead-letter-exchange must be a string"})
	} else if arguments["x-dead-letter-routing-key"] != nil {
		*issues = append(*issues, Issue{"error", resource, "dead_letter_invalid", "x-dead-letter-routing-key requires x-dead-letter-exchange"})
	}
	for key := range arguments {
		if !known[key] {
			*issues = append(*issues, Issue{"error", resource, "queue_argument_unsupported", "unsupported RabbitMQ queue argument " + key})
		}
	}
}

func deadLetterTargets(exchangeName, routingKey string, exchanges map[string]RabbitExchange, bindingsByQueue map[string][]RabbitBinding) []string {
	exchange, exists := exchanges[exchangeName]
	if !exists || (exchange.Type != "direct" && exchange.Type != "fanout") {
		return nil
	}
	if exchange.Type == "direct" && routingKey == "" {
		return nil
	}
	var targets []string
	for queue, bindings := range bindingsByQueue {
		for _, binding := range bindings {
			if binding.Source == exchangeName && (exchange.Type == "fanout" || binding.RoutingKey == routingKey) {
				targets = append(targets, queue)
				break
			}
		}
	}
	sort.Strings(targets)
	return targets
}

func integer(value any) (int64, bool) {
	if value == nil {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Int64()
	return parsed, err == nil && parsed >= 0
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (r *Result) issue(severity, resource, code, message string) {
	r.Report.Issues = append(r.Report.Issues, Issue{Severity: severity, Resource: resource, Code: code, Message: message})
}

func (r *Result) pruneInvalidDependencies() {
	for {
		available := make(map[string]bool, len(r.Queues))
		for _, queue := range r.Queues {
			available[queue.Metadata.Name] = true
		}
		filtered := make([]topology.Queue, 0, len(r.Queues))
		changed := false
		for _, queue := range r.Queues {
			if queue.Spec.DeadLetter != nil && !available[queue.Spec.DeadLetter.Queue] {
				r.issue("error", "queue:"+queue.Metadata.Name, "dead_letter_target_incompatible", "dead-letter target Queue was not converted")
				changed = true
				continue
			}
			filtered = append(filtered, queue)
		}
		r.Queues = filtered
		if !changed {
			break
		}
	}
	byName := make(map[string]topology.Queue, len(r.Queues))
	for _, queue := range r.Queues {
		byName[queue.Metadata.Name] = queue
	}
	cyclic := make(map[string]bool)
	for name := range byName {
		path, positions := []string{}, map[string]int{}
		for current := name; current != ""; {
			if position, exists := positions[current]; exists {
				for _, member := range path[position:] {
					cyclic[member] = true
				}
				break
			}
			positions[current] = len(path)
			path = append(path, current)
			queue, exists := byName[current]
			if !exists || queue.Spec.DeadLetter == nil {
				break
			}
			current = queue.Spec.DeadLetter.Queue
		}
	}
	if len(cyclic) == 0 {
		return
	}
	filtered := make([]topology.Queue, 0, len(r.Queues)-len(cyclic))
	for _, queue := range r.Queues {
		if cyclic[queue.Metadata.Name] {
			r.issue("error", "queue:"+queue.Metadata.Name, "dead_letter_cycle", "dead-letter Queue dependencies contain a cycle")
			continue
		}
		filtered = append(filtered, queue)
	}
	r.Queues = filtered
	// Queues outside a cycle may depend on a removed cycle member.
	r.pruneInvalidDependencies()
}
