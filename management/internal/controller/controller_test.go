package controller

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

type fakeBackend struct {
	leader                                          bool
	leaseErr, listErr, applyErr                     error
	declarations                                    []topology.Declaration
	leaseCalls, listCalls, applyCalls, releaseCalls int
	blocked                                         bool
}

func (f *fakeBackend) AcquireControllerLease(context.Context, string, time.Duration) (bool, error) {
	f.leaseCalls++
	return f.leader, f.leaseErr
}
func (f *fakeBackend) ReleaseControllerLease(context.Context, string) error {
	f.releaseCalls++
	return nil
}
func (f *fakeBackend) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	f.listCalls++
	return f.declarations, f.listErr
}
func (f *fakeBackend) Apply(context.Context, topology.Plan) (topology.ReconcileResult, error) {
	f.applyCalls++
	return topology.ReconcileResult{Blocked: f.blocked}, f.applyErr
}

func testController(backend Backend) *Controller {
	return New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "instance-1", true, time.Second, 3*time.Second)
}

func TestFollowerDoesNotReadDeclarations(t *testing.T) {
	backend := &fakeBackend{leader: false}
	control := testController(backend)
	control.runOnce(context.Background())
	status := control.Status()
	if status.Leader || backend.listCalls != 0 || backend.applyCalls != 0 {
		t.Fatalf("status=%#v backend=%#v", status, backend)
	}
}

func TestLeaderReconcilesEveryDeclarationAndRenewsLease(t *testing.T) {
	backend := &fakeBackend{leader: true, declarations: []topology.Declaration{{Queue: "a"}, {Queue: "b"}}}
	control := testController(backend)
	control.runOnce(context.Background())
	status := control.Status()
	if !status.Leader || status.Reconciled != 2 || status.Declarations != 2 || backend.applyCalls != 2 || backend.leaseCalls != 3 {
		t.Fatalf("status=%#v backend=%#v", status, backend)
	}
}

func TestLeaderReportsBlockedAndApplyFailure(t *testing.T) {
	backend := &fakeBackend{leader: true, blocked: true, declarations: []topology.Declaration{{Queue: "a"}}}
	control := testController(backend)
	control.runOnce(context.Background())
	if status := control.Status(); status.Blocked != 1 || status.LastError != "" {
		t.Fatalf("status=%#v", status)
	}
	backend.blocked, backend.applyErr = false, errors.New("unavailable")
	control.runOnce(context.Background())
	if status := control.Status(); status.LastError != "unavailable" {
		t.Fatalf("status=%#v", status)
	}
}

func TestDisabledControllerDoesNotAcquireLease(t *testing.T) {
	backend := &fakeBackend{leader: true}
	control := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "instance-1", false, time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	control.Run(ctx)
	if backend.leaseCalls != 0 || backend.releaseCalls != 0 || control.Status().Enabled {
		t.Fatalf("backend=%#v status=%#v", backend, control.Status())
	}
}
