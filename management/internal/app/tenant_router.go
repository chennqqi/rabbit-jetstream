package app

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

var errTenantBackendMissing = errors.New("tenant backend is unavailable")

// tenantRouter is the isolation boundary: every contextual operation is sent
// only to the selected tenant's NATS account.
type tenantRouter struct {
	clients     map[string]*jetstream.Client
	monitors    map[string]*monitoring.Client
	controllers map[string]*controller.Controller
	defaultID   string
	closeOnce   sync.Once
}

func (r *tenantRouter) selected(ctx context.Context) (*jetstream.Client, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		id = r.defaultID
	}
	client, ok := r.clients[id]
	if !ok {
		return nil, errTenantBackendMissing
	}
	return client, nil
}

func (r *tenantRouter) ServerURL() string {
	if client := r.clients[r.defaultID]; client != nil {
		return client.ServerURL()
	}
	return ""
}
func (r *tenantRouter) ServerURLFor(ctx context.Context) string {
	client, err := r.selected(ctx)
	if err != nil {
		return ""
	}
	return client.ServerURL()
}
func (r *tenantRouter) Ready(ctx context.Context) error {
	if _, ok := tenant.FromContext(ctx); !ok {
		ids := make([]string, 0, len(r.clients))
		for id := range r.clients {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if err := r.clients[id].Ready(ctx); err != nil {
				return errors.New("one or more tenant backends are unavailable")
			}
		}
		return nil
	}
	c, err := r.selected(ctx)
	if err != nil {
		return err
	}
	return c.Ready(ctx)
}
func (r *tenantRouter) AccountInfo(ctx context.Context) (*jetstream.Account, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.AccountInfo(ctx)
}
func (r *tenantRouter) ListStreams(ctx context.Context) ([]jetstream.Stream, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.ListStreams(ctx)
}
func (r *tenantRouter) Stream(ctx context.Context, name string) (*jetstream.Stream, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.Stream(ctx, name)
}
func (r *tenantRouter) ListConsumers(ctx context.Context, stream string) ([]jetstream.Consumer, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.ListConsumers(ctx, stream)
}
func (r *tenantRouter) Consumer(ctx context.Context, stream, consumer string) (*jetstream.Consumer, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.Consumer(ctx, stream, consumer)
}
func (r *tenantRouter) QueueConsumers(ctx context.Context, queue string) (*jetstream.QueueConsumerCollection, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.QueueConsumers(ctx, queue)
}
func (r *tenantRouter) Preview(ctx context.Context, plan topology.Plan, p jetstream.ApplyPrecondition) (*jetstream.PlanPreview, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.Preview(ctx, plan, p)
}
func (r *tenantRouter) PreviewDelete(ctx context.Context, name string, p jetstream.ApplyPrecondition) (*jetstream.DeletePreview, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.PreviewDelete(ctx, name, p)
}
func (r *tenantRouter) Apply(ctx context.Context, plan topology.Plan) (topology.ReconcileResult, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return topology.ReconcileResult{}, err
	}
	return c.Apply(ctx, plan)
}
func (r *tenantRouter) ApplyConditional(ctx context.Context, plan topology.Plan, p jetstream.ApplyPrecondition) (topology.ReconcileResult, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return topology.ReconcileResult{}, err
	}
	return c.ApplyConditional(ctx, plan, p)
}
func (r *tenantRouter) DeleteQueue(ctx context.Context, name string, force bool) (topology.DeleteResult, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return topology.DeleteResult{}, err
	}
	return c.DeleteQueue(ctx, name, force)
}
func (r *tenantRouter) DeleteQueueConditional(ctx context.Context, name string, force bool, p jetstream.ApplyPrecondition) (topology.DeleteResult, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return topology.DeleteResult{}, err
	}
	return c.DeleteQueueConditional(ctx, name, force, p)
}
func (r *tenantRouter) ListDeclarations(ctx context.Context) ([]topology.Declaration, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.ListDeclarations(ctx)
}
func (r *tenantRouter) Declaration(ctx context.Context, name string) (*topology.Declaration, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return nil, err
	}
	return c.Declaration(ctx, name)
}
func (r *tenantRouter) RecordAudit(ctx context.Context, event jetstream.AuditEvent) (uint64, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return 0, err
	}
	return c.RecordAudit(ctx, event)
}
func (r *tenantRouter) ListAudit(ctx context.Context, offset, limit int) (jetstream.AuditPage, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return jetstream.AuditPage{}, err
	}
	return c.ListAudit(ctx, offset, limit)
}
func (r *tenantRouter) AuditRequest(ctx context.Context, id string, before *uint64) (jetstream.AuditRequestPage, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return jetstream.AuditRequestPage{}, err
	}
	return c.AuditRequest(ctx, id, before)
}
func (r *tenantRouter) AuditWindow(ctx context.Context, filter jetstream.AuditFilter, before *uint64) (jetstream.AuditWindowPage, error) {
	c, err := r.selected(ctx)
	if err != nil {
		return jetstream.AuditWindowPage{}, err
	}
	return c.AuditWindow(ctx, filter, before)
}
func (r *tenantRouter) Nodes(ctx context.Context) monitoring.Snapshot {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		id = r.defaultID
	}
	if m := r.monitors[id]; m != nil {
		return m.Nodes(ctx)
	}
	return monitoring.Snapshot{}
}
func (r *tenantRouter) selectedMonitor(ctx context.Context) (*monitoring.Client, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		id = r.defaultID
	}
	value := r.monitors[id]
	if value == nil {
		return nil, errTenantBackendMissing
	}
	return value, nil
}
func (r *tenantRouter) NodeConnections(ctx context.Context, node string, offset, limit int) (*monitoring.ConnectionPage, error) {
	m, err := r.selectedMonitor(ctx)
	if err != nil {
		return nil, err
	}
	return m.NodeConnections(ctx, node, offset, limit)
}
func (r *tenantRouter) NodeConnectionCIDPage(ctx context.Context, node string, cid uint64, limit int) (*monitoring.ConnectionPage, error) {
	m, err := r.selectedMonitor(ctx)
	if err != nil {
		return nil, err
	}
	return m.NodeConnectionCIDPage(ctx, node, cid, limit)
}
func (r *tenantRouter) NodeConnectionIdentityPage(ctx context.Context, node, kind, value string, offset, limit int) (*monitoring.ConnectionPage, error) {
	m, err := r.selectedMonitor(ctx)
	if err != nil {
		return nil, err
	}
	return m.NodeConnectionIdentityPage(ctx, node, kind, value, offset, limit)
}
func (r *tenantRouter) NodeConnection(ctx context.Context, node string, cid uint64) (*monitoring.ConnectionDetail, error) {
	m, err := r.selectedMonitor(ctx)
	if err != nil {
		return nil, err
	}
	return m.NodeConnection(ctx, node, cid)
}
func (r *tenantRouter) NodeConnectionSubscriptions(ctx context.Context, node string, cid uint64) (*monitoring.ConnectionSubscriptions, error) {
	m, err := r.selectedMonitor(ctx)
	if err != nil {
		return nil, err
	}
	return m.NodeConnectionSubscriptions(ctx, node, cid)
}
func (r *tenantRouter) StatusFor(ctx context.Context) controller.Status {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		id = r.defaultID
	}
	if c := r.controllers[id]; c != nil {
		return c.Status()
	}
	return controller.Status{}
}
func (r *tenantRouter) Status() controller.Status { return r.StatusFor(context.Background()) }
func (r *tenantRouter) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, c := range r.controllers {
		wg.Add(1)
		go func(c *controller.Controller) { defer wg.Done(); c.Run(ctx) }(c)
	}
	wg.Wait()
}
func (r *tenantRouter) Close() {
	r.closeOnce.Do(func() {
		ids := make([]string, 0, len(r.clients))
		for id := range r.clients {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			r.clients[id].Close()
		}
	})
}
