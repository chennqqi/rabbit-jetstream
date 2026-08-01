package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const evidenceSchema = "rabbit-jetstream.io/performance-evidence/v1alpha1"

type artifact struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type evidence struct {
	Schema                         string         `json:"schema"`
	Mode                           string         `json:"mode"`
	GeneratedAt                    time.Time      `json:"generated_at"`
	SourceRevision                 string         `json:"source_revision"`
	NATSImageID                    string         `json:"nats_image_id"`
	Command                        []string       `json:"command"`
	SampleIntervalSeconds          int            `json:"sample_interval_seconds"`
	MaxThroughputRegressionPercent float64        `json:"max_throughput_regression_percent"`
	MaxP99RegressionPercent        float64        `json:"max_p99_regression_percent"`
	Host                           map[string]any `json:"host"`
	Report                         artifact       `json:"report"`
	Resources                      artifact       `json:"resources"`
	Baseline                       artifact       `json:"baseline"`
}

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
	WorkloadMode              string    `json:"workload_mode"`
	ConfiguredDurationSeconds float64   `json:"configured_duration_seconds"`
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
	PublishLatencyP99Millis   float64   `json:"publish_latency_p99_millis"`
	PublishLatencyP50Millis   float64   `json:"publish_latency_p50_millis"`
	PublishLatencyP95Millis   float64   `json:"publish_latency_p95_millis"`
	PublishLatencyMaxMillis   float64   `json:"publish_latency_max_millis"`
}

type resourceSample struct {
	CapturedAt time.Time `json:"captured_at"`
	Node       string    `json:"node"`
}

func verifyEvidence(path string, requireSoak bool) error {
	var proof evidence
	if err := decodeFile(path, &proof); err != nil {
		return fmt.Errorf("decode evidence: %w", err)
	}
	if proof.Schema != evidenceSchema {
		return fmt.Errorf("unsupported evidence schema %q", proof.Schema)
	}
	if proof.GeneratedAt.IsZero() || len(proof.SourceRevision) != 40 || strings.Trim(proof.SourceRevision, "0123456789abcdef") != "" || !strings.HasPrefix(proof.NATSImageID, "sha256:") || len(proof.Command) == 0 || len(proof.Host) == 0 {
		return fmt.Errorf("evidence provenance is incomplete")
	}
	if proof.SampleIntervalSeconds < 1 || proof.MaxThroughputRegressionPercent < 0 || proof.MaxP99RegressionPercent < 0 {
		return fmt.Errorf("evidence thresholds are invalid")
	}
	if requireSoak && proof.Mode != "soak" {
		return fmt.Errorf("release evidence mode is %q, expected soak", proof.Mode)
	}
	if requireSoak {
		host := strings.ToLower(fmt.Sprint(proof.Host["operating_system"], " ", proof.Host["kernel_version"]))
		if proof.Host["os_type"] != "linux" || strings.Contains(host, "docker desktop") || strings.Contains(host, "wsl") {
			return fmt.Errorf("release soak evidence must come from a native Linux production-like host")
		}
	}
	base := filepath.Dir(path)
	reportPath, err := verifyArtifact(base, proof.Report)
	if err != nil {
		return fmt.Errorf("report artifact: %w", err)
	}
	resourcesPath, err := verifyArtifact(base, proof.Resources)
	if err != nil {
		return fmt.Errorf("resource artifact: %w", err)
	}
	baselinePath, err := verifyArtifact(base, proof.Baseline)
	if err != nil {
		return fmt.Errorf("baseline artifact: %w", err)
	}
	var candidate, baseline report
	if err := decodeFile(reportPath, &candidate); err != nil {
		return fmt.Errorf("decode candidate report: %w", err)
	}
	if err := decodeFile(baselinePath, &baseline); err != nil {
		return fmt.Errorf("decode baseline report: %w", err)
	}
	if candidate.Schema != "rabbit-jetstream.io/performance-report/v1alpha1" || candidate.Replicas != 3 || candidate.Published != candidate.RequestedMessages || candidate.Consumed != candidate.RequestedMessages || candidate.Missing != 0 || candidate.Duplicates != 0 || candidate.Corrupt != 0 {
		return fmt.Errorf("candidate integrity or topology gate failed")
	}
	if requireSoak && (candidate.WorkloadMode != "duration" || candidate.ConfiguredDurationSeconds < 24*60*60 || candidate.DurationSeconds < 24*60*60) {
		return fmt.Errorf("candidate does not prove 24 hours of continuous operation")
	}
	if candidate.Replicas != baseline.Replicas || candidate.PayloadBytes != baseline.PayloadBytes || candidate.Publishers != baseline.Publishers || candidate.Batch != baseline.Batch || candidate.WorkloadMode != baseline.WorkloadMode || candidate.ConfiguredDurationSeconds != baseline.ConfiguredDurationSeconds {
		return fmt.Errorf("baseline workload shape differs")
	}
	if candidate.PublishMessagesPerSecond < baseline.PublishMessagesPerSecond*(1-proof.MaxThroughputRegressionPercent/100) || candidate.ConsumeMessagesPerSecond < baseline.ConsumeMessagesPerSecond*(1-proof.MaxThroughputRegressionPercent/100) || candidate.PublishLatencyP99Millis > baseline.PublishLatencyP99Millis*(1+proof.MaxP99RegressionPercent/100) {
		return fmt.Errorf("candidate performance regression exceeds evidence thresholds")
	}
	if err := verifyResourceSamples(resourcesPath, candidate.ConfiguredDurationSeconds, proof.SampleIntervalSeconds); err != nil {
		return err
	}
	return nil
}

func verifyArtifact(base string, item artifact) (string, error) {
	if item.File == "" || filepath.Base(item.File) != item.File || len(item.SHA256) != 64 {
		return "", fmt.Errorf("invalid artifact reference")
	}
	path := filepath.Join(base, item.File)
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	if hex.EncodeToString(digest.Sum(nil)) != item.SHA256 {
		return "", fmt.Errorf("SHA-256 mismatch for %s", item.File)
	}
	return path, nil
}

func verifyResourceSamples(path string, duration float64, interval int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var first, last time.Time
	nodes := map[string]int{}
	lastByNode := map[string]time.Time{}
	continuous := true
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var sample resourceSample
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil || sample.CapturedAt.IsZero() || sample.Node == "" {
			return fmt.Errorf("invalid resource sample")
		}
		if first.IsZero() || sample.CapturedAt.Before(first) {
			first = sample.CapturedAt
		}
		if last.IsZero() || sample.CapturedAt.After(last) {
			last = sample.CapturedAt
		}
		if previous := lastByNode[sample.Node]; !previous.IsZero() {
			gap := sample.CapturedAt.Sub(previous)
			if gap <= 0 || gap > time.Duration(3*interval)*time.Second {
				continuous = false
			}
		}
		lastByNode[sample.Node] = sample.CapturedAt
		nodes[sample.Node]++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	minimumSamples := int(duration/float64(interval)) - 1
	continuous = continuous && minimumSamples > 0
	for _, count := range nodes {
		continuous = continuous && count >= minimumSamples
	}
	if len(nodes) != 3 || !continuous || first.IsZero() || last.Sub(first).Seconds() < duration-float64(2*interval) {
		return fmt.Errorf("resource samples do not span the workload across three nodes")
	}
	return nil
}

func decodeFile(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
