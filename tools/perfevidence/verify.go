package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	DeploymentMode                 string         `json:"deployment_mode,omitempty"`
	NATSBinarySHA256               string         `json:"nats_binary_sha256,omitempty"`
	VerificationRevision           string         `json:"verification_revision,omitempty"`
	NativePreflight                artifact       `json:"native_preflight,omitempty"`
	Command                        []string       `json:"command"`
	SampleIntervalSeconds          int            `json:"sample_interval_seconds"`
	MaxThroughputRegressionPercent float64        `json:"max_throughput_regression_percent"`
	MaxP99RegressionPercent        float64        `json:"max_p99_regression_percent"`
	BaselineMode                   string         `json:"baseline_mode,omitempty"`
	MinPublishMessagesPerSecond    float64        `json:"min_publish_messages_per_second,omitempty"`
	MinConsumeMessagesPerSecond    float64        `json:"min_consume_messages_per_second,omitempty"`
	MaxPublishLatencyP99Millis     float64        `json:"max_publish_latency_p99_millis,omitempty"`
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
	TargetPublishRate         float64   `json:"target_publish_messages_per_second"`
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
	PublishRetries            int64     `json:"publish_retries"`
	ConsumeRetries            int64     `json:"consume_retries"`
	AllowRedeliveries         bool      `json:"allow_redeliveries"`
	ConsumerStartDelayMillis  float64   `json:"consumer_start_delay_millis"`
	ConsumerDelayMillis       float64   `json:"consumer_delay_millis"`
	PeakBacklogMessages       int64     `json:"peak_backlog_messages"`
	BacklogAtPublishEnd       int64     `json:"backlog_at_publish_end"`
	DrainSeconds              float64   `json:"drain_seconds"`
	DrainMessagesPerSecond    float64   `json:"drain_messages_per_second"`
}

type resourceSample struct {
	CapturedAt time.Time `json:"captured_at"`
	Node       string    `json:"node"`
}

