package jetstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

const maximumDeadLetterDependencies = 100

// ErrDeadLetterCycle means the observed declaration chain contains a cycle,
// not that the broker is unavailable or the source revision is stale.
var ErrDeadLetterCycle = errors.New("DLQ dependency cycle")

// Follow saved declarations, not broker data. Reaching the proposed source or
// any already visited target is unsafe. The caller rechecks all KV revisions
// after its direct-target Stream observation; this is not a multi-key lock.
func (c *Client) deadLetterDependencyChain(ctx context.Context, source topology.Plan) ([]*topology.Declaration, error) {
	seen := map[string]bool{source.Queue: true}
	chain := make([]*topology.Declaration, 0)
	name := source.DeadLetter.Queue
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[name] {
			return nil, fmt.Errorf("%w at %s", ErrDeadLetterCycle, name)
		}
		if len(chain) >= maximumDeadLetterDependencies {
			return nil, fmt.Errorf("DLQ dependency chain exceeds %d declarations", maximumDeadLetterDependencies)
		}
		seen[name] = true
		target, err := c.Declaration(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("DLQ dependency %s is not applied: %w", name, err)
		}
		if target == nil || target.Queue != name || target.Plan.Queue != name || target.Revision != target.Plan.Revision || target.Plan.Stream.Name != topology.StreamName(name) {
			return nil, fmt.Errorf("DLQ dependency %s has inconsistent declaration identity", name)
		}
		if _, err := topology.QueueDocument(target.Plan); err != nil {
			return nil, fmt.Errorf("DLQ dependency %s cannot reconstruct its declaration: %w", name, err)
		}
		chain = append(chain, target)
		if target.Plan.DeadLetter == nil {
			return chain, nil
		}
		name = target.Plan.DeadLetter.Queue
	}
}
