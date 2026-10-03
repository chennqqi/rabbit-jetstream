package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

const (
	globalConsumerMaxStreams   = 10000
	globalConsumerMaxRows      = 100000
	globalConsumerMaxBytes     = 128 << 20
	globalConsumerWorkers      = 16
	globalConsumerCollectLimit = 30 * time.Second
)

// globalConsumerRow is deliberately a list projection. Exact configuration
// remains on the existing Stream-scoped Consumer detail endpoint.
type globalConsumerRow struct {
	Queue      string                 `json:"queue,omitempty"`
	Stream     string                 `json:"stream"`
	Name       string                 `json:"name"`
	Durable    string                 `json:"durable,omitempty"`
	Mode       string                 `json:"mode"`
	Status     string                 `json:"status"`
	Ownership  string                 `json:"ownership"`
	Pending    *uint64                `json:"pending"`
	AckPending *int                   `json:"ack_pending"`
	Expected   *topology.ConsumerPlan `json:"-"`
}

type globalConsumerGeneration struct {
	ID          string              `json:"generation_id"`
	StartedAt   time.Time           `json:"started_at"`
	CompletedAt time.Time           `json:"completed_at"`
	Rows        []globalConsumerRow `json:"-"`
}

type globalConsumerIndex struct {
	mu         sync.RWMutex
	generation *globalConsumerGeneration
	refreshing bool
	failedAt   *time.Time
}

type globalConsumerIndexStatus struct {
	State      string                    `json:"state"`
	Generation *globalConsumerGeneration `json:"generation,omitempty"`
	FailedAt   *time.Time                `json:"failed_at,omitempty"`
}

func (index *globalConsumerIndex) status() globalConsumerIndexStatus {
	index.mu.RLock()
	defer index.mu.RUnlock()
	state := "unavailable"
	if index.refreshing {
		state = "collecting"
	}
	if index.generation != nil {
		state = "ready"
		if index.failedAt != nil {
			state = "stale"
		}
	}
	return globalConsumerIndexStatus{State: state, Generation: index.generation, FailedAt: index.failedAt}
}

func (index *globalConsumerIndex) refresh(ctx context.Context, source globalConsumerSource, now func() time.Time) (*globalConsumerGeneration, bool, error) {
	index.mu.Lock()
	if index.refreshing {
		index.mu.Unlock()
		return nil, true, nil
	}
	index.refreshing = true
	index.mu.Unlock()
	generation, err := collectGlobalConsumers(ctx, source, now)
	index.mu.Lock()
	defer index.mu.Unlock()
	index.refreshing = false
	if err != nil {
		failed := now().UTC()
		index.failedAt = &failed
		return nil, false, err
	}
	index.generation, index.failedAt = generation, nil
	return generation, false, nil
}

type globalConsumerQuery struct {
	search, queue, stream, mode, state, generation string
	descending                                     bool
	offset, limit                                  int
}

func queryGlobalConsumers(rows []globalConsumerRow, query globalConsumerQuery) []globalConsumerRow {
	result := make([]globalConsumerRow, 0)
	for _, row := range rows {
		if query.queue != "" && row.Queue != query.queue || query.stream != "" && row.Stream != query.stream || query.mode != "" && row.Mode != query.mode || query.state != "" && row.Status != query.state {
			continue
		}
		matches := query.search == "" || strings.Contains(strings.ToLower(row.Queue), query.search) || strings.Contains(strings.ToLower(row.Stream), query.search) || strings.Contains(strings.ToLower(row.Name), query.search) || strings.Contains(strings.ToLower(row.Durable), query.search)
		if !matches {
			continue
		}
		result = append(result, row)
	}
	sort.SliceStable(result, func(i, j int) bool {
		less := result[i].Stream < result[j].Stream || result[i].Stream == result[j].Stream && result[i].Name < result[j].Name
		if query.descending {
			return !less && (result[i].Stream != result[j].Stream || result[i].Name != result[j].Name)
		}
		return less
	})
	return result
}

