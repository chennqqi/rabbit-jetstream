package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSamplingScheduleKeepsExistingAcceptanceThresholds(t *testing.T) {
	start := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name           string
		interval, span time.Duration
		accepted       bool
	}{
		{"fixed cadence", 10 * time.Second, 24*time.Hour + 20*time.Second, true},
		{"observed collection drift", 10*time.Second + 17672*time.Microsecond, 24*time.Hour + 30*time.Second, false},
		{"missing slots", 20 * time.Second, 24*time.Hour + 20*time.Second, false},
		{"long gaps", 40 * time.Second, 24*time.Hour + 20*time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "resources.ndjson")
			writeSamplesFrom(t, path, start, tc.span, tc.interval)
			err := verifyResourceSamples(path, 86400, 10, start, start.Add(24*time.Hour))
			if tc.accepted && err != nil {
				t.Fatal(err)
			}
			if !tc.accepted && (err == nil || !strings.Contains(err.Error(), "minimum_per_node=8639")) {
				t.Fatalf("expected unchanged minimum with diagnostics, got %v", err)
			}
		})
	}
}
