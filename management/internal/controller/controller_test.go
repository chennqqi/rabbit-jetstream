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
	dlqResult                                       topology.DeadLetterProcessResult
	dlqErr                                          error
	leaderResults                                   []bool
}

func (f *fakeBackend) ProcessDeadLetters(context.Context, []topology.Declaration, int) (topology.DeadLetterProcessResult, error) {
	return f.dlqResult, f.dlqErr
}

func (f *fakeBackend) AcquireControllerLease(context.Context, string, time.Duration) (bool, error) {
	f.leaseCalls++
	if len(f.leaderResults) > 0 {
		result := f.leaderResults[0]
		f.leaderResults = f.leaderResults[1:]
		return result, f.leaseErr
	}
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
func (f *fakeBackend) ApplyDeclaration(context.Context, topology.Declaration) (topology.ReconcileResult, error) {
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
	backend := &fakeBackend{leader: true, declarations: []topology.Declaration{{Queue: "a"}, {Queue: "b"}}, dlqResult: topology.DeadLetterProcessResult{Processed: 2, Moved: 1}}
	control := testController(backend)
	control.runOnce(context.Background())
	status := control.Status()
	if !status.Leader || status.Reconciled != 2 || status.Declarations != 2 || status.DLQProcessed != 2 || status.DLQMoved != 1 || backend.applyCalls != 2 || backend.leaseCalls != 3 {
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

func TestLeaderReportsDeadLetterFailure(t *testing.T) {
	backend := &fakeBackend{leader: true, dlqErr: errors.New("dlq unavailable")}
	control := testController(backend)
	control.runOnce(context.Background())
	if status := control.Status(); status.LastError != "dlq unavailable" || !status.Leader {
		t.Fatalf("status=%#v", status)
	}
}

func TestControllerReportsLeaseAndListFailures(t *testing.T) {
	backend := &fakeBackend{leader: true, leaseErr: errors.New("lease unavailable")}
	control := testController(backend)
	control.runOnce(context.Background())
	if status := control.Status(); status.Leader || status.LastError != "lease unavailable" || backend.listCalls != 0 {
		t.Fatalf("status=%#v backend=%#v", status, backend)
	}
	backend.leaseErr, backend.listErr = nil, errors.New("list unavailable")
	control.runOnce(context.Background())
	if status := control.Status(); !status.Leader || status.LastError != "list unavailable" || backend.listCalls != 1 {
		t.Fatalf("status=%#v backend=%#v", status, backend)
	}
}

func TestControllerStopsWhenLeaseIsLostDuringReconcile(t *testing.T) {
	backend := &fakeBackend{leader: true, leaderResults: []bool{true, true, false}, declarations: []topology.Declaration{{Queue: "a"}, {Queue: "b"}}}
	control := testController(backend)
	control.runOnce(context.Background())
	if status := control.Status(); status.Leader || backend.applyCalls != 1 || status.Reconciled != 1 {
		t.Fatalf("status=%#v backend=%#v", status, backend)
	}
}

func TestControllerAccumulatesDeadLetterCounters(t *testing.T) {
	backend := &fakeBackend{leader: true, dlqResult: topology.DeadLetterProcessResult{Processed: 3, Moved: 1, Failed: 1, Ignored: 1}}
	control := testController(backend)
	control.runOnce(context.Background())
	control.runOnce(context.Background())
	status := control.Status()
	if status.DLQProcessed != 6 || status.DLQMoved != 2 || status.DLQFailed != 2 || status.DLQIgnored != 2 {
		t.Fatalf("status=%#v", status)
	}
}

func TestControllerRetainsPartialDeadLetterResults(t *testing.T) {
	for _, batchErr := range []error{errors.New("ack DLQ advisory: unavailable"), errors.New("read DLQ advisory batch: unavailable"), context.DeadlineExceeded} {
		t.Run(batchErr.Error(), func(t *testing.T) {
			backend := &fakeBackend{leader: true, declarations: []topology.Declaration{{Queue: "a"}}, dlqResult: topology.DeadLetterProcessResult{Processed: 3, Moved: 1, Failed: 1, Ignored: 1}}
			control := testController(backend)
			control.runOnce(context.Background())
			lastSuccess := control.Status().LastSuccess
			backend.dlqResult = topology.DeadLetterProcessResult{Processed: 4, Moved: 2, Failed: 1, Ignored: 1}
			backend.dlqErr = batchErr
			control.runOnce(context.Background())
			status := control.Status()
			if status.DLQProcessed != 7 || status.DLQMoved != 3 || status.DLQFailed != 2 || status.DLQIgnored != 2 {
				t.Fatalf("partial batch counters lost: %#v", status)
			}
			if status.LastError != batchErr.Error() || !status.LastSuccess.Equal(lastSuccess) || !status.Leader || status.Reconciled != 1 {
				t.Fatalf("partial batch must retain failure and prior success: %#v", status)
			}
			backend.dlqResult = topology.DeadLetterProcessResult{}
			control.runOnce(context.Background())
			if got := control.Status(); got.DLQProcessed != 7 || got.DLQMoved != 3 || got.DLQFailed != 2 || got.DLQIgnored != 2 {
				t.Fatalf("empty failed batch changed counters: %#v", got)
			}
			backend.dlqErr = nil
			backend.dlqResult = topology.DeadLetterProcessResult{Processed: 1, Moved: 1}
			control.runOnce(context.Background())
			if got := control.Status(); got.DLQProcessed != 8 || got.DLQMoved != 4 || got.DLQFailed != 2 || got.DLQIgnored != 2 || got.LastError != "" || !got.LastSuccess.Equal(got.LastRun) {
				t.Fatalf("recovered batch did not accumulate exactly once: %#v", got)
			}
		})
	}
}

func TestControllerRunReleasesLease(t *testing.T) {
	backend := &fakeBackend{leader: true}
	control := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "instance-1", true, time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	control.Run(ctx)
	if backend.releaseCalls != 1 || backend.leaseCalls != 1 {
		t.Fatalf("backend=%#v", backend)
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
