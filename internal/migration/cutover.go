package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const CutoverSchema = "rabbit-jetstream.io/cutover-plan/v1alpha1"

var cutoverNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type CutoverStep struct {
	Name       string   `json:"name"`
	Action     []string `json:"action"`
	Rollback   []string `json:"rollback"`
	Timeout    string   `json:"timeout,omitempty"`
	Idempotent bool     `json:"idempotent"`
}

type CutoverEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type CutoverPlan struct {
	Schema                string            `json:"schema"`
	MigrationID           string            `json:"migration_id"`
	ReconciliationReports []CutoverEvidence `json:"reconciliation_reports"`
	Steps                 []CutoverStep     `json:"steps"`
}

type CutoverEvent struct {
	PlanSHA256 string    `json:"plan_sha256"`
	Operation  string    `json:"operation"`
	Step       string    `json:"step,omitempty"`
	Status     string    `json:"status"`
	At         time.Time `json:"at"`
}

type CutoverState struct {
	PlanSHA256       string
	CutoverCompleted map[string]bool
	RollbackComplete map[string]bool
	Finished         bool
	RolledBack       bool
}

func ReadCutoverPlan(path string) (CutoverPlan, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CutoverPlan{}, nil, fmt.Errorf("read cutover plan: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var plan CutoverPlan
	if err := decoder.Decode(&plan); err != nil {
		return plan, nil, fmt.Errorf("decode cutover plan: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return plan, nil, fmt.Errorf("decode cutover plan: %w", err)
	}
	if err := ValidateCutoverPlan(plan); err != nil {
		return plan, nil, err
	}
	canonical, _ := json.Marshal(plan)
	return plan, canonical, nil
}

func ValidateCutoverPlan(plan CutoverPlan) error {
	if plan.Schema != CutoverSchema {
		return fmt.Errorf("schema must be %q", CutoverSchema)
	}
	if !cutoverNamePattern.MatchString(plan.MigrationID) {
		return errors.New("migration_id must match [A-Za-z0-9][A-Za-z0-9._-]{0,127}")
	}
	if len(plan.ReconciliationReports) < 2 {
		return errors.New("at least two consecutive reconciliation reports are required")
	}
	evidencePaths := map[string]bool{}
	for _, evidence := range plan.ReconciliationReports {
		digest, err := hex.DecodeString(evidence.SHA256)
		if strings.TrimSpace(evidence.Path) == "" || evidencePaths[evidence.Path] || err != nil || len(digest) != sha256.Size {
			return errors.New("reconciliation report paths must be unique and each requires a SHA-256 digest")
		}
		evidencePaths[evidence.Path] = true
	}
	if len(plan.Steps) == 0 {
		return errors.New("at least one cutover step is required")
	}
	seen := map[string]bool{}
	for i, step := range plan.Steps {
		if !cutoverNamePattern.MatchString(step.Name) || seen[step.Name] {
			return fmt.Errorf("invalid or duplicate step name at index %d", i)
		}
		seen[step.Name] = true
		if err := validateCommand(step.Action); err != nil {
			return fmt.Errorf("step %q action: %w", step.Name, err)
		}
		if err := validateCommand(step.Rollback); err != nil {
			return fmt.Errorf("step %q rollback: %w", step.Name, err)
		}
		if !step.Idempotent {
			return fmt.Errorf("step %q must declare idempotent=true for crash-safe resume", step.Name)
		}
		if _, err := StepTimeout(step); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
	}
	return nil
}

func validateCommand(command []string) error {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return errors.New("command argv must not be empty")
	}
	for _, arg := range command {
		if strings.ContainsRune(arg, '\x00') {
			return errors.New("command argv contains NUL")
		}
	}
	return nil
}

func StepTimeout(step CutoverStep) (time.Duration, error) {
	if step.Timeout == "" {
		return 2 * time.Minute, nil
	}
	d, err := time.ParseDuration(step.Timeout)
	if err != nil || d <= 0 || d > 30*time.Minute {
		return 0, errors.New("timeout must be between 1ns and 30m")
	}
	return d, nil
}

func CutoverPlanDigest(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func VerifyCutoverEvidence(plan CutoverPlan, planDirectory string, now time.Time, maxAge time.Duration) error {
	if maxAge <= 0 {
		return errors.New("evidence max age must be positive")
	}
	previous := time.Time{}
	var baseline *Thresholds
	for _, evidence := range plan.ReconciliationReports {
		if strings.TrimSpace(evidence.Path) == "" {
			return errors.New("reconciliation evidence path is required")
		}
		expected, err := hex.DecodeString(strings.ToLower(evidence.SHA256))
		if err != nil || len(expected) != sha256.Size {
			return fmt.Errorf("reconciliation report %q has invalid sha256", evidence.Path)
		}
		path := evidence.Path
		if !strings.ContainsAny(evidence.Path, `/\`) || !isAbsolutePath(evidence.Path) {
			path = joinPath(planDirectory, evidence.Path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("open reconciliation report %q: %w", evidence.Path, err)
		}
		actual := sha256.Sum256(raw)
		if !strings.EqualFold(hex.EncodeToString(actual[:]), evidence.SHA256) {
			return fmt.Errorf("reconciliation report %q sha256 mismatch", evidence.Path)
		}
		var report ReconciliationReport
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&report)
		if err != nil {
			return fmt.Errorf("decode reconciliation report %q: %w", evidence.Path, err)
		}
		if err := requireJSONEOF(decoder); err != nil {
			return fmt.Errorf("decode reconciliation report %q: %w", evidence.Path, err)
		}
		if report.Schema != ReconciliationSchema || !report.Passed {
			return fmt.Errorf("reconciliation report %q is not a passing %s report", evidence.Path, ReconciliationSchema)
		}
		if baseline == nil {
			value := report.Thresholds
			baseline = &value
		} else if report.Thresholds.MinSource != baseline.MinSource || report.Thresholds.MaxMissing != baseline.MaxMissing || report.Thresholds.MaxUnexpected != baseline.MaxUnexpected || report.Thresholds.MaxMismatch != baseline.MaxMismatch || report.Thresholds.MaxDuplicates != baseline.MaxDuplicates {
			return errors.New("reconciliation reports must use identical gate thresholds")
		}
		if report.GeneratedAt.After(now.Add(time.Minute)) || now.Sub(report.GeneratedAt) > maxAge {
			return fmt.Errorf("reconciliation report %q is outside the evidence age window", evidence.Path)
		}
		if !previous.IsZero() && !report.GeneratedAt.After(previous) {
			return errors.New("reconciliation reports must be ordered oldest to newest with distinct timestamps")
		}
		previous = report.GeneratedAt
	}
	return nil
}

// Small wrappers keep path behavior platform-native and independently testable.
var joinPath = func(base, name string) string { return base + string(os.PathSeparator) + name }
var isAbsolutePath = func(path string) bool { return len(path) > 2 && (path[1] == ':' || path[0] == '/' || path[0] == '\\') }

func ReadCutoverJournal(input io.Reader) (CutoverState, error) {
	state := CutoverState{CutoverCompleted: map[string]bool{}, RollbackComplete: map[string]bool{}}
	decoder := json.NewDecoder(input)
	for {
		var event CutoverEvent
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return state, fmt.Errorf("decode cutover journal: %w", err)
		}
		if len(event.PlanSHA256) != 64 || event.At.IsZero() {
			return state, errors.New("invalid cutover journal event")
		}
		if state.PlanSHA256 != "" && state.PlanSHA256 != event.PlanSHA256 {
			return state, errors.New("cutover journal plan digest conflict")
		}
		state.PlanSHA256 = event.PlanSHA256
		switch {
		case event.Operation == "cutover" && event.Status == "completed" && event.Step != "":
			state.CutoverCompleted[event.Step] = true
		case event.Operation == "cutover" && event.Status == "finished" && event.Step == "":
			state.Finished = true
		case event.Operation == "rollback" && event.Status == "completed" && event.Step != "":
			state.RollbackComplete[event.Step] = true
		case event.Operation == "rollback" && event.Status == "finished" && event.Step == "":
			state.RolledBack = true
		case (event.Operation == "cutover" || event.Operation == "rollback") && event.Status == "started":
		default:
			return state, errors.New("invalid cutover journal event transition")
		}
	}
	return state, nil
}

func CompletedCutoverSteps(plan CutoverPlan, state CutoverState) []CutoverStep {
	steps := []CutoverStep{}
	for _, step := range plan.Steps {
		if state.CutoverCompleted[step.Name] {
			steps = append(steps, step)
		}
	}
	sort.SliceStable(steps, func(i, j int) bool { return indexOfStep(plan, steps[i].Name) > indexOfStep(plan, steps[j].Name) })
	return steps
}

func indexOfStep(plan CutoverPlan, name string) int {
	for i := range plan.Steps {
		if plan.Steps[i].Name == name {
			return i
		}
	}
	return -1
}
