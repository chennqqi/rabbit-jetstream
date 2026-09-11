package jetstream

import (
	"context"
	"fmt"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

// DeclarationReview is separate from observed-resource reconciliation. Neither
// an empty declaration diff nor a successful conversion authorizes a write.
type DeclarationReview struct {
	Status   string          `json:"status"`
	Reason   string          `json:"reason,omitempty"`
	Document *topology.Queue `json:"document,omitempty"`
	Diff     *topology.Diff  `json:"diff,omitempty"`
}

func (c *Client) previewDeclaration(ctx context.Context, plan topology.Plan, precondition ApplyPrecondition) (*DeclarationReview, error) {
	target, err := topology.QueueDocument(plan)
	if err != nil {
		return &DeclarationReview{Status: "unavailable", Reason: "target_unrepresentable"}, nil
	}
	review := &DeclarationReview{Status: "create", Document: target}
	if precondition.CreateOnly {
		return review, nil
	}
	declaration, err := c.Declaration(ctx, plan.Queue)
	if err != nil {
		return nil, err
	}
	if precondition.ExpectedRevision == nil || declaration.KVRevision != *precondition.ExpectedRevision {
		return nil, fmt.Errorf("%w: Queue %s revision changed during declaration review", ErrConflict, plan.Queue)
	}
	if declaration.APIVersion != topology.DeclarationAPIVersion || declaration.Queue != plan.Queue || declaration.Plan.Queue != plan.Queue || declaration.Revision != declaration.Plan.Revision {
		review.Status, review.Reason = "unavailable", "base_identity_mismatch"
		return review, nil
	}
	base, err := topology.QueueDocument(declaration.Plan)
	if err != nil {
		review.Status, review.Reason = "unavailable", "base_unrepresentable"
		return review, nil
	}
	diff := topology.Compare(*base, *target)
	review.Status, review.Diff = "available", &diff
	return review, nil
}
