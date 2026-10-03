package topology

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// QueueDocument reconstructs a normalized editable document only when its
// complete generated plan is identical. Legacy/unrepresentable plans remain
// inspectable but must not be silently converted into a lossy editor draft.
func QueueDocument(plan Plan) (*Queue, error) {
	if plan.APIVersion != PlanAPIVersion {
		return nil, fmt.Errorf("unsupported plan version %q", plan.APIVersion)
	}
	ackWait := Duration(plan.Consumer.AckWaitNanos)
	maxDeliver := plan.Consumer.MaxDeliver
	queue := Queue{
		APIVersion: QueueAPIVersion, Kind: QueueKind,
		Metadata: Metadata{Name: plan.Queue},
		Spec: QueueSpec{
			Storage: plan.Stream.Storage, Replicas: plan.Stream.Replicas,
			Retention:   RetentionPolicy{MaxAge: Duration(plan.Stream.MaxAgeNanos), MaxBytes: ByteSize(plan.Stream.MaxBytes), MaxMessages: plan.Stream.MaxMessages},
			Delivery:    DeliveryPolicy{AckWait: &ackWait, MaxDeliver: &maxDeliver},
			MaxPriority: cloneInt(plan.MaxPriority),
		},
	}
	const labelPrefix = "rabbit-jetstream.io/label."
	for key, value := range plan.Stream.Metadata {
		if strings.HasPrefix(key, labelPrefix) {
			if queue.Metadata.Labels == nil {
				queue.Metadata.Labels = map[string]string{}
			}
			queue.Metadata.Labels[strings.TrimPrefix(key, labelPrefix)] = value
		}
	}
	if len(plan.Routing) > 0 {
		for _, route := range plan.Routing {
			queue.Spec.Bindings = append(queue.Spec.Bindings, Binding{Exchange: route.Exchange, Type: route.Type, Keys: append([]string(nil), route.Keys...)})
		}
	} else {
		queue.Spec.Subjects = append([]string(nil), plan.DeclarationSubjects...)
	}
	if plan.DeadLetter != nil {
		queue.Spec.DeadLetter = &DeadLetterPolicy{Queue: plan.DeadLetter.Queue}
	}
	queue.Default()
	rebuilt, err := BuildPlan(queue)
	if err != nil {
		return nil, fmt.Errorf("reconstruct Queue document: %w", err)
	}
	originalJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("encode original plan: %w", err)
	}
	rebuiltJSON, err := json.Marshal(rebuilt)
	if err != nil {
		return nil, fmt.Errorf("encode reconstructed plan: %w", err)
	}
	if !bytes.Equal(originalJSON, rebuiltJSON) {
		return nil, fmt.Errorf("plan cannot be converted to a Queue document without changing generated configuration")
	}
	return &queue, nil
}
