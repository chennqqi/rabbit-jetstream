package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/chennqqi/rabbit-jetstream/internal/natsclient"
)

type report struct {
	Schema                    string    `json:"schema"`
	NATSVersion               string    `json:"nats_version"`
	GOOS                      string    `json:"goos"`
	GOARCH                    string    `json:"goarch"`
	CPUs                      int       `json:"cpus"`
	Replicas                  int       `json:"replicas"`
	PayloadBytes              int       `json:"payload_bytes"`
	Publishers                int       `json:"publishers"`
	Batch                     int       `json:"batch"`
	TargetPublishRate         int64     `json:"target_publish_messages_per_second,omitempty"`
	WorkloadMode              string    `json:"workload_mode"`
	ConfiguredDurationSeconds float64   `json:"configured_duration_seconds,omitempty"`
	RequestedMessages         int64     `json:"requested_messages"`
	Published                 int64     `json:"published"`
	Consumed                  int64     `json:"consumed"`
	Missing                   int64     `json:"missing"`
	Duplicates                int64     `json:"duplicates"`
	Corrupt                   int64     `json:"corrupt"`
	StartedAt                 time.Time `json:"started_at"`
	FinishedAt                time.Time `json:"finished_at"`
	DurationSeconds           float64   `json:"duration_seconds"`
	PublishMessagesPerSecond  float64   `json:"publish_messages_per_second"`
	ConsumeMessagesPerSecond  float64   `json:"consume_messages_per_second"`
	PublishLatencyP50Millis   float64   `json:"publish_latency_p50_millis"`
	PublishLatencyP95Millis   float64   `json:"publish_latency_p95_millis"`
	PublishLatencyP99Millis   float64   `json:"publish_latency_p99_millis"`
	PublishLatencyMaxMillis   float64   `json:"publish_latency_max_millis"`
	PublishRetries            int64     `json:"publish_retries"`
	ConsumeRetries            int64     `json:"consume_retries"`
	AllowRedeliveries         bool      `json:"allow_redeliveries,omitempty"`
	ConsumerStartDelayMillis  float64   `json:"consumer_start_delay_millis"`
	ConsumerDelayMillis       float64   `json:"consumer_delay_millis"`
	PeakBacklogMessages       int64     `json:"peak_backlog_messages"`
	BacklogAtPublishEnd       int64     `json:"backlog_at_publish_end"`
	DrainSeconds              float64   `json:"drain_seconds"`
	DrainMessagesPerSecond    float64   `json:"drain_messages_per_second"`
}

type latencySamples struct {
	mu      sync.Mutex
	buckets [1_000_001]uint64
	count   uint64
	max     time.Duration
}

