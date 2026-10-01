package diagnostics

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type collectorFunc func(context.Context) (Result, error)

func (f collectorFunc) Collect(ctx context.Context) (Result, error) { return f(ctx) }

func TestStoreLifecycleOwnershipCapacityAndCopies(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	ids := []string{"11111111111111111111111111111111", "22222222222222222222222222222222"}
	store := NewStore(Config{Now: func() time.Time { return now }, NewID: func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil }, MaxRetained: 2})
	defer store.Close()
	release := make(chan struct{})
	job, err := store.Start("alice", collectorFunc(func(context.Context) (Result, error) {
		<-release
		return Result{Archive: []byte("zip"), Manifest: Manifest{Schema: "rjs.diagnostics.v1"}}, nil
	}))
	if err != nil || job.State != StateCollecting {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	if _, err = store.Start("alice", collectorFunc(nil)); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy err=%v", err)
	}
	if _, err = store.Get("bob", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ownership err=%v", err)
	}
	close(release)
	waitState(t, store, "alice", job.ID, StateReady)
	archive, err := store.Download("alice", job.ID)
	if err != nil || string(archive) != "zip" {
		t.Fatalf("archive=%q err=%v", archive, err)
	}
	archive[0] = 'X'
	again, _ := store.Download("alice", job.ID)
	if string(again) != "zip" {
		t.Fatal("download exposed store memory")
	}
	second, err := store.Start("alice", collectorFunc(func(context.Context) (Result, error) { return Result{Archive: []byte("two")}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, store, "alice", second.ID, StateReady)
	if _, err = store.Start("alice", collectorFunc(nil)); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity err=%v", err)
	}
}

func TestStoreCancelExpiryAndLateCompletion(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	store := NewStore(Config{Now: func() time.Time { return now }, TTL: time.Minute, NewID: func() (string, error) { return "11111111111111111111111111111111", nil }})
	defer store.Close()
	started, release := make(chan struct{}), make(chan struct{})
	job, _ := store.Start("alice", collectorFunc(func(context.Context) (Result, error) {
		close(started)
		<-release
		return Result{Archive: []byte("late")}, nil
	}))
	<-started
	cancelled, err := store.Cancel("alice", job.ID)
	if err != nil || cancelled.State != StateCancelled {
		t.Fatalf("job=%#v err=%v", cancelled, err)
	}
	close(release)
	time.Sleep(10 * time.Millisecond)
	got, _ := store.Get("alice", job.ID)
	if got.State != StateCancelled {
		t.Fatalf("late completion revived %#v", got)
	}
	now = now.Add(2 * time.Minute)
	expired, err := store.Get("alice", job.ID)
	if err != nil || expired.State != StateExpired {
		t.Fatalf("expired job=%#v err=%v", expired, err)
	}
	if _, err = store.Download("alice", job.ID); !errors.Is(err, ErrNotReady) {
		t.Fatalf("expired download err=%v", err)
	}
}

func TestStoreTimeoutArchiveLimitAndClose(t *testing.T) {
	var n atomic.Int32
	store := NewStore(Config{Timeout: 15 * time.Millisecond, MaxArchive: 2, NewID: func() (string, error) { n.Add(1); return "11111111111111111111111111111111", nil }})
	job, _ := store.Start("alice", collectorFunc(func(ctx context.Context) (Result, error) { <-ctx.Done(); return Result{}, ctx.Err() }))
	got := waitState(t, store, "alice", job.ID, StateFailed)
	if got.ErrorCode != "collection_timeout" {
		t.Fatalf("job=%#v", got)
	}
	store.Close()
	if _, err := store.Start("alice", collectorFunc(nil)); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed err=%v", err)
	}
}

func TestExpiredJobsRemainVisibleUntilAdmissionNeedsCapacity(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	ids := []string{"11111111111111111111111111111111", "22222222222222222222222222222222"}
	store := NewStore(Config{Now: func() time.Time { return now }, TTL: time.Minute, MaxRetained: 1, NewID: func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil }})
	defer store.Close()
	first, err := store.Start("alice", collectorFunc(func(context.Context) (Result, error) { return Result{Archive: []byte("one")}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, store, "alice", first.ID, StateReady)
	now = now.Add(2 * time.Minute)
	if expired, err := store.Get("alice", first.ID); err != nil || expired.State != StateExpired {
		t.Fatalf("expired=%#v err=%v", expired, err)
	}
	second, err := store.Start("alice", collectorFunc(func(context.Context) (Result, error) { return Result{Archive: []byte("two")}, nil }))
	if err != nil || second.ID == first.ID {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	if _, err := store.Get("alice", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reclaimed lookup err=%v", err)
	}
}

func TestExpiryTimerRemovesArchiveWithoutAnotherStoreCall(t *testing.T) {
	fired := make(chan struct{})
	store := NewStore(Config{TTL: 15 * time.Millisecond, AfterFunc: func(delay time.Duration, callback func()) func() {
		timer := time.AfterFunc(delay, func() { callback(); close(fired) })
		return func() { timer.Stop() }
	}})
	defer store.Close()
	job, err := store.Start("alice", collectorFunc(func(context.Context) (Result, error) { return Result{Archive: []byte("zip")}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, store, "alice", job.ID, StateReady)
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("expiry timer did not run")
	}
	expired, err := store.Get("alice", job.ID)
	if err != nil || expired.State != StateExpired {
		t.Fatalf("expired=%#v err=%v", expired, err)
	}
	if _, err = store.Download("alice", job.ID); !errors.Is(err, ErrNotReady) {
		t.Fatalf("download err=%v", err)
	}
}

func waitState(t *testing.T, store *Store, owner, id, state string) Job {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		job, err := store.Get(owner, id)
		if err == nil && job.State == state {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	job, err := store.Get(owner, id)
	t.Fatalf("state=%#v err=%v, want %s", job, err, state)
	return Job{}
}
