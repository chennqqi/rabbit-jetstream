package jetstream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

// DeletePreview is advisory evidence, not a reservation or an empty/unused CAS.
type DeletePreview struct {
	Queue         string    `json:"queue"`
	Stream        string    `json:"stream"`
	BaseRevision  string    `json:"base_revision"`
	ObservedAt    time.Time `json:"observed_at"`
	StreamPresent bool      `json:"stream_present"`
	Ownership     string    `json:"ownership"`
	Messages      *uint64   `json:"messages,omitempty"`
	Consumers     *int      `json:"consumers,omitempty"`
	RequiresForce bool      `json:"requires_force"`
	Blocked       bool      `json:"blocked"`
	Reason        string    `json:"reason,omitempty"`
}

func (c *Client) PreviewDelete(ctx context.Context, name string, precondition ApplyPrecondition) (*DeletePreview, error) {
	if !topology.ValidQueueName(name) || precondition.CreateOnly || precondition.ExpectedRevision == nil {
		return nil, fmt.Errorf("%w: deletion preview requires Queue identity and original declaration revision", ErrConflict)
	}
	declaration, err := c.Declaration(ctx, name)
	if err != nil {
		return nil, err
	}
	if declaration.APIVersion != topology.DeclarationAPIVersion || declaration.Revision != declaration.Plan.Revision || declaration.KVRevision != *precondition.ExpectedRevision || declaration.Queue != name || declaration.Plan.Queue != name || declaration.Plan.Stream.Name != topology.StreamName(name) {
		return nil, fmt.Errorf("%w: declaration identity or revision changed", ErrConflict)
	}
	result := &DeletePreview{Queue: name, Stream: topology.StreamName(name), BaseRevision: fmt.Sprintf("\"%d\"", *precondition.ExpectedRevision), Ownership: "unobserved"}
	stream, err := c.Stream(ctx, result.Stream)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err == nil {
		if stream == nil || stream.Name != result.Stream || stream.Consumers < 0 {
			return nil, fmt.Errorf("invalid Stream deletion observation")
		}
		result.StreamPresent = true
		result.Messages, result.Consumers = &stream.Messages, &stream.Consumers
		result.Ownership = queueOwnership(stream.Metadata, name)
		result.RequiresForce = stream.Messages > 0
		if result.Ownership != "matching" {
			result.Blocked, result.Reason = true, "Stream is not owned by this Queue"
		} else if result.RequiresForce {
			result.Blocked, result.Reason = true, "Stream contains messages; force is required"
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.checkApplyPrecondition(ctx, name, precondition); err != nil {
		return nil, err
	}
	result.ObservedAt = time.Now().UTC()
	return result, nil
}
