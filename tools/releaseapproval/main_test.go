package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testServerRevision = "1111111111111111111111111111111111111111"
	testSDKRevision    = "2222222222222222222222222222222222222222"
)

func TestVerifyApproval(t *testing.T) {
	path := createApprovalFixture(t, nil)
	if err := verifyApproval(path, testServerRevision); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyApprovalRejectsUnsafePromotion(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*approval)
		want   string
	}{
		{"wrong revision", func(value *approval) { value.ServerRevision = strings.Repeat("3", 40) }, "release identity"},
		{"missing architecture", func(value *approval) { value.NativePreflights = value.NativePreflights[:1] }, "amd64 and arm64"},
		{"bad stage order", func(value *approval) { value.CanaryStages[1].TrafficPercent = 11 }, "missing or out of order"},
		{"overlapping stage", func(value *approval) { value.CanaryStages[1].StartedAt = value.CanaryStages[0].StartedAt }, "overlap"},
		{"message loss", func(value *approval) { value.CanaryStages[2].MissingMessages = 1 }, "message integrity"},
		{"high errors", func(value *approval) { value.CanaryStages[2].Management5xxPercent = 5 }, "error, storage or backlog"},
		{"latency regression", func(value *approval) { value.CanaryStages[2].PublishP99Millis = 131 }, "publish P99"},
		{"throughput regression", func(value *approval) { value.CanaryStages[2].ThroughputPerSecond = 79 }, "throughput"},
		{"duplicate budget", func(value *approval) { value.CanaryStages[2].DuplicateRatePercent = 0.2 }, "duplicate-rate"},
		{"node failure", func(value *approval) { value.NodeFailure.ReplicasConverged = false }, "node-failure"},
		{"rollback", func(value *approval) { value.Rollback.ServiceRestored = false }, "rollback"},
		{"partial approval", func(value *approval) { value.PartialDependencies[0].Approved = false }, "not approved"},
		{"missing priority approval", func(value *approval) { value.PartialDependencies[0].ID = "ordering" }, "priority-queue"},
		{"unknown partial", func(value *approval) { value.PartialDependencies[0].ID = "typo" }, "not approved"},
		{"AMQP limitation", func(value *approval) { value.KnownLimitations = []string{"at-least-once-delivery"} }, "no-amqp-wire-compatibility"},
		{"signoff", func(value *approval) { value.Signoffs[0].Approved = false }, "invalid release sign-off"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := createApprovalFixture(t, test.mutate)
			if err := verifyApproval(path, testServerRevision); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verifyApproval() = %v, want %q", err, test.want)
			}
		})
	}
}

func TestVerifyApprovalRejectsTamperedArtifact(t *testing.T) {
	path := createApprovalFixture(t, nil)
	var proof approval
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), proof.CanaryStages[0].Evidence.File), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyApproval(path, testServerRevision); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered artifact was accepted: %v", err)
	}
}

