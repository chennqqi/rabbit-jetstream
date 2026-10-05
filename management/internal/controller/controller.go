package controller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

type Backend interface {
	AcquireControllerLease(context.Context, string, time.Duration) (bool, error)
	ReleaseControllerLease(context.Context, string) error
	ListDeclarations(context.Context) ([]topology.Declaration, error)
	ApplyDeclaration(context.Context, topology.Declaration) (topology.ReconcileResult, error)
	ProcessDeadLetters(context.Context, []topology.Declaration, int) (topology.DeadLetterProcessResult, error)
}

type Status struct {
	InstanceID   string                                    `json:"instanceId"`
	Enabled      bool                                      `json:"enabled"`
	Leader       bool                                      `json:"leader"`
	LastRun      time.Time                                 `json:"lastRun,omitempty"`
	LastSuccess  time.Time                                 `json:"lastSuccess,omitempty"`
	Declarations int                                       `json:"declarations"`
	Reconciled   int                                       `json:"reconciled"`
	Blocked      int                                       `json:"blocked"`
	DLQProcessed int                                       `json:"dlqProcessed"`
	DLQMoved     int                                       `json:"dlqMoved"`
	DLQFailed    int                                       `json:"dlqFailed"`
	DLQIgnored   int                                       `json:"dlqIgnored"`
	DLQByQueue   map[string]topology.DeadLetterQueueCounts `json:"dlqByQueue,omitempty"`
	LastError    string                                    `json:"lastError,omitempty"`
}

type Controller struct {
	backend            Backend
	logger             *slog.Logger
	instanceID         string
	interval, leaseTTL time.Duration
	enabled            bool
	mu                 sync.RWMutex
	status             Status
}

func New(backend Backend, logger *slog.Logger, instanceID string, enabled bool, interval, leaseTTL time.Duration) *Controller {
	return &Controller{backend: backend, logger: logger, instanceID: instanceID, enabled: enabled, interval: interval, leaseTTL: leaseTTL, status: Status{InstanceID: instanceID, Enabled: enabled}}
}

func (c *Controller) Run(ctx context.Context) {
	if !c.enabled {
		return
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := c.backend.ReleaseControllerLease(releaseCtx, c.instanceID); err != nil {
			c.logger.Warn("controller lease release failed", "error", err)
		}
	}()
	c.runOnce(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *Controller) Status() Status { c.mu.RLock(); defer c.mu.RUnlock(); return c.status }

func (c *Controller) runOnce(parent context.Context) {
	now := time.Now().UTC()
	leaseCtx, cancel := context.WithTimeout(parent, c.interval)
	leader, err := c.backend.AcquireControllerLease(leaseCtx, c.instanceID, c.leaseTTL)
	cancel()
	if err != nil {
		c.update(now, false, 0, 0, 0, err.Error())
		c.logger.Warn("controller lease failed", "error", err)
		return
	}
	if !leader {
		c.update(now, false, 0, 0, 0, "")
		return
	}
	listCtx, cancel := context.WithTimeout(parent, c.interval)
	declarations, err := c.backend.ListDeclarations(listCtx)
	cancel()
	if err != nil {
		c.update(now, true, 0, 0, 0, err.Error())
		c.logger.Warn("controller declarations failed", "error", err)
		return
	}
	reconciled, blocked := 0, 0
	for _, declaration := range declarations {
		renewCtx, renewCancel := context.WithTimeout(parent, c.interval)
		stillLeader, renewErr := c.backend.AcquireControllerLease(renewCtx, c.instanceID, c.leaseTTL)
		renewCancel()
		if renewErr != nil || !stillLeader {
			message := "controller lease lost"
			if renewErr != nil {
				message = renewErr.Error()
			}
			c.update(now, false, len(declarations), reconciled, blocked, message)
			return
		}
		applyCtx, applyCancel := context.WithTimeout(parent, c.interval)
		result, applyErr := c.backend.ApplyDeclaration(applyCtx, declaration)
		applyCancel()
		if applyErr != nil {
			c.update(now, true, len(declarations), reconciled, blocked, applyErr.Error())
			c.logger.Warn("controller reconcile failed", "queue", declaration.Queue, "error", applyErr)
			return
		}
		if result.Blocked {
			blocked++
		} else {
			reconciled++
		}
	}
	dlqCtx, dlqCancel := context.WithTimeout(parent, c.interval)
	dlqResult, dlqErr := c.backend.ProcessDeadLetters(dlqCtx, declarations, 100)
	dlqCancel()
	if dlqErr != nil {
		c.updateWithDeadLetters(now, true, len(declarations), reconciled, blocked, dlqErr.Error(), dlqResult)
		c.logger.Warn("dead-letter processing failed", "error", dlqErr)
		return
	}
	c.updateWithDeadLetters(now, true, len(declarations), reconciled, blocked, "", dlqResult)
}

func (c *Controller) update(now time.Time, leader bool, declarations, reconciled, blocked int, lastError string) {
	c.updateWithDeadLetters(now, leader, declarations, reconciled, blocked, lastError, topology.DeadLetterProcessResult{})
}

func (c *Controller) updateWithDeadLetters(now time.Time, leader bool, declarations, reconciled, blocked int, lastError string, result topology.DeadLetterProcessResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A batch may report completed work together with a later read/ack error.
	// Publish its counters and run status atomically, even on partial failure.
	c.status.DLQProcessed += result.Processed
	c.status.DLQMoved += result.Moved
	c.status.DLQFailed += result.Failed
	c.status.DLQIgnored += result.Ignored
	for queue, counts := range result.PerQueue {
		if c.status.DLQByQueue == nil {
			c.status.DLQByQueue = make(map[string]topology.DeadLetterQueueCounts)
		}
		current := c.status.DLQByQueue[queue]
		current.Moved += counts.Moved
		current.Failed += counts.Failed
		c.status.DLQByQueue[queue] = current
	}
	c.status.Leader, c.status.LastRun = leader, now
	c.status.Declarations, c.status.Reconciled, c.status.Blocked, c.status.LastError = declarations, reconciled, blocked, lastError
	if lastError == "" && leader {
		c.status.LastSuccess = now
	}
}
