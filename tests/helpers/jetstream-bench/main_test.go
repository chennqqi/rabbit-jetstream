package main

import (
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type retryPublisher struct {
	failures int
	calls    int
}

func (p *retryPublisher) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	p.calls++
	if p.calls <= p.failures {
		return nil, errors.New("temporary")
	}
	return &nats.PubAck{}, nil
}

func TestPayloadIntegrity(t *testing.T) {
	for _, id := range []int64{0, 1, 999999} {
		payload := makePayload(id, 128)
		actual, ok := validatePayload(payload, 128)
		if !ok || actual != id {
			t.Fatalf("id=%d actual=%d ok=%t", id, actual, ok)
		}
		payload[64]++
		if _, ok := validatePayload(payload, 128); ok {
			t.Fatal("corruption accepted")
		}
	}
	if _, ok := validatePayload([]byte("short"), 128); ok {
		t.Fatal("short payload accepted")
	}
}

func TestLatencySummary(t *testing.T) {
	samples := &latencySamples{}
	if p50, p95, p99, max := samples.summary(); p50+p95+p99+max != 0 {
		t.Fatal("empty samples nonzero")
	}
	for i := 1; i <= 100; i++ {
		samples.add(time.Duration(i) * time.Millisecond)
	}
	p50, p95, p99, max := samples.summary()
	if p50 != 50 || p95 != 95 || p99 != 99 || max != 100 {
		t.Fatalf("p50=%v p95=%v p99=%v max=%v", p50, p95, p99, max)
	}
}

func TestRateDeadline(t *testing.T) {
	start := time.Unix(100, 0)
	if target := rateDeadline(start, 5000, 5000); !target.Equal(start.Add(time.Second)) {
		t.Fatalf("target = %v", target)
	}
	if target := rateDeadline(start, 1, 0); !target.IsZero() {
		t.Fatalf("unlimited target = %v", target)
	}
}

func TestPublishWithRetry(t *testing.T) {
	publisher := &retryPublisher{failures: 2}
	retries, err := publishWithRetry(publisher, nats.NewMsg("test"), time.Second)
	if err != nil || retries != 2 || publisher.calls != 3 {
		t.Fatalf("retries=%d calls=%d err=%v", retries, publisher.calls, err)
	}
	publisher = &retryPublisher{failures: 100}
	if _, err := publishWithRetry(publisher, nats.NewMsg("test"), time.Millisecond); err == nil {
		t.Fatal("retry deadline was ignored")
	}
}

func TestTransientConsumeError(t *testing.T) {
	for _, err := range []error{nats.ErrNoResponders, nats.ErrDisconnected, nats.ErrConnectionClosed} {
		if !transientConsumeError(err) {
			t.Fatalf("transient error rejected: %v", err)
		}
	}
	if transientConsumeError(errors.New("permission denied")) {
		t.Fatal("permanent error accepted")
	}
}