func TestRun(t *testing.T) {
	path := createApprovalFixture(t, nil)
	var stdout, stderr strings.Builder
	if code := run([]string{"-evidence", path, "-source-revision", testServerRevision}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "verified") {
		t.Fatalf("run() = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("run() without arguments = %d", code)
	}
	if code := run([]string{"-evidence", path, "-source-revision", strings.Repeat("3", 40)}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() with wrong revision = %d", code)
	}
}

func createApprovalFixture(t *testing.T, mutate func(*approval)) string {
	t.Helper()
	directory := t.TempDir()
	write := func(name string, value any) artifact {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		return artifact{File: name, SHA256: fmt.Sprintf("%x", sum)}
	}
	local := referencedEvidence{Schema: "rabbit-jetstream.io/local-rc/v1alpha1", Mode: "release"}
	local.Server.Revision = testServerRevision
	local.SDK.Version, local.SDK.Revision = "0.1.0-rc.1", testSDKRevision
	proof := approval{
		Schema: approvalSchema, ReleaseVersion: "v0.1.0-rc.1", ServerRevision: testServerRevision,
		SDKVersion: "0.1.0-rc.1", SDKRevision: testSDKRevision,
		LocalReleaseEvidence: write("evidence/local-rc.json", local),
		KnownLimitations:     []string{"no-amqp-wire-compatibility", "at-least-once-delivery"},
		PartialDependencies:  []compatibilityApproval{{ID: "priority-queue", Approved: true, Owner: "application", Rationale: "native SDK contract accepted"}},
	}
	for _, runtime := range []string{"linux/amd64", "linux/arm64"} {
		preflightProof := referencedEvidence{Schema: "rabbit-jetstream.io/native-linux-preflight/v1alpha1", SourceRevision: testServerRevision, SDKVersion: "0.1.0-rc.1", SDKRevision: testSDKRevision, Runtime: runtime}
		proof.NativePreflights = append(proof.NativePreflights, preflight{Runtime: runtime, Evidence: write("evidence/preflight-"+strings.TrimPrefix(runtime, "linux/")+".json", preflightProof)})
	}
	soakProof := referencedEvidence{Schema: "rabbit-jetstream.io/performance-evidence/v1alpha1", SourceRevision: testServerRevision, Mode: "soak"}
	proof.SoakEvidence = write("evidence/soak.json", soakProof)
	for index, percent := range []int{1, 10, 25, 50, 100} {
		item := validStage(percent)
		item.StartedAt = item.StartedAt.Add(time.Duration(index) * time.Hour)
		item.EndedAt = item.EndedAt.Add(time.Duration(index) * time.Hour)
		item.Evidence = write(fmt.Sprintf("evidence/canary-%03d.json", percent), map[string]any{"traffic_percent": percent, "source": "observability-export"})
		proof.CanaryStages = append(proof.CanaryStages, item)
	}
	proof.NodeFailure = nodeFailure{TrafficPercent: 10, Passed: true, PubAckContinued: true, ConsumeContinued: true, ReplicasConverged: true, RecoverySeconds: 30, RecoverySLOSeconds: 60, Evidence: write("evidence/node-failure.json", map[string]bool{"passed": true})}
	proof.Rollback = rollback{Passed: true, MessagesReconciled: true, ServiceRestored: true, DurationSeconds: 90, SLOSeconds: 120, Evidence: write("evidence/rollback.json", map[string]bool{"passed": true})}
	approvedAt := time.Date(2026, 8, 3, 7, 0, 0, 0, time.UTC)
	proof.Signoffs = []signoff{{Role: "service_owner", Name: "service", Approved: true, ApprovedAt: approvedAt}, {Role: "application_owner", Name: "application", Approved: true, ApprovedAt: approvedAt}, {Role: "on_call_operator", Name: "operator", Approved: true, ApprovedAt: approvedAt}}
	if mutate != nil {
		mutate(&proof)
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "release-approval.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validStage(percent int) stage {
	start := time.Date(2026, 8, 3, 1, 0, 0, 0, time.UTC)
	return stage{
		TrafficPercent: percent, StartedAt: start, EndedAt: start.Add(time.Hour), JetStreamAvailable: true,
		ControllerActive: true, ExpectedNodes: 3, CurrentNodes: 3, BacklogMessages: 10, BacklogLimitMessages: 100,
		Management5xxPercent: 0.1, StoragePercent: 50, PublishP99Millis: 100, PublishP99SLOMillis: 150,
		BaselinePublishP99Millis: 100, ThroughputPerSecond: 100, BaselineThroughput: 100,
		DuplicateRatePercent: 0.01, DuplicateBudgetPercent: 0.1,
	}
}

func TestHelpers(t *testing.T) {
	if !validRevision(testServerRevision) || validRevision("ABC") {
		t.Fatal("revision validation failed")
	}
	for _, path := range []string{"", "../escape", `bad\\path`} {
		if safeRelativePath(path) {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
	if !safeRelativePath("evidence/file.json") {
		t.Fatal("safe path rejected")
	}
	if !equalBytes([]byte{1, 2}, []byte{1, 2}) || equalBytes([]byte{1}, []byte{2}) || equalBytes([]byte{1}, []byte{1, 2}) {
		t.Fatal("constant-time byte comparison failed")
	}
}

func TestKnownPartialIDsMatchCompatibilityContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "api", "client-compatibility.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Features []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, feature := range contract.Features {
		if feature.Status == "partial" {
			count++
			if !knownPartialIDs[feature.ID] {
				t.Errorf("partial compatibility ID %q is missing from release approval", feature.ID)
			}
		}
	}
	if count != len(knownPartialIDs) {
		t.Errorf("release approval has %d partial IDs, contract has %d", len(knownPartialIDs), count)
	}
}