func (s *latencySamples) add(value time.Duration) {
	s.mu.Lock()
	index := int((value + 10*time.Microsecond - 1) / (10 * time.Microsecond))
	if index >= len(s.buckets) {
		index = len(s.buckets) - 1
	}
	s.buckets[index]++
	s.count++
	if value > s.max {
		s.max = value
	}
	s.mu.Unlock()
}
func (s *latencySamples) summary() (float64, float64, float64, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count == 0 {
		return 0, 0, 0, 0
	}
	at := func(q float64) float64 {
		target := uint64(float64(s.count-1)*q) + 1
		var cumulative uint64
		for index, count := range s.buckets {
			cumulative += count
			if cumulative >= target {
				return float64(index) / 100
			}
		}
		return 10000
	}
	return at(.50), at(.95), at(.99), float64(s.max) / float64(time.Millisecond)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	server := flag.String("server", "nats://127.0.0.1:4222", "NATS URLs")
	output := flag.String("output", "", "new JSON report")
	messages := flag.Int64("messages", 10000, "messages to publish; 0 uses duration")
	duration := flag.Duration("duration", 0, "time-based publish duration")
	payloadBytes := flag.Int("payload-bytes", 1024, "payload bytes, at least 16")
	publishers := flag.Int("publishers", 4, "concurrent synchronous publishers")
	publishRate := flag.Int64("publish-rate", 0, "maximum generated messages per second; 0 is unlimited")
	publishRetryTimeout := flag.Duration("publish-retry-timeout", 2*time.Minute, "maximum time to retry a publish with the same message ID")
	consumeRetryTimeout := flag.Duration("consume-retry-timeout", 2*time.Minute, "maximum continuous transient consumer failure")
	allowRedeliveries := flag.Bool("allow-redeliveries", false, "allow and count at-least-once redeliveries while requiring every unique message")
	batch := flag.Int("batch", 256, "consumer fetch batch")
	replicas := flag.Int("replicas", 3, "Stream replicas")
	timeout := flag.Duration("timeout", 10*time.Minute, "drain timeout after publishing")
	consumerStartDelay := flag.Duration("consumer-start-delay", 0, "delay before consuming to create backlog")
	consumerDelay := flag.Duration("consumer-delay", 0, "per-message processing delay before acknowledgement")
	flag.Parse()
	if *output == "" || *messages < 0 || (*messages == 0 && *duration <= 0) || *payloadBytes < 16 || *publishers < 1 || *publishRate < 0 || *publishRetryTimeout <= 0 || *consumeRetryTimeout <= 0 || *batch < 1 || (*replicas != 1 && *replicas != 3 && *replicas != 5) || *timeout <= 0 || *consumerStartDelay < 0 || *consumerDelay < 0 {
		return errors.New("invalid benchmark arguments")
	}
	connectionOptions, err := natsclient.Options(natsclient.FromEnv())
	if err != nil {
		return err
	}
	connectionOptions = append(connectionOptions, nats.Timeout(10*time.Second), nats.MaxReconnects(-1))
	nc, err := nats.Connect(*server, connectionOptions...)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(*publishers * 4))
	if err != nil {
		return err
	}
	stream := "RJS_PERF_" + fmt.Sprint(time.Now().UnixNano())
	subject := "rjs.perf." + fmt.Sprint(time.Now().UnixNano())
	_, err = js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{subject}, Storage: nats.FileStorage, Retention: nats.WorkQueuePolicy, Discard: nats.DiscardOld, Replicas: *replicas, MaxAge: 48 * time.Hour})
	if err != nil {
		return fmt.Errorf("create Stream: %w", err)
	}
	defer js.DeleteStream(stream)
	sub, err := js.PullSubscribe(subject, "RJS_PERF_CONSUMER", nats.BindStream(stream), nats.ManualAck(), nats.AckExplicit(), nats.AckWait(2*time.Minute), nats.MaxDeliver(5))
	if err != nil {
		return err
	}
	started := time.Now().UTC()
	jobs := make(chan int64, *publishers*2)
	publishErr := make(chan error, 1)
	var generated, published, publishRetries atomic.Int64
	samples := &latencySamples{}
	var workers sync.WaitGroup
	for worker := 0; worker < *publishers; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				payload := makePayload(id, *payloadBytes)
				msg := nats.NewMsg(subject)
				msg.Header.Set("Nats-Msg-Id", fmt.Sprintf("perf-%d", id))
				msg.Data = payload
				begin := time.Now()
				retries, err := publishWithRetry(js, msg, *publishRetryTimeout)
				publishRetries.Add(retries)
				if err != nil {
					select {
					case publishErr <- err:
					default:
						{
						}
					}
					continue
				}
				samples.add(time.Since(begin))
				published.Add(1)
			}
		}()
	}
	publishDone := make(chan struct{})
	go func() {
		defer close(jobs)
		defer close(publishDone)
		generationStarted := time.Now()
		deadline := time.Now().Add(*duration)
		for id := int64(0); ; id++ {
			if *messages > 0 && id >= *messages {
				return
			}
			if *messages == 0 && time.Now().After(deadline) {
				return
			}
			if target := rateDeadline(generationStarted, id, *publishRate); !target.IsZero() {
				if delay := time.Until(target); delay > 0 {
					time.Sleep(delay)
				}
			}
			generated.Add(1)
			jobs <- id
		}
	}()
	seen := make([]uint64, 0)
	var consumed, duplicates, corrupt int64
	var consumeRetries int64
	var consumeFailureStarted time.Time
	consumeStarted := time.Now()
	if *consumerStartDelay > 0 {
		time.Sleep(*consumerStartDelay)
	}
	publishFinished := time.Time{}
	publishingDone := false
	drainDeadline := time.Time{}
	var peakBacklog, backlogAtPublishEnd int64
	for {
		backlog := published.Load() - consumed
		if backlog > peakBacklog {
			peakBacklog = backlog
		}
		if publishingDone && consumed >= published.Load() && published.Load() == generated.Load() {
			break
		}
		if publishingDone && time.Now().After(drainDeadline) {
			return errors.New("consumer did not drain all published messages before timeout")
		}
		select {
		case err := <-publishErr:
			return fmt.Errorf("publish: %w", err)
		default:
			{
			}
		}
		fetchContext, fetchCancel := context.WithTimeout(context.Background(), time.Second)
		msgs, fetchErr := sub.Fetch(*batch, nats.Context(fetchContext))
		fetchCancel()
		if fetchErr != nil && !errors.Is(fetchErr, nats.ErrTimeout) && !errors.Is(fetchErr, context.DeadlineExceeded) {
			if !transientConsumeError(fetchErr) {
				return fmt.Errorf("consume: %w", fetchErr)
			}
			if consumeFailureStarted.IsZero() {
				consumeFailureStarted = time.Now()
			}
			if time.Since(consumeFailureStarted) > *consumeRetryTimeout {
				return fmt.Errorf("consume unavailable for %s: %w", *consumeRetryTimeout, fetchErr)
			}
			consumeRetries++
			time.Sleep(100 * time.Millisecond)
			continue
		}
		consumeFailureStarted = time.Time{}
		for _, msg := range msgs {
			id, ok := validatePayload(msg.Data, *payloadBytes)
			if !ok || id < 0 {
				corrupt++
				consumed++
			} else {
				word := int(id / 64)
				for len(seen) <= word {
					seen = append(seen, 0)
				}
				mask := uint64(1) << uint(id%64)
				if seen[word]&mask != 0 {
					duplicates++
				} else {
					seen[word] |= mask
					consumed++
				}
			}
			if *consumerDelay > 0 {
				time.Sleep(*consumerDelay)
			}
			if err := msg.Ack(); err != nil {
				return err
			}
		}
		if !publishingDone {
			select {
			case <-publishDone:
				workers.Wait()
				publishFinished = time.Now().UTC()
				backlogAtPublishEnd = published.Load() - consumed
				drainDeadline = time.Now().Add(*timeout)
				publishingDone = true
			default:
			}
		}
	}
	if err := nc.Flush(); err != nil {
		return err
	}
	finished := time.Now().UTC()
	if publishFinished.IsZero() {
		publishFinished = finished
	}
	expected := generated.Load()
	missing := expected - consumed
	if missing < 0 {
		missing = 0
	}
	p50, p95, p99, max := samples.summary()
	elapsed := finished.Sub(started).Seconds()
	publishElapsed := publishFinished.Sub(started).Seconds()
	consumeElapsed := finished.Sub(consumeStarted).Seconds()
	drainSeconds := finished.Sub(publishFinished).Seconds()
	drainRate := float64(0)
	if backlogAtPublishEnd > 0 && drainSeconds > 0 {
		drainRate = float64(backlogAtPublishEnd) / drainSeconds
	}
	mode := "count"
	if *messages == 0 {
		mode = "duration"
	}
	value := report{Schema: "rabbit-jetstream.io/performance-report/v1alpha1", NATSVersion: nc.ConnectedServerVersion(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPUs: runtime.NumCPU(), Replicas: *replicas, PayloadBytes: *payloadBytes, Publishers: *publishers, Batch: *batch, TargetPublishRate: *publishRate, WorkloadMode: mode, ConfiguredDurationSeconds: duration.Seconds(), RequestedMessages: expected, Published: published.Load(), Consumed: consumed, Missing: missing, Duplicates: duplicates, Corrupt: corrupt, StartedAt: started, FinishedAt: finished, DurationSeconds: elapsed, PublishMessagesPerSecond: float64(published.Load()) / publishElapsed, ConsumeMessagesPerSecond: float64(consumed) / consumeElapsed, PublishLatencyP50Millis: p50, PublishLatencyP95Millis: p95, PublishLatencyP99Millis: p99, PublishLatencyMaxMillis: max, PublishRetries: publishRetries.Load(), ConsumeRetries: consumeRetries, AllowRedeliveries: *allowRedeliveries, ConsumerStartDelayMillis: float64(*consumerStartDelay) / float64(time.Millisecond), ConsumerDelayMillis: float64(*consumerDelay) / float64(time.Millisecond), PeakBacklogMessages: peakBacklog, BacklogAtPublishEnd: backlogAtPublishEnd, DrainSeconds: drainSeconds, DrainMessagesPerSecond: drainRate}
	integrityOK := value.Published == expected && value.Consumed == expected && missing == 0 && corrupt == 0 && (duplicates == 0 || *allowRedeliveries)
	if !integrityOK {
		return fmt.Errorf("integrity failed: generated=%d published=%d consumed=%d missing=%d duplicates=%d corrupt=%d", expected, value.Published, value.Consumed, missing, duplicates, corrupt)
	}
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(value)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func rateDeadline(start time.Time, id, messagesPerSecond int64) time.Time {
	if messagesPerSecond <= 0 {
		return time.Time{}
	}
	return start.Add(time.Duration(id) * time.Second / time.Duration(messagesPerSecond))
}

type messagePublisher interface {
	PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error)
}

func publishWithRetry(publisher messagePublisher, message *nats.Msg, timeout time.Duration) (int64, error) {
	deadline := time.Now().Add(timeout)
	var retries int64
	for {
		if _, err := publisher.PublishMsg(message); err == nil {
			return retries, nil
		} else if time.Now().After(deadline) {
			return retries, err
		}
		retries++
		time.Sleep(100 * time.Millisecond)
	}
}

func transientConsumeError(err error) bool {
	return errors.Is(err, nats.ErrNoResponders) || errors.Is(err, nats.ErrDisconnected) || errors.Is(err, nats.ErrConnectionClosed)
}

func makePayload(id int64, size int) []byte {
	payload := make([]byte, size)
	binary.BigEndian.PutUint64(payload[:8], uint64(id))
	for i := 8; i < size; i++ {
		payload[i] = byte((id + int64(i)) % 251)
	}
	return payload
}
func validatePayload(payload []byte, size int) (int64, bool) {
	if len(payload) != size {
		return 0, false
	}
	id := int64(binary.BigEndian.Uint64(payload[:8]))
	for i := 8; i < size; i++ {
		if payload[i] != byte((id+int64(i))%251) {
			return id, false
		}
	}
	return id, true
}
