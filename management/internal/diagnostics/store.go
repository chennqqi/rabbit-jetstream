// Package diagnostics owns the bounded, instance-local lifecycle for metadata
// diagnostic bundles. It does not know about HTTP credentials or filesystems.
package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const (
	StateCollecting = "collecting"
	StateReady      = "ready"
	StatePartial    = "partial"
	StateFailed     = "failed"
	StateCancelled  = "cancelled"
	StateExpired    = "expired"
)

var (
	ErrBusy     = errors.New("diagnostic collection is already active")
	ErrCapacity = errors.New("diagnostic job capacity is exhausted")
	ErrNotFound = errors.New("diagnostic job not found")
	ErrNotReady = errors.New("diagnostic bundle is not available")
	ErrClosed   = errors.New("diagnostic job store is closed")
)

type Result struct {
	Archive  []byte
	Partial  bool
	Manifest Manifest
}

type Manifest struct {
	Schema      string          `json:"schema"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Entries     []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	Source      string    `json:"source"`
	File        string    `json:"file,omitempty"`
	CollectedAt time.Time `json:"collectedAt"`
	Size        int       `json:"size,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type Collector interface {
	Collect(context.Context) (Result, error)
}

type Job struct {
	ID         string     `json:"id"`
	Profile    string     `json:"profile"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Manifest   *Manifest  `json:"manifest,omitempty"`
	ErrorCode  string     `json:"errorCode,omitempty"`
}

type Config struct {
	Now         func() time.Time
	AfterFunc   func(time.Duration, func()) func()
	TTL         time.Duration
	Timeout     time.Duration
	MaxRetained int
	MaxArchive  int
	NewID       func() (string, error)
}

type storedJob struct {
	view       Job
	owner      string
	archive    []byte
	cancel     context.CancelFunc
	stopExpiry func()
	epoch      uint64
}

type Store struct {
	mu     sync.Mutex
	cfg    Config
	jobs   map[string]*storedJob
	active string
	closed bool
	wg     sync.WaitGroup
}

func NewStore(cfg Config) *Store {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.AfterFunc == nil {
		cfg.AfterFunc = func(delay time.Duration, callback func()) func() {
			timer := time.AfterFunc(delay, callback)
			return func() { timer.Stop() }
		}
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 10 * time.Minute
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetained <= 0 {
		cfg.MaxRetained = 2
	}
	if cfg.MaxArchive <= 0 {
		cfg.MaxArchive = 8 << 20
	}
	if cfg.NewID == nil {
		cfg.NewID = randomID
	}
	return &Store{cfg: cfg, jobs: make(map[string]*storedJob)}
}

func (s *Store) Start(owner string, collector Collector) (Job, error) {
	return s.start(owner, "", collector, true)
}

// StartWithID admits an externally reserved cryptographically random identity.
// It lets the HTTP layer persist an audit intent before collection begins.
func (s *Store) StartWithID(owner, id string, collector Collector) (Job, error) {
	return s.start(owner, id, collector, false)
}

func (s *Store) start(owner, id string, collector Collector, generateID bool) (Job, error) {
	if owner == "" || collector == nil {
		return Job{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	if s.closed {
		return Job{}, ErrClosed
	}
	if s.active != "" {
		return Job{}, ErrBusy
	}
	s.reclaimExpiredLocked()
	if len(s.jobs) >= s.cfg.MaxRetained {
		return Job{}, ErrCapacity
	}
	if generateID {
		var err error
		id, err = s.cfg.NewID()
		if err != nil {
			return Job{}, err
		}
	}
	if len(id) < 32 || s.jobs[id] != nil {
		return Job{}, errors.New("diagnostic job ID is invalid or duplicated")
	}
	now := s.cfg.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout)
	entry := &storedJob{owner: owner, cancel: cancel, epoch: 1, view: Job{ID: id, Profile: "metadata-v1", State: StateCollecting, CreatedAt: now, ExpiresAt: now.Add(s.cfg.TTL)}}
	s.jobs[id], s.active = entry, id
	entry.stopExpiry = s.cfg.AfterFunc(s.cfg.TTL, func() { s.expireID(id) })
	s.wg.Add(1)
	go s.collect(ctx, id, entry.epoch, collector)
	return entry.view, nil
}

func (s *Store) Get(owner, id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	entry := s.jobs[id]
	if entry == nil || entry.owner != owner {
		return Job{}, ErrNotFound
	}
	return cloneJob(entry.view), nil
}

func (s *Store) Download(owner, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	entry := s.jobs[id]
	if entry == nil || entry.owner != owner {
		return nil, ErrNotFound
	}
	if entry.view.State != StateReady && entry.view.State != StatePartial {
		return nil, ErrNotReady
	}
	return append([]byte(nil), entry.archive...), nil
}

func (s *Store) Cancel(owner, id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	entry := s.jobs[id]
	if entry == nil || entry.owner != owner {
		return Job{}, ErrNotFound
	}
	if entry.view.State == StateCollecting {
		entry.epoch++
		entry.cancel()
		entry.cancel = nil
		entry.archive = nil
		entry.view.State, entry.view.ErrorCode = StateCancelled, "cancelled"
		now := s.cfg.Now().UTC()
		entry.view.FinishedAt = &now
		if s.active == id {
			s.active = ""
		}
	}
	return cloneJob(entry.view), nil
}

func (s *Store) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.wg.Wait()
		return
	}
	s.closed = true
	for _, entry := range s.jobs {
		entry.epoch++
		if entry.stopExpiry != nil {
			entry.stopExpiry()
		}
		if entry.cancel != nil {
			entry.cancel()
		}
		entry.archive = nil
	}
	s.jobs = make(map[string]*storedJob)
	s.active = ""
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Store) collect(ctx context.Context, id string, epoch uint64, collector Collector) {
	defer s.wg.Done()
	result, err := collector.Collect(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.jobs[id]
	if entry == nil || entry.epoch != epoch || s.closed || entry.view.State != StateCollecting {
		return
	}
	entry.cancel()
	entry.cancel = nil
	if s.active == id {
		s.active = ""
	}
	now := s.cfg.Now().UTC()
	entry.view.FinishedAt = &now
	if err != nil {
		entry.view.State, entry.view.ErrorCode = StateFailed, "collection_failed"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			entry.view.ErrorCode = "collection_timeout"
		}
		return
	}
	if len(result.Archive) == 0 || len(result.Archive) > s.cfg.MaxArchive {
		entry.view.State, entry.view.ErrorCode = StateFailed, "archive_limit_exceeded"
		return
	}
	entry.archive = append([]byte(nil), result.Archive...)
	manifest := cloneManifest(result.Manifest)
	entry.view.Manifest = &manifest
	entry.view.State = StateReady
	if result.Partial {
		entry.view.State = StatePartial
	}
}

func (s *Store) expireLocked() {
	now := s.cfg.Now().UTC()
	for id, entry := range s.jobs {
		if now.Before(entry.view.ExpiresAt) {
			continue
		}
		entry.epoch++
		if entry.cancel != nil {
			entry.cancel()
			entry.cancel = nil
		}
		entry.archive = nil
		entry.view.State, entry.view.ErrorCode = StateExpired, "expired"
		if s.active == id {
			s.active = ""
		}
		// Expired identities remain visible until a new admission needs capacity.
	}
}

func (s *Store) expireID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.jobs[id]
	if entry == nil || s.closed {
		return
	}
	entry.epoch++
	if entry.cancel != nil {
		entry.cancel()
		entry.cancel = nil
	}
	entry.archive = nil
	entry.view.State, entry.view.ErrorCode = StateExpired, "expired"
	if s.active == id {
		s.active = ""
	}
}

func (s *Store) reclaimExpiredLocked() {
	for len(s.jobs) >= s.cfg.MaxRetained {
		oldestID := ""
		var oldest time.Time
		for id, entry := range s.jobs {
			if entry.view.State != StateExpired {
				continue
			}
			if oldestID == "" || entry.view.CreatedAt.Before(oldest) {
				oldestID, oldest = id, entry.view.CreatedAt
			}
		}
		if oldestID == "" {
			return
		}
		delete(s.jobs, oldestID)
	}
}

func cloneJob(in Job) Job {
	out := in
	if in.FinishedAt != nil {
		value := *in.FinishedAt
		out.FinishedAt = &value
	}
	if in.Manifest != nil {
		value := cloneManifest(*in.Manifest)
		out.Manifest = &value
	}
	return out
}

func cloneManifest(in Manifest) Manifest {
	out := in
	out.Entries = append([]ManifestEntry(nil), in.Entries...)
	return out
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
