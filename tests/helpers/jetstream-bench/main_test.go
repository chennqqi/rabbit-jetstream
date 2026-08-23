package main

import (
	"testing"
	"time"
)

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
