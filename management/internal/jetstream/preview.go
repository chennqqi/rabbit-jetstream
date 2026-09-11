package jetstream

import (
	"context"
	"fmt"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

type PlanPreview struct {
	Plan              topology.Plan            `json:"plan"`
	Result            topology.ReconcileResult `json:"result"`
	ObservedAt        time.Time                `json:"observed_at"`
	BaseRevision      string                   `json:"base_revision"`
	CreateOnly        bool                     `json:"create_only"`
	DeclarationReview *DeclarationReview       `json:"declaration_review,omitempty"`
}

// Preview is advisory and strictly read-only. No lock, metadata, audit intent,
// broker resource or DLQ infrastructure is created. Apply must recheck safety.
func (c *Client) Preview(ctx context.Context, plan topology.Plan, precondition ApplyPrecondition) (*PlanPreview, error) {
	if err := c.checkApplyPrecondition(ctx, plan.Queue, precondition); err != nil {
		return nil, err
	}
	declarationReview, err := c.previewDeclaration(ctx, plan, precondition)
	if err != nil {
		return nil, err
	}
	observed, err := c.observedTopology(ctx, plan)
	if err != nil {
		return nil, err
	}
	result := topology.Reconcile(plan, observed)
	// Match apply's ordering: unsafe transitions are reported before DLQ checks.
	if !result.Blocked && plan.DeadLetter != nil {
		if err := c.checkDeadLetterDependency(ctx, plan); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.checkApplyPrecondition(ctx, plan.Queue, precondition); err != nil {
		return nil, err
	}
	preview := &PlanPreview{Plan: plan, Result: result, ObservedAt: time.Now().UTC(), CreateOnly: precondition.CreateOnly, DeclarationReview: declarationReview}
	if !precondition.CreateOnly && precondition.ExpectedRevision != nil {
		preview.BaseRevision = fmt.Sprintf("\"%d\"", *precondition.ExpectedRevision)
	}
	return preview, nil
}

func (c *Client) checkDeadLetterDependency(ctx context.Context, plan topology.Plan) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	chain, err := c.deadLetterDependencyChain(ctx, plan)
	if err != nil {
		return err
	}
	target := chain[0]
	if err := validateDeadLetterPriority(plan, target.Plan); err != nil {
		return err
	}
	stream, err := c.Stream(ctx, target.Plan.Stream.Name)
	if err != nil {
		return fmt.Errorf("DLQ dependency %s Stream unavailable: %w", target.Queue, err)
	}
	if stream.Name != target.Plan.Stream.Name {
		return fmt.Errorf("DLQ dependency %s Stream identity mismatch", target.Queue)
	}
	// Compare ownership/configuration without repairing the target. This does
	// not prove Consumer health, delivery or atomicity against external writers.
	result := topology.Reconcile(target.Plan, topology.ObservedTopology{Stream: observedStream(stream)})
	if len(result.Operations) == 0 || result.Operations[0].Resource != "stream" || result.Operations[0].Action != "noop" {
		return fmt.Errorf("DLQ dependency %s Stream differs from its saved declaration", target.Queue)
	}
	for _, dependency := range chain {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := c.Declaration(ctx, dependency.Queue)
		if err != nil {
			return fmt.Errorf("recheck DLQ dependency %s: %w", dependency.Queue, err)
		}
		if current.KVRevision != dependency.KVRevision {
			return fmt.Errorf("DLQ dependency %s declaration changed during observation", dependency.Queue)
		}
	}
	return ctx.Err()
}
