package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyReleaseEvidence(t *testing.T) {
	dir := t.TempDir()
	reportValue := validReport()
	writeJSON(t, filepath.Join(dir, "candidate.json"), reportValue)
	writeJSON(t, filepath.Join(dir, "baseline.json"), reportValue)
	writeSamples(t, filepath.Join(dir, "resources.ndjson"), 24*time.Hour, time.Hour)
	proof := validEvidence(t, dir)
	writeJSON(t, filepath.Join(dir, "evidence.json"), proof)
	if err := verifyEvidence(filepath.Join(dir, "evidence.json"), true); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyReleaseEvidenceRejectsTamperingAndShortRuns(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*report, *evidence)
	}{
		{"short run", func(candidate *report, _ *evidence) {
			candidate.ConfiguredDurationSeconds = 3600
			candidate.DurationSeconds = 3600
		}},
		{"missing messages", func(candidate *report, _ *evidence) { candidate.Missing = 1 }},
		{"regression", func(candidate *report, _ *evidence) { candidate.PublishMessagesPerSecond = 1 }},
		{"wrong mode", func(_ *report, proof *evidence) { proof.Mode = "duration" }},
		{"desktop host", func(_ *report, proof *evidence) { proof.Host["operating_system"] = "Docker Desktop" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			candidate := validReport()
			baseline := validReport()
			writeJSON(t, filepath.Join(dir, "candidate.json"), candidate)
			writeJSON(t, filepath.Join(dir, "baseline.json"), baseline)
			writeSamples(t, filepath.Join(dir, "resources.ndjson"), 24*time.Hour, time.Hour)
			proof := validEvidence(t, dir)
			test.mutate(&candidate, &proof)
			writeJSON(t, filepath.Join(dir, "candidate.json"), candidate)
			proof.Report.SHA256 = digestFile(t, filepath.Join(dir, "candidate.json"))
			writeJSON(t, filepath.Join(dir, "evidence.json"), proof)
			if err := verifyEvidence(filepath.Join(dir, "evidence.json"), true); err == nil {
				t.Fatal("invalid release evidence was accepted")
			}
		})
	}
}

func TestVerifyEvidenceRejectsArtifactHashMismatch(t *testing.T) {
	dir := t.TempDir()
	value := validReport()
	writeJSON(t, filepath.Join(dir, "candidate.json"), value)
	writeJSON(t, filepath.Join(dir, "baseline.json"), value)
	writeSamples(t, filepath.Join(dir, "resources.ndjson"), 24*time.Hour, time.Hour)
	proof := validEvidence(t, dir)
	proof.Report.SHA256 = fmt.Sprintf("%064d", 0)
	writeJSON(t, filepath.Join(dir, "evidence.json"), proof)
	if err := verifyEvidence(filepath.Join(dir, "evidence.json"), true); err == nil {
		t.Fatal("tampered artifact was accepted")
	}
}

func TestCLIExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "required") {
		t.Fatalf("missing evidence exit=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"-unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("bad flag exit=%d", code)
	}
	stderr.Reset()
	if code := run([]string{"-evidence", filepath.Join(t.TempDir(), "missing.json")}, &stdout, &stderr); code != 1 || stderr.Len() == 0 {
		t.Fatalf("missing file exit=%d stderr=%q", code, stderr.String())
	}
}

func TestVerifyEvidenceRejectsIncompleteProvenanceAndSparseSamples(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*evidence)
	}{
		{"short revision", func(proof *evidence) { proof.SourceRevision = "abc" }},
		{"missing image", func(proof *evidence) { proof.NATSImageID = "" }},
		{"negative threshold", func(proof *evidence) { proof.MaxP99RegressionPercent = -1 }},
		{"path traversal", func(proof *evidence) { proof.Report.File = "../candidate.json" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			value := validReport()
			writeJSON(t, filepath.Join(dir, "candidate.json"), value)
			writeJSON(t, filepath.Join(dir, "baseline.json"), value)
			writeSamples(t, filepath.Join(dir, "resources.ndjson"), 24*time.Hour, time.Hour)
			proof := validEvidence(t, dir)
			test.mutate(&proof)
			writeJSON(t, filepath.Join(dir, "evidence.json"), proof)
			if err := verifyEvidence(filepath.Join(dir, "evidence.json"), true); err == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}

	dir := t.TempDir()
	value := validReport()
	writeJSON(t, filepath.Join(dir, "candidate.json"), value)
	writeJSON(t, filepath.Join(dir, "baseline.json"), value)
	writeSamples(t, filepath.Join(dir, "resources.ndjson"), 24*time.Hour, 4*time.Hour)
	proof := validEvidence(t, dir)
	proof.Resources.SHA256 = digestFile(t, filepath.Join(dir, "resources.ndjson"))
	writeJSON(t, filepath.Join(dir, "evidence.json"), proof)
	if err := verifyEvidence(filepath.Join(dir, "evidence.json"), true); err == nil {
		t.Fatal("sparse resource evidence was accepted")
	}
}

func validReport() report {
	return report{Schema: "rabbit-jetstream.io/performance-report/v1alpha1", Replicas: 3, PayloadBytes: 1024, Publishers: 4, Batch: 256, WorkloadMode: "duration", ConfiguredDurationSeconds: 86400, RequestedMessages: 1_000_000, Published: 1_000_000, Consumed: 1_000_000, DurationSeconds: 86400, PublishMessagesPerSecond: 1000, ConsumeMessagesPerSecond: 1000, PublishLatencyP99Millis: 5}
}

func validEvidence(t *testing.T, dir string) evidence {
	t.Helper()
	return evidence{Schema: evidenceSchema, Mode: "soak", GeneratedAt: time.Now().UTC(), SourceRevision: "0123456789abcdef0123456789abcdef01234567", NATSImageID: "sha256:image", Command: []string{"jetstream.ps1", "-Mode", "soak"}, SampleIntervalSeconds: 3600, MaxThroughputRegressionPercent: 20, MaxP99RegressionPercent: 30, Host: map[string]any{"operating_system": "Linux", "os_type": "linux", "kernel_version": "6.8.0"}, Report: artifact{File: "candidate.json", SHA256: digestFile(t, filepath.Join(dir, "candidate.json"))}, Resources: artifact{File: "resources.ndjson", SHA256: digestFile(t, filepath.Join(dir, "resources.ndjson"))}, Baseline: artifact{File: "baseline.json", SHA256: digestFile(t, filepath.Join(dir, "baseline.json"))}}
}

func writeSamples(t *testing.T, path string, duration, interval time.Duration) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	start := time.Now().UTC().Add(-duration)
	for elapsed := time.Duration(0); elapsed <= duration; elapsed += interval {
		for node := 1; node <= 3; node++ {
			value, _ := json.Marshal(resourceSample{CapturedAt: start.Add(elapsed), Node: fmt.Sprintf("nats-%d", node)})
			fmt.Fprintln(file, string(value))
		}
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func digestFile(t *testing.T, path string) string {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
