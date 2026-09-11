package main

import (
	"errors"
	"testing"
	"time"
)

func TestSamplingDoesNotAccumulateCollectionTime(t *testing.T) {
	start := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	current := start
	count := 0
	err := runSampling(24*time.Hour+20*time.Second, 10*time.Second, func() time.Time { return current }, func(d time.Duration) {
		if d < 0 {
			t.Fatal("negative wait")
		}
		current = current.Add(d)
	}, func(at time.Time) error {
		expected := start.Add(time.Duration(count) * 10 * time.Second)
		if !at.Equal(expected) {
			t.Fatalf("sample %d drifted: %v, expected %v", count, at, expected)
		}
		count++
		current = current.Add(17672 * time.Microsecond)
		return nil
	})
	if err != nil || count != 8643 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestSamplingSkipsOverrunsWithoutBackfill(t *testing.T) {
	start := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	current := start
	var stamps []time.Duration
	err := runSampling(time.Minute, 10*time.Second, func() time.Time { return current }, func(d time.Duration) { current = current.Add(d) }, func(at time.Time) error {
		stamps = append(stamps, at.Sub(start))
		current = current.Add(25 * time.Second)
		return nil
	})
	if err != nil || len(stamps) != 3 || stamps[0] != 0 || stamps[1] != 30*time.Second || stamps[2] != time.Minute {
		t.Fatalf("stamps=%v err=%v", stamps, err)
	}
}

func TestSamplingStopsOnCollectionFailure(t *testing.T) {
	want := errors.New("write failed")
	err := runSampling(time.Minute, time.Second, time.Now, func(time.Duration) { t.Fatal("slept after failure") }, func(time.Time) error { return want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestParseNodes(t *testing.T) {
	nodes, err := parseNodes("nats-1=http://one,nats-2=http://two/,nats-3=http://three")
	if err != nil || len(nodes) != 3 || nodes["nats-2"] != "http://two" {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	for _, invalid := range []string{"", "a=x", "a=x,a=y,b=z", "a=x,b=y,c="} {
		if _, err := parseNodes(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}

func TestFreeBytes(t *testing.T) {
	if value, err := freeBytes(t.TempDir()); err != nil || value == 0 {
		t.Fatalf("freeBytes=%d err=%v", value, err)
	}
}
