package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
)

type fakeCommandExecutor struct {
	commands [][]string
	failAt   int
}

func writeCutoverFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	evidence := make([]migration.CutoverEvidence, 2)
	for i := range evidence {
		report := migration.ReconciliationReport{Schema: migration.ReconciliationSchema, GeneratedAt: time.Now().UTC().Add(time.Duration(i-2) * time.Minute), Passed: true, SourceUnique: 1, Matched: 1}
		data, _ := json.Marshal(report)
		name := fmt.Sprintf("report-%d.json", i)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		evidence[i] = migration.CutoverEvidence{Path: name, SHA256: hex.EncodeToString(digest[:])}
	}
	plan := migration.CutoverPlan{Schema: migration.CutoverSchema, MigrationID: "unit-cutover", ReconciliationReports: evidence, Steps: []migration.CutoverStep{{Name: "route", Action: []string{"go", "version"}, Rollback: []string{"go", "version"}, Timeout: "30s", Idempotent: true}}}
	data, _ := json.Marshal(plan)
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return planPath, filepath.Join(dir, "journal.ndjson")
}

func TestRunCutoverApplyStatusRollback(t *testing.T) {
	plan, journal := writeCutoverFixture(t)
	stdout := &bytes.Buffer{}
	if err := runCutover([]string{"apply", "--plan", plan, "--journal", journal, "--confirm", "unit-cutover"}, stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := runCutover([]string{"status", "--plan", plan, "--journal", journal}, stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || status["finished"] != true {
		t.Fatalf("status=%v err=%v", status, err)
	}
	if err := runCutover([]string{"rollback", "--plan", plan, "--journal", journal, "--confirm", "unit-cutover"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := runCutover([]string{"status", "--plan", plan, "--journal", journal}, stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || status["rolled_back"] != true {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestRunCutoverRejectsUnsafeInvocation(t *testing.T) {
	plan, journal := writeCutoverFixture(t)
	for _, args := range [][]string{nil, {"unknown"}, {"apply"}, {"apply", "--plan", plan, "--journal", journal, "--confirm", "wrong"}, {"status", "--plan", plan, "--journal", journal}} {
		if err := runCutover(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted args=%v", args)
		}
	}
}

func (f *fakeCommandExecutor) run(_ context.Context, argv []string, _, _ io.Writer) error {
	f.commands = append(f.commands, append([]string(nil), argv...))
	if f.failAt > 0 && len(f.commands) == f.failAt {
		return errors.New("failed")
	}
	return nil
}

func TestCutoverResumeAndReverseRollback(t *testing.T) {
	plan := migration.CutoverPlan{Steps: []migration.CutoverStep{{Name: "one", Action: []string{"do", "one"}, Rollback: []string{"undo", "one"}, Idempotent: true}, {Name: "two", Action: []string{"do", "two"}, Rollback: []string{"undo", "two"}, Idempotent: true}}}
	path := filepath.Join(t.TempDir(), "state.ndjson")
	digest := strings.Repeat("a", 64)
	journal, state, unlock, err := openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandExecutor{failAt: 2}
	err = executeCutover(plan, digest, &state, journal, runner, &bytes.Buffer{}, &bytes.Buffer{})
	_ = journal.file.Close()
	unlock()
	if err == nil || !state.CutoverCompleted["one"] {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	journal, state, unlock, err = openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	runner = &fakeCommandExecutor{}
	if err := executeCutover(plan, digest, &state, journal, runner, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.commands, [][]string{{"do", "two"}}) {
		t.Fatalf("commands=%v", runner.commands)
	}
	runner = &fakeCommandExecutor{}
	if err := executeRollback(plan, digest, &state, journal, runner, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.commands, [][]string{{"undo", "two"}, {"undo", "one"}}) {
		t.Fatalf("commands=%v", runner.commands)
	}
	_ = journal.file.Close()
	unlock()
}

func TestCutoverRejectsPlanDriftAndConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.ndjson")
	digest := strings.Repeat("a", 64)
	journal, _, unlock, err := openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := openCutoverJournal(path, digest); err == nil {
		t.Fatal("concurrent writer accepted")
	}
	if err := journal.append(digest, "cutover", "one", "started"); err != nil {
		t.Fatal(err)
	}
	_ = journal.file.Close()
	unlock()
	if _, _, _, err := openCutoverJournal(path, strings.Repeat("b", 64)); err == nil {
		t.Fatal("plan drift accepted")
	}
}

func TestCutoverTerminalStatesAreIdempotentAndSafe(t *testing.T) {
	plan := migration.CutoverPlan{Steps: []migration.CutoverStep{{Name: "one", Action: []string{"do"}, Rollback: []string{"undo"}, Idempotent: true}}}
	path := filepath.Join(t.TempDir(), "state.ndjson")
	digest := strings.Repeat("a", 64)
	journal, _, unlock, err := openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandExecutor{}
	finished := migration.CutoverState{Finished: true, CutoverCompleted: map[string]bool{}, RollbackComplete: map[string]bool{}}
	if err := executeCutover(plan, digest, &finished, journal, runner, io.Discard, io.Discard); err != nil || len(runner.commands) != 0 {
		t.Fatalf("err=%v commands=%v", err, runner.commands)
	}
	rolledBack := migration.CutoverState{RolledBack: true, CutoverCompleted: map[string]bool{"one": true}, RollbackComplete: map[string]bool{"one": true}}
	if err := executeCutover(plan, digest, &rolledBack, journal, runner, io.Discard, io.Discard); err == nil {
		t.Fatal("cutover after rollback accepted")
	}
	if err := executeRollback(plan, digest, &rolledBack, journal, runner, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	empty := migration.CutoverState{CutoverCompleted: map[string]bool{}, RollbackComplete: map[string]bool{}}
	if err := executeRollback(plan, digest, &empty, journal, runner, io.Discard, io.Discard); err == nil {
		t.Fatal("empty rollback accepted")
	}
	_ = journal.file.Close()
	unlock()
}

func TestRollbackFailureIsJournaledAndRecoverable(t *testing.T) {
	plan := migration.CutoverPlan{Steps: []migration.CutoverStep{{Name: "one", Action: []string{"do"}, Rollback: []string{"undo"}, Idempotent: true}}}
	path := filepath.Join(t.TempDir(), "state.ndjson")
	digest := strings.Repeat("a", 64)
	journal, state, unlock, err := openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	state.CutoverCompleted["one"] = true
	if err := executeRollback(plan, digest, &state, journal, &fakeCommandExecutor{failAt: 1}, io.Discard, io.Discard); err == nil {
		t.Fatal("rollback command failure ignored")
	}
	if state.RollbackComplete["one"] {
		t.Fatal("failed rollback marked complete")
	}
	_ = journal.file.Close()
	unlock()
	journal, state, unlock, err = openCutoverJournal(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	state.CutoverCompleted["one"] = true
	if err := executeRollback(plan, digest, &state, journal, &fakeCommandExecutor{}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	_ = journal.file.Close()
	unlock()
}
