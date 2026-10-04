package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const approvalSchema = "rabbit-jetstream.io/release-approval/v1alpha1"

var knownPartialIDs = map[string]bool{"fanout-routing": true, "publisher-confirm": true, "redelivery": true, "prefetch-backpressure": true, "priority-queue": true, "queue-ttl": true, "length-limit": true, "dead-letter": true, "ordering": true, "quorum-queue": true}

type artifact struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type preflight struct {
	Runtime  string   `json:"runtime"`
	Evidence artifact `json:"evidence"`
}

type stage struct {
	TrafficPercent           int       `json:"traffic_percent"`
	StartedAt                time.Time `json:"started_at"`
	EndedAt                  time.Time `json:"ended_at"`
	JetStreamAvailable       bool      `json:"jetstream_available"`
	ControllerActive         bool      `json:"controller_active"`
	ExpectedNodes            int       `json:"expected_nodes"`
	CurrentNodes             int       `json:"current_nodes"`
	MissingMessages          int64     `json:"missing_messages"`
	CorruptMessages          int64     `json:"corrupt_messages"`
	DLQTransferFailures      int64     `json:"dlq_transfer_failures"`
	Management5xxPercent     float64   `json:"management_5xx_percent"`
	StoragePercent           float64   `json:"storage_percent"`
	BacklogMessages          int64     `json:"backlog_messages"`
	BacklogLimitMessages     int64     `json:"backlog_limit_messages"`
	PublishP99Millis         float64   `json:"publish_p99_millis"`
	PublishP99SLOMillis      float64   `json:"publish_p99_slo_millis"`
	BaselinePublishP99Millis float64   `json:"baseline_publish_p99_millis"`
	ThroughputPerSecond      float64   `json:"throughput_per_second"`
	BaselineThroughput       float64   `json:"baseline_throughput_per_second"`
	DuplicateRatePercent     float64   `json:"duplicate_rate_percent"`
	DuplicateBudgetPercent   float64   `json:"duplicate_budget_percent"`
	Evidence                 artifact  `json:"evidence"`
}

type nodeFailure struct {
	TrafficPercent     int      `json:"traffic_percent"`
	Passed             bool     `json:"passed"`
	PubAckContinued    bool     `json:"puback_continued"`
	ConsumeContinued   bool     `json:"consume_continued"`
	ReplicasConverged  bool     `json:"replicas_converged"`
	RecoverySeconds    float64  `json:"recovery_seconds"`
	RecoverySLOSeconds float64  `json:"recovery_slo_seconds"`
	Evidence           artifact `json:"evidence"`
}

type rollback struct {
	Passed             bool     `json:"passed"`
	MessagesReconciled bool     `json:"messages_reconciled"`
	ServiceRestored    bool     `json:"service_restored"`
	DurationSeconds    float64  `json:"duration_seconds"`
	SLOSeconds         float64  `json:"slo_seconds"`
	Evidence           artifact `json:"evidence"`
}

type compatibilityApproval struct {
	ID        string `json:"id"`
	Approved  bool   `json:"approved"`
	Owner     string `json:"owner"`
	Rationale string `json:"rationale"`
}

type signoff struct {
	Role       string    `json:"role"`
	Name       string    `json:"name"`
	Approved   bool      `json:"approved"`
	ApprovedAt time.Time `json:"approved_at"`
}

type approval struct {
	Schema                          string                  `json:"schema"`
	ReleaseVersion                  string                  `json:"release_version"`
	ServerRevision                  string                  `json:"server_revision"`
	SDKVersion                      string                  `json:"sdk_version"`
	SDKRevision                     string                  `json:"sdk_revision"`
	LocalReleaseEvidence            artifact                `json:"local_release_evidence"`
	NativePreflights                []preflight             `json:"native_preflights"`
	SoakEvidence                    artifact                `json:"soak_evidence"`
	CanaryStages                    []stage                 `json:"canary_stages"`
	NodeFailure                     nodeFailure             `json:"node_failure"`
	Rollback                        rollback                `json:"rollback"`
	ClusterQualificationEvidence    artifact                `json:"cluster_qualification_evidence,omitempty"`
	PartialDependencies             []compatibilityApproval `json:"partial_dependencies"`
	KnownLimitations                []string                `json:"known_limitations"`
	Signoffs                        []signoff               `json:"signoffs"`
}

