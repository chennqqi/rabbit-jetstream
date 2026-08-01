package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validCutoverPlan() CutoverPlan {
	return CutoverPlan{Schema: CutoverSchema, MigrationID: "orders-2026-08", ReconciliationReports: []CutoverEvidence{{Path: "one.json", SHA256: strings.Repeat("a", 64)}, {Path: "two.json", SHA256: strings.Repeat("b", 64)}}, Steps: []CutoverStep{{Name: "publishers", Action: []string{"ctl", "enable-js"}, Rollback: []string{"ctl", "enable-rabbit"}, Idempotent: true}, {Name: "consumers", Action: []string{"ctl", "enable-js-consumers"}, Rollback: []string{"ctl", "enable-rabbit-consumers"}, Timeout: "30s", Idempotent: true}}}
}

func TestCutoverPlanValidationAndDigest(t *testing.T) {
	plan := validCutoverPlan()
	if err := ValidateCutoverPlan(plan); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(plan)
	if len(CutoverPlanDigest(data)) != 64 {
		t.Fatal("invalid digest")
	}
	invalid := []CutoverPlan{
		{},
		{Schema: CutoverSchema, MigrationID: "x", ReconciliationReports: []CutoverEvidence{{Path: "one"}}, Steps: plan.Steps},
		{Schema: CutoverSchema, MigrationID: "x", ReconciliationReports: plan.ReconciliationReports, Steps: []CutoverStep{{Name: "x", Action: []string{"a"}}}},
	}
	for _, value := range invalid {
		if ValidateCutoverPlan(value) == nil {
			t.Fatalf("accepted %+v", value)
		}
	}
}

func TestReadCutoverPlanIsStrict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	data, _ := json.Marshal(validCutoverPlan())
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, canonical, err := ReadCutoverPlan(path)
	if err != nil || plan.MigrationID == "" || len(canonical) == 0 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":"`+CutoverSchema+`","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadCutoverPlan(path); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestVerifyCutoverEvidenceRequiresPassingFreshOrderedWindows(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	plan := validCutoverPlan()
	write := func(name string, at time.Time, passed bool) string {
		t.Helper()
		report := ReconciliationReport{Schema: ReconciliationSchema, GeneratedAt: at, Passed: passed}
		data, _ := json.Marshal(report)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		return hex.EncodeToString(digest[:])
	}
	plan.ReconciliationReports[0].SHA256 = write("one.json", now.Add(-2*time.Minute), true)
	plan.ReconciliationReports[1].SHA256 = write("two.json", now.Add(-time.Minute), true)
	if err := VerifyCutoverEvidence(plan, dir, now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	plan.ReconciliationReports[1].SHA256 = write("two.json", now.Add(-10*time.Minute), true)
	if err := VerifyCutoverEvidence(plan, dir, now, 5*time.Minute); err == nil {
		t.Fatal("stale or unordered evidence accepted")
	}
	plan.ReconciliationReports[1].SHA256 = write("two.json", now.Add(-time.Minute), false)
	if err := VerifyCutoverEvidence(plan, dir, now, 5*time.Minute); err == nil {
		t.Fatal("failed evidence accepted")
	}
}

func TestCutoverJournalReconstructsAndRejectsDrift(t *testing.T) {
	digest := strings.Repeat("a", 64)
	at := time.Now().UTC().Format(time.RFC3339Nano)
	journal := `{"plan_sha256":"` + digest + `","operation":"cutover","step":"publishers","status":"started","at":"` + at + `"}` + "\n" + `{"plan_sha256":"` + digest + `","operation":"cutover","step":"publishers","status":"completed","at":"` + at + `"}`
	state, err := ReadCutoverJournal(strings.NewReader(journal))
	if err != nil || !state.CutoverCompleted["publishers"] {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	steps := CompletedCutoverSteps(validCutoverPlan(), CutoverState{CutoverCompleted: map[string]bool{"publishers": true, "consumers": true}})
	if len(steps) != 2 || steps[0].Name != "consumers" {
		t.Fatalf("steps=%+v", steps)
	}
	if _, err := ReadCutoverJournal(strings.NewReader(journal + "\n" + `{"plan_sha256":"` + strings.Repeat("b", 64) + `","operation":"cutover","step":"x","status":"started","at":"` + at + `"}`)); err == nil {
		t.Fatal("digest drift accepted")
	}
}
