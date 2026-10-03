package topology

import (
	"fmt"
	"strings"
)

// RoutingProbe specifies either a literal NATS subject or an exchange target.
// It never requests publication or resolves bindings across other Queues.
type RoutingProbe struct {
	Subject    string `json:"subject,omitempty"`
	Exchange   string `json:"exchange,omitempty"`
	Type       string `json:"type,omitempty"`
	RoutingKey string `json:"routingKey,omitempty"`
}

type RoutingProbeBinding struct {
	RoutingPlan
	MatchedSubjects []string `json:"matchedSubjects"`
}

// RoutingProbeResult describes a generated declaration, not live delivery.
// Bindings and Stream matches are deliberately separate: priority Streams do
// not capture the ordinary exchange/ingress subjects.
type RoutingProbeResult struct {
	Queue                 string                `json:"queue"`
	Revision              string                `json:"revision"`
	Stream                string                `json:"stream"`
	Subject               string                `json:"subject"`
	StreamSubjects        []string              `json:"streamSubjects"`
	MatchedStreamSubjects []string              `json:"matchedStreamSubjects"`
	Bindings              []RoutingProbeBinding `json:"bindings"`
}

// ProbeRouting validates and translates a Queue using the production planner.
// It performs no I/O and does not modify the caller's declaration. A successful
// result with no matches is valid; malformed or unsupported inputs are errors.
func ProbeRouting(queue Queue, probe RoutingProbe) (RoutingProbeResult, error) {
	// BuildPlan defaults/sorts slice contents, even though Queue is passed by
	// value. Copy precisely those slices before invoking it.
	plan, err := BuildPlan(planningCopy(queue))
	if err != nil {
		return RoutingProbeResult{}, fmt.Errorf("routing declaration: %w", err)
	}
	subject := probe.Subject
	if subject != "" {
		if probe.Exchange != "" || probe.Type != "" || probe.RoutingKey != "" {
			return RoutingProbeResult{}, fmt.Errorf("subject and exchange target are mutually exclusive")
		}
		if err := validateSubject(subject); err != nil {
			return RoutingProbeResult{}, fmt.Errorf("probe subject: %w", err)
		}
		if strings.ContainsAny(subject, "*>") {
			return RoutingProbeResult{}, fmt.Errorf("probe subject must be literal")
		}
	} else {
		if plan.MaxPriority != nil {
			return RoutingProbeResult{}, fmt.Errorf("priority Queue requires a literal priority subject; exchange-to-priority resolution is not supported")
		}
		subject, err = QueuePublishSubject(plan.Queue, probe.Exchange, probe.Type, probe.RoutingKey)
		if err != nil {
			return RoutingProbeResult{}, fmt.Errorf("probe exchange target: %w", err)
		}
	}
	result := RoutingProbeResult{
		Queue: plan.Queue, Revision: plan.Revision, Stream: plan.Stream.Name, Subject: subject,
		StreamSubjects: plan.Stream.Subjects, MatchedStreamSubjects: matchedRoutingSubjects(plan.Stream.Subjects, subject),
		Bindings: make([]RoutingProbeBinding, 0, len(plan.Routing)),
	}
	for _, route := range plan.Routing {
		result.Bindings = append(result.Bindings, RoutingProbeBinding{
			RoutingPlan: route, MatchedSubjects: matchedRoutingSubjects(route.Subjects, subject),
		})
	}
	return result, nil
}

func matchedRoutingSubjects(patterns []string, subject string) []string {
	matched := make([]string, 0)
	tokens := strings.Split(subject, ".")
	for _, pattern := range patterns {
		if routingSubjectMatches(strings.Split(pattern, "."), tokens) {
			matched = append(matched, pattern)
		}
	}
	return matched
}

// Inputs are already validated. NATS '>' consumes at least one token, unlike
// RabbitMQ '#'; the planner expands a supported trailing '#' into two subjects.
func routingSubjectMatches(pattern, subject []string) bool {
	for index, token := range pattern {
		if index >= len(subject) {
			return false
		}
		if token == ">" {
			return true
		}
		if token != "*" && token != subject[index] {
			return false
		}
	}
	return len(pattern) == len(subject)
}