func verifyEvidence(path string, requireSoak bool, expectedRevision string) error {
	var proof evidence
	if err := decodeFile(path, &proof); err != nil {
		return fmt.Errorf("decode evidence: %w", err)
	}
	if proof.Schema != evidenceSchema {
		return fmt.Errorf("unsupported evidence schema %q", proof.Schema)
	}
	if proof.GeneratedAt.IsZero() || len(proof.SourceRevision) != 40 || strings.Trim(proof.SourceRevision, "0123456789abcdef") != "" || len(proof.Command) == 0 || len(proof.Host) == 0 {
		return fmt.Errorf("evidence provenance is incomplete")
	}
	if proof.DeploymentMode == "bare-metal" {
		if proof.NATSImageID != "" || !validSHA256(proof.NATSBinarySHA256) || len(proof.VerificationRevision) != 40 || strings.Trim(proof.VerificationRevision, "0123456789abcdef") != "" {
			return errors.New("bare-metal binary provenance is incomplete or mixed with image provenance")
		}
		preflightPath, err := verifyArtifact(filepath.Dir(path), proof.NativePreflight)
		if err != nil {
			return fmt.Errorf("bare-metal preflight: %w", err)
		}
		var preflight nativePreflight
		if err := decodePreflight(preflightPath, &preflight); err != nil {
			return err
		}
		if preflight.Schema != "rabbit-jetstream.io/native-linux-preflight/v1alpha1" || preflight.Runtime != "linux/amd64" || preflight.DeploymentMode != "bare-metal" || preflight.SourceRevision != proof.SourceRevision || preflight.NATSBinarySHA256 != proof.NATSBinarySHA256 || preflight.VerificationRevision != proof.VerificationRevision || !validSHA256(preflight.BundleManifestSHA256) {
			return errors.New("bare-metal preflight does not bind this workload")
		}
		boundHost, _ := json.Marshal(preflight.Host)
		claimedHost, _ := json.Marshal(proof.Host)
		if string(boundHost) != string(claimedHost) || preflight.Host["architecture"] != "amd64" || preflight.Host["os_type"] != "linux" {
			return errors.New("bare-metal host identity differs from preflight")
		}
	} else {
		if (proof.DeploymentMode != "" && proof.DeploymentMode != "container") || len(proof.NATSImageID) != 71 || !strings.HasPrefix(proof.NATSImageID, "sha256:") || !validSHA256(strings.TrimPrefix(proof.NATSImageID, "sha256:")) || proof.NATSBinarySHA256 != "" {
			return errors.New("container image provenance is incomplete")
		}
	}
	if requireSoak && (len(expectedRevision) != 40 || strings.Trim(expectedRevision, "0123456789abcdef") != "" || proof.SourceRevision != expectedRevision) {
		return fmt.Errorf("release evidence source revision does not match the expected revision")
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
	baselineMode := proof.BaselineMode
	if baselineMode == "" {
		baselineMode = "comparison"
	}
	if baselineMode != "comparison" && baselineMode != "inaugural" {
		return fmt.Errorf("unsupported baseline mode %q", proof.BaselineMode)
	}
	var baselinePath string
	if baselineMode == "comparison" {
		baselinePath, err = verifyArtifact(base, proof.Baseline)
		if err != nil {
			return fmt.Errorf("baseline artifact: %w", err)
		}
	} else if proof.Baseline.File != "" || proof.Baseline.SHA256 != "" {
		return fmt.Errorf("inaugural evidence must not reference a prior baseline")
	}
	var candidate report
	if err := decodeFile(reportPath, &candidate); err != nil {
		return fmt.Errorf("decode candidate report: %w", err)
	}
	if candidate.Schema != "rabbit-jetstream.io/performance-report/v1alpha1" || candidate.Replicas != 3 || candidate.RequestedMessages < 1 || candidate.Published != candidate.RequestedMessages || candidate.Consumed != candidate.RequestedMessages || candidate.Missing != 0 || candidate.Duplicates != 0 || candidate.Corrupt != 0 || candidate.PublishRetries != 0 || candidate.ConsumeRetries != 0 || candidate.AllowRedeliveries {
		return fmt.Errorf("candidate integrity or topology gate failed")
	}
	wallDuration := candidate.FinishedAt.Sub(candidate.StartedAt).Seconds()
	allowedClockDifference := candidate.DurationSeconds * 0.01
	if allowedClockDifference < 5 {
		allowedClockDifference = 5
	}
	if candidate.NATSVersion == "" || candidate.GOOS != "linux" || candidate.GOARCH == "" || candidate.CPUs < 1 || candidate.StartedAt.IsZero() || candidate.FinishedAt.IsZero() || wallDuration <= 0 || abs(wallDuration-candidate.DurationSeconds) > allowedClockDifference || proof.GeneratedAt.Before(candidate.FinishedAt) {
		return fmt.Errorf("candidate runtime provenance is invalid")
	}
	if requireSoak && (candidate.WorkloadMode != "duration" || candidate.ConfiguredDurationSeconds < 24*60*60 || candidate.DurationSeconds < 24*60*60) {
		return fmt.Errorf("candidate does not prove 24 hours of continuous operation")
	}
	if requireSoak && wallDuration < 24*60*60 {
		return fmt.Errorf("candidate timestamps do not prove 24 hours of continuous operation")
	}
	if baselineMode == "comparison" {
		var baseline report
		if err := decodeFile(baselinePath, &baseline); err != nil {
			return fmt.Errorf("decode baseline report: %w", err)
		}
		if candidate.Replicas != baseline.Replicas || candidate.PayloadBytes != baseline.PayloadBytes || candidate.Publishers != baseline.Publishers || candidate.Batch != baseline.Batch || candidate.WorkloadMode != baseline.WorkloadMode || candidate.ConfiguredDurationSeconds != baseline.ConfiguredDurationSeconds {
			return fmt.Errorf("baseline workload shape differs")
		}
		if baseline.Schema != "rabbit-jetstream.io/performance-report/v1alpha1" || baseline.RequestedMessages < 1 || baseline.Published != baseline.RequestedMessages || baseline.Consumed != baseline.RequestedMessages || baseline.PublishMessagesPerSecond <= 0 || baseline.ConsumeMessagesPerSecond <= 0 || baseline.PublishLatencyP99Millis <= 0 || baseline.Missing != 0 || baseline.Duplicates != 0 || baseline.Corrupt != 0 {
			return fmt.Errorf("baseline integrity or measurements are invalid")
		}
		if candidate.PublishMessagesPerSecond < baseline.PublishMessagesPerSecond*(1-proof.MaxThroughputRegressionPercent/100) || candidate.ConsumeMessagesPerSecond < baseline.ConsumeMessagesPerSecond*(1-proof.MaxThroughputRegressionPercent/100) || candidate.PublishLatencyP99Millis > baseline.PublishLatencyP99Millis*(1+proof.MaxP99RegressionPercent/100) {
			return fmt.Errorf("candidate performance regression exceeds evidence thresholds")
		}
	} else {
		if proof.MinPublishMessagesPerSecond <= 0 || proof.MinConsumeMessagesPerSecond <= 0 || proof.MaxPublishLatencyP99Millis <= 0 {
			return fmt.Errorf("inaugural evidence requires positive absolute performance thresholds")
		}
		if candidate.PublishMessagesPerSecond < proof.MinPublishMessagesPerSecond || candidate.ConsumeMessagesPerSecond < proof.MinConsumeMessagesPerSecond || candidate.PublishLatencyP99Millis > proof.MaxPublishLatencyP99Millis {
			return fmt.Errorf("candidate performance exceeds inaugural absolute thresholds")
		}
	}
	if err := verifyResourceSamplesForMode(resourcesPath, candidate.ConfiguredDurationSeconds, proof.SampleIntervalSeconds, candidate.StartedAt, candidate.FinishedAt, proof.DeploymentMode == "bare-metal"); err != nil {
		return err
	}
	return nil
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
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

func verifyResourceSamples(path string, duration float64, interval int, workloadStarted, workloadFinished time.Time) error {
	return verifyResourceSamplesForMode(path, duration, interval, workloadStarted, workloadFinished, false)
}

func validSHA256(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func verifyResourceSamplesForMode(path string, duration float64, interval int, workloadStarted, workloadFinished time.Time, bare bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var first, last time.Time
	nodes := map[string]int{}
	lastByNode := map[string]time.Time{}
	continuous := true
	identities := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var sample resourceSample
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil || sample.CapturedAt.IsZero() || sample.Node == "" {
			return fmt.Errorf("invalid resource sample")
		}
		if bare {
			var health struct {
				Healthy bool `json:"healthy"`
				Server  struct {
					ID    string `json:"server_id"`
					Start string `json:"start"`
				} `json:"server_statistics"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &health); err != nil || !health.Healthy || health.Server.ID == "" || health.Server.Start == "" {
				return errors.New("bare-metal node health or identity missing")
			}
			identity := health.Server.ID + "/" + health.Server.Start
			if previous := identities[sample.Node]; previous != "" && previous != identity {
				return errors.New("bare-metal node restarted during soak")
			}
			identities[sample.Node] = identity
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
	tolerance := time.Duration(2*interval) * time.Second
	if tolerance < 30*time.Second {
		tolerance = 30 * time.Second
	}
	windowAligned := !workloadStarted.IsZero() && !workloadFinished.IsZero() && !first.Before(workloadStarted.Add(-tolerance)) && !first.After(workloadStarted.Add(tolerance)) && !last.Before(workloadFinished.Add(-tolerance)) && !last.After(workloadFinished.Add(tolerance))
	if len(nodes) != 3 || !continuous || first.IsZero() || last.Sub(first).Seconds() < duration-float64(2*interval) || !windowAligned {
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
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON documents are not allowed")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}
