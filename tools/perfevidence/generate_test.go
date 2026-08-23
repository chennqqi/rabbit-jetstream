package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateInauguralEvidence(t *testing.T) {
	dir := t.TempDir()
	reportValue := validReport()
	reportPath := filepath.Join(dir, "candidate.json")
	resourcesPath := filepath.Join(dir, "resources.ndjson")
	preflightPath := filepath.Join(dir, "preflight.json")
	evidencePath := filepath.Join(dir, "evidence.json")
	writeJSON(t, reportPath, reportValue)
	writeSamplesFrom(t, resourcesPath, reportValue.StartedAt, 24*time.Hour, time.Hour)
	writePreflight(t, preflightPath)
	revision, err := createInauguralEvidence(generateOptions{
		EvidencePath: evidencePath, ReportPath: reportPath, ResourcesPath: resourcesPath, PreflightPath: preflightPath,
		NATSImageID: strings.Repeat("a", 64), CommandJSON: `["jetstream-bench","--duration","24h"]`,
		SampleInterval: 3600, MinPublish: 900, MinConsume: 900, MaxP99: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if revision != testRevision {
		t.Fatalf("revision = %q", revision)
	}
	if err := verifyEvidence(evidencePath, true, testRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := createInauguralEvidence(generateOptions{EvidencePath: evidencePath}); err == nil {
		t.Fatal("existing evidence or incomplete arguments were accepted")
	}
}

func TestCreateInauguralEvidenceRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "candidate.json"), validReport())
	if _, err := createInauguralEvidence(generateOptions{EvidencePath: filepath.Join(dir, "proof.json")}); err == nil {
		t.Fatal("incomplete inputs accepted")
	}
}

func TestCLICreatesAndVerifiesInauguralEvidence(t *testing.T) {
	dir := t.TempDir()
	value := validReport()
	reportPath := filepath.Join(dir, "candidate.json")
	resourcesPath := filepath.Join(dir, "resources.ndjson")
	preflightPath := filepath.Join(dir, "preflight.json")
	evidencePath := filepath.Join(dir, "evidence.json")
	writeJSON(t, reportPath, value)
	writeSamplesFrom(t, resourcesPath, value.StartedAt, 24*time.Hour, time.Hour)
	writePreflight(t, preflightPath)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-create-inaugural", "-evidence", evidencePath, "-report", reportPath, "-resources", resourcesPath,
		"-native-preflight", preflightPath, "-nats-image-id", strings.Repeat("a", 64),
		"-command-json", `["jetstream-bench","--duration","24h"]`, "-sample-interval", "3600",
		"-min-publish", "900", "-min-consume", "900", "-max-p99", "10",
	}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "created and verified") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func writePreflight(t *testing.T, path string) {
	t.Helper()
	value := nativePreflight{Schema: "rabbit-jetstream.io/native-linux-preflight/v1alpha1", SourceRevision: testRevision, Runtime: "linux/amd64"}
	value.Docker.OperatingSystem = "Rocky Linux"
	value.Docker.OSType = "linux"
	value.Docker.Architecture = "amd64"
	value.Docker.KernelVersion = "6.12.0"
	value.Docker.Driver = "overlay"
	value.Docker.NCPU = 32
	value.Docker.MemTotal = 128 << 30
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}