type globalConsumerSource interface {
	ListStreams(context.Context) ([]jetstream.Stream, error)
	ListConsumers(context.Context, string) ([]jetstream.Consumer, error)
	ListDeclarations(context.Context) ([]topology.Declaration, error)
}

// collectGlobalConsumers publishes nothing: callers may atomically replace a
// completed generation only after this function succeeds in full.
func collectGlobalConsumers(parent context.Context, source globalConsumerSource, now func() time.Time) (*globalConsumerGeneration, error) {
	ctx, cancel := context.WithTimeout(parent, globalConsumerCollectLimit)
	defer cancel()
	started := now().UTC()
	declarations, err := source.ListDeclarations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list declarations: %w", err)
	}
	streams, err := source.ListStreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("list streams: %w", err)
	}
	if len(streams) > globalConsumerMaxStreams {
		return nil, fmt.Errorf("global Consumer Stream limit %d exceeded", globalConsumerMaxStreams)
	}

	type declaredStream struct {
		queue    string
		revision uint64
		plans    map[string]topology.ConsumerPlan
		owned    bool
	}
	byStream := make(map[string]declaredStream, len(declarations))
	for _, declaration := range declarations {
		plan := declaration.Plan
		if declaration.Queue == "" || plan.Queue != declaration.Queue || plan.Stream.Name == "" {
			return nil, errors.New("inconsistent Queue declaration identity")
		}
		if _, duplicate := byStream[plan.Stream.Name]; duplicate {
			return nil, errors.New("duplicate declared Stream identity")
		}
		plans := make(map[string]topology.ConsumerPlan)
		for _, consumer := range append([]topology.ConsumerPlan{plan.Consumer}, plan.PriorityConsumers...) {
			if consumer.Stream != plan.Stream.Name || consumer.Name == "" {
				return nil, errors.New("inconsistent declared Consumer identity")
			}
			if _, duplicate := plans[consumer.Name]; duplicate {
				return nil, errors.New("duplicate declared Consumer identity")
			}
			plans[consumer.Name] = consumer
		}
		byStream[plan.Stream.Name] = declaredStream{queue: declaration.Queue, revision: declaration.KVRevision, plans: plans}
	}
	for _, stream := range streams {
		declared, ok := byStream[stream.Name]
		if ok {
			declared.owned = stream.Metadata["rabbit-jetstream.io/queue"] == declared.queue
			byStream[stream.Name] = declared
		}
	}

	type result struct {
		stream    string
		consumers []jetstream.Consumer
		err       error
	}
	jobs, results := make(chan string), make(chan result, len(streams))
	var workers sync.WaitGroup
	for i := 0; i < globalConsumerWorkers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for name := range jobs {
				items, err := source.ListConsumers(ctx, name)
				results <- result{name, items, err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, stream := range streams {
			select {
			case jobs <- stream.Name:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { workers.Wait(); close(results) }()
	observed := make(map[string]map[string]jetstream.Consumer, len(streams))
	for result := range results {
		if result.err != nil {
			cancel()
			return nil, fmt.Errorf("list Consumers for Stream %s: %w", result.stream, result.err)
		}
		members := make(map[string]jetstream.Consumer, len(result.consumers))
		for _, consumer := range result.consumers {
			if consumer.Stream != result.stream || consumer.Name == "" {
				cancel()
				return nil, errors.New("inconsistent Consumer enumeration")
			}
			if _, duplicate := members[consumer.Name]; duplicate {
				cancel()
				return nil, errors.New("duplicate observed Consumer identity")
			}
			members[consumer.Name] = consumer
		}
		observed[result.stream] = members
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows := make([]globalConsumerRow, 0)
	encodedBytes := 2
	add := func(row globalConsumerRow) error {
		if len(rows) >= globalConsumerMaxRows {
			return fmt.Errorf("global Consumer row limit %d exceeded", globalConsumerMaxRows)
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode global Consumer row: %w", err)
		}
		encodedBytes += len(encoded) + 1
		if encodedBytes > globalConsumerMaxBytes {
			return fmt.Errorf("global Consumer byte limit %d exceeded", globalConsumerMaxBytes)
		}
		rows = append(rows, row)
		return nil
	}
	seenStreams := make(map[string]bool, len(streams))
	for _, stream := range streams {
		if stream.Name == "" || seenStreams[stream.Name] {
			return nil, errors.New("duplicate or empty Stream identity")
		}
		seenStreams[stream.Name] = true
		declared, managed := byStream[stream.Name]
		members := observed[stream.Name]
		if managed {
			for name, expected := range declared.plans {
				consumer, present := members[name]
				row := globalConsumerRow{Queue: declared.queue, Stream: stream.Name, Name: name, Mode: expected.Mode, Status: "missing", Ownership: "unknown", Expected: &expected}
				if present {
					row = observedGlobalConsumer(consumer, declared.queue, declared.owned)
					row.Expected = &expected
					row.Status = "present"
					if !consumerMatchesPlan(consumer, expected) {
						row.Status = "mismatched"
					}
					delete(members, name)
				}
				if err := add(row); err != nil {
					return nil, err
				}
			}
		}
		for _, consumer := range members {
			queue := ""
			owned := false
			if managed {
				queue, owned = declared.queue, declared.owned
			}
			if err := add(observedGlobalConsumer(consumer, queue, owned)); err != nil {
				return nil, err
			}
		}
	}
	for stream, declared := range byStream {
		if seenStreams[stream] {
			continue
		}
		for name, expected := range declared.plans {
			expected := expected
			if err := add(globalConsumerRow{Queue: declared.queue, Stream: stream, Name: name, Mode: expected.Mode, Status: "missing", Ownership: "unknown", Expected: &expected}); err != nil {
				return nil, err
			}
		}
	}
	current, err := source.ListDeclarations(ctx)
	if err != nil {
		return nil, fmt.Errorf("recheck declarations: %w", err)
	}
	if !sameDeclarationSet(declarations, current) {
		return nil, errors.New("Queue declarations changed during global Consumer collection")
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Stream != rows[j].Stream {
			return rows[i].Stream < rows[j].Stream
		}
		return rows[i].Name < rows[j].Name
	})
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, fmt.Errorf("create generation identity: %w", err)
	}
	return &globalConsumerGeneration{ID: hex.EncodeToString(random[:]), StartedAt: started, CompletedAt: now().UTC(), Rows: rows}, nil
}

func observedGlobalConsumer(consumer jetstream.Consumer, queue string, streamOwned bool) globalConsumerRow {
	pending, ack := consumer.Pending, consumer.AckPending
	ownership := "external"
	if queue != "" {
		if streamOwned && consumer.Metadata["rabbit-jetstream.io/queue"] == queue {
			ownership = "matching"
		} else {
			ownership = "different"
		}
	}
	return globalConsumerRow{Queue: queue, Stream: consumer.Stream, Name: consumer.Name, Durable: consumer.Durable, Mode: consumer.Mode, Status: "present", Ownership: ownership, Pending: &pending, AckPending: &ack}
}

func consumerMatchesPlan(actual jetstream.Consumer, expected topology.ConsumerPlan) bool {
	if actual.Mode != expected.Mode || actual.DeliverPolicy != expected.DeliverPolicy || actual.AckPolicy != expected.AckPolicy || actual.AckWaitNanos != expected.AckWaitNanos || actual.MaxDeliver != expected.MaxDeliver || actual.ReplayPolicy != expected.ReplayPolicy {
		return false
	}
	left, right := append([]string(nil), actual.FilterSubjects...), append([]string(nil), expected.FilterSubjects...)
	sort.Strings(left)
	sort.Strings(right)
	return slices.Equal(left, right)
}

func sameDeclarationSet(left, right []topology.Declaration) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[string]uint64, len(left))
	for _, item := range left {
		if item.Queue == "" || item.KVRevision == 0 {
			return false
		}
		if _, duplicate := values[item.Queue]; duplicate {
			return false
		}
		values[item.Queue] = item.KVRevision
	}
	seen := make(map[string]bool, len(right))
	for _, item := range right {
		if seen[item.Queue] || values[item.Queue] != item.KVRevision || item.KVRevision == 0 {
			return false
		}
		seen[item.Queue] = true
	}
	return true
}