type referencedEvidence struct {
	Schema         string `json:"schema"`
	SourceRevision string `json:"source_revision"`
	SDKVersion     string `json:"sdk_version"`
	SDKRevision    string `json:"sdk_revision"`
	Server         struct {
		Revision string `json:"revision"`
		Dirty    bool   `json:"dirty"`
	} `json:"server"`
	SDK struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
		Dirty    bool   `json:"dirty"`
	} `json:"sdk"`
	Runtime string `json:"runtime"`
	Mode    string `json:"mode"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("releaseapproval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	evidencePath := flags.String("evidence", "", "release approval evidence JSON")
	expectedRevision := flags.String("source-revision", "", "expected 40-character server revision")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *evidencePath == "" || !validRevision(*expectedRevision) {
		fmt.Fprintln(stderr, "-evidence and a 40-character -source-revision are required")
		return 2
	}
	if err := verifyApproval(*evidencePath, *expectedRevision); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "release approval evidence verified")
	return 0
}

func verifyApproval(path, expectedRevision string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var proof approval
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proof); err != nil {
		return fmt.Errorf("decode approval evidence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("approval evidence must contain exactly one JSON document")
	}
	if proof.Schema != approvalSchema || proof.ServerRevision != expectedRevision || !validRevision(proof.ServerRevision) || !validRevision(proof.SDKRevision) || strings.TrimPrefix(proof.ReleaseVersion, "v") != proof.SDKVersion {
		return errors.New("release identity is incomplete or mismatched")
	}
	base := filepath.Dir(path)
	seenArtifacts := make(map[string]bool)
	verify := func(item artifact) ([]byte, error) {
		if !safeRelativePath(item.File) || seenArtifacts[item.File] || len(item.SHA256) != 64 {
			return nil, fmt.Errorf("invalid or duplicate evidence artifact %q", item.File)
		}
		seenArtifacts[item.File] = true
		expected, err := hex.DecodeString(item.SHA256)
		if err != nil {
			return nil, fmt.Errorf("invalid artifact checksum %q", item.File)
		}
		content, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(item.File)))
		if err != nil {
			return nil, err
		}
		actual := sha256.Sum256(content)
		if !equalBytes(expected, actual[:]) {
			return nil, fmt.Errorf("artifact checksum mismatch: %s", item.File)
		}
		return content, nil
	}
	local, err := verify(proof.LocalReleaseEvidence)
	if err != nil {
		return err
	}
	var localProof referencedEvidence
	if err := json.Unmarshal(local, &localProof); err != nil || localProof.Schema != "rabbit-jetstream.io/local-rc/v1alpha1" || (localProof.Mode != "release" && localProof.Mode != "quick") || localProof.Server.Revision != proof.ServerRevision || localProof.Server.Dirty || localProof.SDK.Version != proof.SDKVersion || localProof.SDK.Revision != proof.SDKRevision || localProof.SDK.Dirty {
		return errors.New("local Release evidence does not bind the approved candidate")
	}
	if len(proof.NativePreflights) != 1 {
		return errors.New("native preflight evidence is required for linux/amd64")
	}
	runtimes := make(map[string]bool)
	for _, item := range proof.NativePreflights {
		content, err := verify(item.Evidence)
		if err != nil {
			return err
		}
		var preflightProof referencedEvidence
		if err := json.Unmarshal(content, &preflightProof); err != nil || preflightProof.Schema != "rabbit-jetstream.io/native-linux-preflight/v1alpha1" || preflightProof.SourceRevision != proof.ServerRevision || preflightProof.SDKVersion != proof.SDKVersion || preflightProof.SDKRevision != proof.SDKRevision || preflightProof.Runtime != item.Runtime {
			return fmt.Errorf("invalid native preflight evidence for %s", item.Runtime)
		}
		runtimes[item.Runtime] = true
	}
	if !runtimes["linux/amd64"] {
		return errors.New("native preflight must cover linux/amd64")
	}
	soak, err := verify(proof.SoakEvidence)
	if err != nil {
		return err
	}
	var soakProof referencedEvidence
	if err := json.Unmarshal(soak, &soakProof); err != nil || soakProof.Schema != "rabbit-jetstream.io/performance-evidence/v1alpha1" || soakProof.Mode != "soak" || soakProof.SourceRevision != proof.ServerRevision {
		return errors.New("soak evidence does not bind the approved candidate")
	}
	expectedStages := []int{1, 10, 25, 50, 100}
	if len(proof.CanaryStages) != len(expectedStages) {
		return errors.New("canary must contain 1, 10, 25, 50 and 100 percent stages")
	}
	var previousStageEnd time.Time
	for index, item := range proof.CanaryStages {
		if item.TrafficPercent != expectedStages[index] {
			return errors.New("canary stages are missing or out of order")
		}
		if !previousStageEnd.IsZero() && item.StartedAt.Before(previousStageEnd) {
			return errors.New("canary observation windows overlap or are out of order")
		}
		if err := verifyStage(item); err != nil {
			return fmt.Errorf("canary %d%%: %w", item.TrafficPercent, err)
		}
		if _, err := verify(item.Evidence); err != nil {
			return err
		}
		previousStageEnd = item.EndedAt
	}
	if err := verifyNodeFailure(proof.NodeFailure); err != nil {
		return err
	}
	if _, err := verify(proof.NodeFailure.Evidence); err != nil {
		return err
	}
	if err := verifyRollback(proof.Rollback); err != nil {
		return err
	}
	if _, err := verify(proof.Rollback.Evidence); err != nil {
		return err
	}
	if len(proof.PartialDependencies) == 0 {
		return errors.New("partial compatibility dependency review is missing")
	}
	partialIDs := make(map[string]bool)
	for _, item := range proof.PartialDependencies {
		if !knownPartialIDs[item.ID] || partialIDs[item.ID] || !item.Approved || item.Owner == "" || item.Rationale == "" {
			return fmt.Errorf("partial compatibility dependency %q is not approved", item.ID)
		}
		partialIDs[item.ID] = true
	}
	if !partialIDs["priority-queue"] {
		return errors.New("priority-queue partial compatibility approval is required")
	}
	limitations := make(map[string]bool)
	for _, item := range proof.KnownLimitations {
		limitations[item] = true
	}
	for _, required := range []string{"no-amqp-wire-compatibility", "at-least-once-delivery"} {
		if !limitations[required] {
			return fmt.Errorf("required known limitation is missing: %s", required)
		}
	}
	requiredRoles := map[string]bool{"service_owner": false, "application_owner": false, "on_call_operator": false}
	// A sole owner explicitly accepts all three operational responsibilities.
	// The same evidence and observation gates apply to both signing models.
	if len(proof.Signoffs) == 1 && proof.Signoffs[0].Role == "sole_owner" {
		requiredRoles = map[string]bool{"sole_owner": false}
	}
	if len(proof.Signoffs) != len(requiredRoles) {
		return errors.New("one sole_owner or three role-specific release sign-offs are required")
	}
	canaryEndedAt := proof.CanaryStages[len(proof.CanaryStages)-1].EndedAt
	for _, item := range proof.Signoffs {
		if _, exists := requiredRoles[item.Role]; !exists || requiredRoles[item.Role] || strings.TrimSpace(item.Name) == "" || !item.Approved || item.ApprovedAt.IsZero() || item.ApprovedAt.Before(canaryEndedAt) {
			return fmt.Errorf("invalid release sign-off for role %q", item.Role)
		}
		requiredRoles[item.Role] = true
	}
	return nil
}

func verifyStage(item stage) error {
	if item.StartedAt.IsZero() || !item.EndedAt.After(item.StartedAt) || !item.JetStreamAvailable || !item.ControllerActive || item.ExpectedNodes < 3 || item.CurrentNodes != item.ExpectedNodes || item.MissingMessages != 0 || item.CorruptMessages != 0 || item.DLQTransferFailures != 0 {
		return errors.New("availability, topology or message integrity gate failed")
	}
	if item.Management5xxPercent < 0 || item.Management5xxPercent >= 5 || item.StoragePercent < 0 || item.StoragePercent >= 70 || item.BacklogLimitMessages <= 0 || item.BacklogMessages < 0 || item.BacklogMessages > item.BacklogLimitMessages {
		return errors.New("error, storage or backlog gate failed")
	}
	if item.PublishP99Millis <= 0 || item.PublishP99SLOMillis <= 0 || item.BaselinePublishP99Millis <= 0 || item.PublishP99Millis > item.PublishP99SLOMillis || item.PublishP99Millis > item.BaselinePublishP99Millis*1.30 {
		return errors.New("publish P99 gate failed")
	}
	if item.ThroughputPerSecond <= 0 || item.BaselineThroughput <= 0 || item.ThroughputPerSecond < item.BaselineThroughput*0.80 {
		return errors.New("throughput gate failed")
	}
	if item.DuplicateRatePercent < 0 || item.DuplicateBudgetPercent < 0 || item.DuplicateRatePercent > item.DuplicateBudgetPercent {
		return errors.New("duplicate-rate budget failed")
	}
	return nil
}

func verifyNodeFailure(item nodeFailure) error {
	if item.TrafficPercent < 1 || item.TrafficPercent > 10 || !item.Passed || !item.PubAckContinued || !item.ConsumeContinued || !item.ReplicasConverged || item.RecoverySeconds <= 0 || item.RecoverySLOSeconds <= 0 || item.RecoverySeconds > item.RecoverySLOSeconds {
		return errors.New("node-failure canary gate failed")
	}
	return nil
}

func verifyRollback(item rollback) error {
	if !item.Passed || !item.MessagesReconciled || !item.ServiceRestored || item.DurationSeconds <= 0 || item.SLOSeconds <= 0 || item.DurationSeconds > item.SLOSeconds {
		return errors.New("rollback rehearsal gate failed")
	}
	return nil
}

func validRevision(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

func safeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || strings.Contains(path, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && !strings.HasPrefix(clean, "../")
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
