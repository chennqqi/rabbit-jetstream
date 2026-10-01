package jetstream

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

const QueueConsumerReadLimit = 2000

type QueueConsumer struct {
	Stream    string                 `json:"stream"`
	Name      string                 `json:"name"`
	Expected  *topology.ConsumerPlan `json:"expected"`
	Observed  *Consumer              `json:"observed"`
	Status    string                 `json:"status"`
	Ownership string                 `json:"ownership"`
}

type QueueConsumerCollection struct {
	Queue               string          `json:"queue"`
	Stream              string          `json:"stream"`
	DeclarationRevision string          `json:"declaration_revision"`
	StreamStatus        string          `json:"stream_status"`
	StreamOwnership     string          `json:"stream_ownership"`
	Items               []QueueConsumer `json:"items"`
}

// QueueConsumers only reads the declared Stream. The result is not an atomic
// broker snapshot; revision checking prevents mixing different declarations.
func (c *Client) QueueConsumers(ctx context.Context, name string) (*QueueConsumerCollection, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	declaration, err := c.Declaration(ctx, name)
	if err != nil {
		return nil, err
	}
	plan := declaration.Plan
	if declaration.Queue != name || plan.Queue != name || plan.Stream.Name == "" || len(plan.PriorityConsumers) > topology.MaximumPriority {
		return nil, fmt.Errorf("invalid declaration identity for Queue %s", name)
	}
	result := &QueueConsumerCollection{Queue: name, Stream: plan.Stream.Name, DeclarationRevision: fmt.Sprintf("\"%d\"", declaration.KVRevision), StreamStatus: "missing", StreamOwnership: "unknown", Items: []QueueConsumer{}}
	indices := map[string]int{}
	for _, expected := range append([]topology.ConsumerPlan{plan.Consumer}, plan.PriorityConsumers...) {
		if expected.Stream != plan.Stream.Name || expected.Name == "" {
			return nil, fmt.Errorf("invalid Consumer identity in Queue %s declaration", name)
		}
		if _, exists := indices[expected.Name]; exists {
			return nil, fmt.Errorf("duplicate Consumer in Queue %s declaration", name)
		}
		indices[expected.Name] = len(result.Items)
		result.Items = append(result.Items, QueueConsumer{Stream: expected.Stream, Name: expected.Name, Expected: &expected, Status: "missing", Ownership: "unknown"})
	}
	stream, err := c.js.Stream(ctx, plan.Stream.Name)
	if err != nil && !errors.Is(err, jsapi.ErrStreamNotFound) {
		return nil, fmt.Errorf("read Queue stream: %w", err)
	}
	if err == nil {
		result.StreamStatus = "present"
		result.StreamOwnership = queueOwnership(stream.CachedInfo().Config.Metadata, name)
		lister := stream.ListConsumers(ctx)
		count := 0
		seen := map[string]bool{}
		for info := range lister.Info() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			count++
			if count > QueueConsumerReadLimit {
				return nil, fmt.Errorf("Queue Consumer read limit %d exceeded", QueueConsumerReadLimit)
			}
			if info.Stream != plan.Stream.Name || info.Name == "" || seen[info.Name] {
				return nil, errors.New("inconsistent Queue Consumer enumeration; refresh required")
			}
			seen[info.Name] = true
			observed := consumerFromInfo(info)
			index, expected := indices[observed.Name]
			if !expected {
				index = len(result.Items)
				result.Items = append(result.Items, QueueConsumer{Stream: observed.Stream, Name: observed.Name})
			}
			result.Items[index].Observed = &observed
			result.Items[index].Status = "present"
			result.Items[index].Ownership = queueOwnership(observed.Metadata, name)
		}
		if err := lister.Err(); err != nil {
			return nil, fmt.Errorf("list Queue Consumers: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	current, err := c.Declaration(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("%w: Queue declaration disappeared", ErrConflict)
	}
	if err != nil {
		return nil, err
	}
	if current.KVRevision != declaration.KVRevision {
		return nil, fmt.Errorf("%w: Queue declaration changed during read", ErrConflict)
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Name < result.Items[j].Name })
	return result, nil
}

// Ownership reports metadata evidence, not authorization or reconciliation health.
func queueOwnership(metadata map[string]string, queue string) string {
	owner := metadata["rabbit-jetstream.io/queue"]
	if owner == "" {
		return "unmarked"
	}
	if owner == queue {
		return "matching"
	}
	return "different"
}
