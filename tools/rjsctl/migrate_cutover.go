package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
)

type cutoverJournal struct {
	file    *os.File
	encoder *json.Encoder
}

func openCutoverJournal(path, digest string) (*cutoverJournal, migration.CutoverState, func(), error) {
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, migration.CutoverState{}, nil, fmt.Errorf("acquire cutover lock: %w", err)
	}
	_, _ = fmt.Fprintf(lock, "pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	_ = lock.Close()
	unlock := func() { _ = os.Remove(lockPath) }
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		unlock()
		return nil, migration.CutoverState{}, nil, err
	}
	state, err := migration.ReadCutoverJournal(bufio.NewReader(file))
	if err != nil {
		_ = file.Close()
		unlock()
		return nil, state, nil, err
	}
	if state.PlanSHA256 != "" && state.PlanSHA256 != digest {
		_ = file.Close()
		unlock()
		return nil, state, nil, errors.New("cutover plan differs from the journal-bound plan")
	}
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		unlock()
		return nil, state, nil, err
	}
	return &cutoverJournal{file, json.NewEncoder(file)}, state, unlock, nil
}

func (j *cutoverJournal) append(digest, operation, step, status string) error {
	if err := j.encoder.Encode(migration.CutoverEvent{PlanSHA256: digest, Operation: operation, Step: step, Status: status, At: time.Now().UTC()}); err != nil {
		return err
	}
	return j.file.Sync()
}

type commandExecutor interface {
	run(context.Context, []string, io.Writer, io.Writer) error
}
type osCommandExecutor struct{}

func (osCommandExecutor) run(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Stdout, command.Stderr, command.Stdin = stdout, stderr, nil
	return command.Run()
}

func executeCutover(plan migration.CutoverPlan, digest string, state *migration.CutoverState, journal *cutoverJournal, runner commandExecutor, stdout, stderr io.Writer) error {
	if state.RolledBack {
		return errors.New("migration was rolled back; create a new migration plan and ID")
	}
	if state.Finished {
		return nil
	}
	for _, step := range plan.Steps {
		if state.CutoverCompleted[step.Name] {
			continue
		}
		if err := journal.append(digest, "cutover", step.Name, "started"); err != nil {
			return err
		}
		timeout, _ := migration.StepTimeout(step)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := runner.run(ctx, step.Action, stdout, stderr)
		cancel()
		if err != nil {
			return fmt.Errorf("cutover step %q failed: %w", step.Name, err)
		}
		if err := journal.append(digest, "cutover", step.Name, "completed"); err != nil {
			return err
		}
		state.CutoverCompleted[step.Name] = true
	}
	if err := journal.append(digest, "cutover", "", "finished"); err != nil {
		return err
	}
	state.Finished = true
	return nil
}

func executeRollback(plan migration.CutoverPlan, digest string, state *migration.CutoverState, journal *cutoverJournal, runner commandExecutor, stdout, stderr io.Writer) error {
	if state.RolledBack {
		return nil
	}
	steps := migration.CompletedCutoverSteps(plan, *state)
	if len(steps) == 0 {
		return errors.New("no completed cutover steps are available to roll back")
	}
	for _, step := range steps {
		if state.RollbackComplete[step.Name] {
			continue
		}
		if err := journal.append(digest, "rollback", step.Name, "started"); err != nil {
			return err
		}
		timeout, _ := migration.StepTimeout(step)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := runner.run(ctx, step.Rollback, stdout, stderr)
		cancel()
		if err != nil {
			return fmt.Errorf("rollback step %q failed: %w", step.Name, err)
		}
		if err := journal.append(digest, "rollback", step.Name, "completed"); err != nil {
			return err
		}
		state.RollbackComplete[step.Name] = true
	}
	if err := journal.append(digest, "rollback", "", "finished"); err != nil {
		return err
	}
	state.RolledBack = true
	return nil
}

func runCutover(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 || (args[0] != "apply" && args[0] != "rollback" && args[0] != "status") {
		return errors.New("usage: rjsctl migrate cutover apply|rollback|status --plan FILE --journal FILE [flags]")
	}
	operation := args[0]
	fs := flag.NewFlagSet("migrate cutover "+operation, flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", "", "cutover plan JSON")
	journalPath := fs.String("journal", "", "append-only state journal")
	confirm := fs.String("confirm", "", "must exactly match migration_id for apply or rollback")
	maxEvidenceAge := fs.Duration("max-evidence-age", 30*time.Minute, "maximum reconciliation report age")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *planPath == "" || *journalPath == "" {
		return errors.New("usage: rjsctl migrate cutover apply|rollback|status --plan FILE --journal FILE [--confirm MIGRATION_ID] [--max-evidence-age DURATION]")
	}
	plan, canonical, err := migration.ReadCutoverPlan(*planPath)
	if err != nil {
		return err
	}
	digest := migration.CutoverPlanDigest(canonical)
	if operation == "status" {
		file, err := os.Open(*journalPath)
		if err != nil {
			return fmt.Errorf("open cutover journal: %w", err)
		}
		state, readErr := migration.ReadCutoverJournal(bufio.NewReader(file))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if state.PlanSHA256 != "" && state.PlanSHA256 != digest {
			return errors.New("cutover plan differs from the journal-bound plan")
		}
		return json.NewEncoder(stdout).Encode(map[string]any{"migration_id": plan.MigrationID, "plan_sha256": digest, "finished": state.Finished, "rolled_back": state.RolledBack, "completed_steps": state.CutoverCompleted, "rollback_steps": state.RollbackComplete})
	}
	if *confirm != plan.MigrationID {
		return errors.New("--confirm must exactly match migration_id")
	}
	journal, state, unlock, err := openCutoverJournal(*journalPath, digest)
	if err != nil {
		return err
	}
	defer unlock()
	defer journal.file.Close()
	if operation == "apply" {
		if err := migration.VerifyCutoverEvidence(plan, filepath.Dir(*planPath), time.Now().UTC(), *maxEvidenceAge); err != nil {
			return fmt.Errorf("cutover evidence gate failed: %w", err)
		}
		err = executeCutover(plan, digest, &state, journal, osCommandExecutor{}, stdout, stderr)
	} else {
		err = executeRollback(plan, digest, &state, journal, osCommandExecutor{}, stdout, stderr)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "migration %s %s completed; journal=%s\n", plan.MigrationID, operation, *journalPath)
	return nil
}
