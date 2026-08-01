package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
	"gopkg.in/yaml.v3"
)

func runMigrate(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: rjsctl migrate rabbitmq-definitions|reconcile [flags]")
	}
	if args[0] == "capture" {
		return runCapture(args[1:], stdout, stderr)
	}
	if args[0] == "dual-write" {
		return runDualWrite(args[1:], stdout, stderr)
	}
	if args[0] == "reconcile" {
		return runReconcile(args[1:], stdout, stderr)
	}
	if args[0] != "rabbitmq-definitions" {
		return errors.New("usage: rjsctl migrate rabbitmq-definitions|reconcile [flags]")
	}
	fs := flag.NewFlagSet("migrate rabbitmq-definitions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "new output directory")
	vhost := fs.String("vhost", "/", "RabbitMQ virtual host to convert")
	replicas := fs.Int("replicas", 3, "Queue replica count (1, 3, or 5)")
	allowLossy := fs.Bool("allow-lossy", false, "write compatible subset even when the report contains errors")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 || *output == "" {
		return errors.New("usage: rjsctl migrate rabbitmq-definitions --output DIRECTORY [--vhost VHOST] [--replicas N] [--allow-lossy] FILE")
	}
	input, err := os.Open(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("open RabbitMQ definitions: %w", err)
	}
	defer input.Close()
	result, err := migration.ConvertRabbitMQ(input, *vhost, *replicas)
	if err != nil {
		return err
	}
	if err := writeMigrationResult(*output, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "converted %d/%d Queues; report written to %s\n", result.Report.Converted, result.Report.Queues, filepath.Join(*output, "migration-report.json"))
	if !result.Report.Compatible && !*allowLossy {
		return errors.New("RabbitMQ definitions contain unsupported semantics; inspect migration-report.json (use --allow-lossy only after review)")
	}
	return nil
}

func runReconcile(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sourcePath := fs.String("source", "", "RabbitMQ shadow observation NDJSON")
	targetPath := fs.String("target", "", "JetStream shadow observation NDJSON")
	output := fs.String("output", "", "new reconciliation report JSON file")
	maxMissing := fs.Int("max-missing", 0, "maximum missing message IDs")
	minSource := fs.Int("min-source", 1, "minimum unique source message IDs required for a qualifying window")
	maxUnexpected := fs.Int("max-unexpected", 0, "maximum unexpected target message IDs")
	maxMismatch := fs.Int("max-mismatch", 0, "maximum content mismatches")
	maxDuplicates := fs.Int("max-duplicates", 0, "maximum duplicate observations across both sides")
	maxDetails := fs.Int("max-details", 1000, "maximum issue details retained in the report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *sourcePath == "" || *targetPath == "" || *output == "" {
		return errors.New("usage: rjsctl migrate reconcile --source RABBIT.ndjson --target JETSTREAM.ndjson --output REPORT.json [threshold flags]")
	}
	source, err := os.Open(*sourcePath)
	if err != nil {
		return fmt.Errorf("open source observations: %w", err)
	}
	defer source.Close()
	target, err := os.Open(*targetPath)
	if err != nil {
		return fmt.Errorf("open target observations: %w", err)
	}
	defer target.Close()
	report, err := migration.ReconcileMessages(source, target, migration.Thresholds{MinSource: *minSource, MaxMissing: *maxMissing, MaxUnexpected: *maxUnexpected, MaxMismatch: *maxMismatch, MaxDuplicates: *maxDuplicates, MaxDetails: *maxDetails})
	if err != nil {
		return err
	}
	if err := writeReconciliationReport(*output, report); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "matched=%d missing=%d unexpected=%d mismatched=%d duplicates=%d passed=%t report=%s\n", report.Matched, report.Missing, report.Unexpected, report.ContentMismatch, report.SourceDuplicates+report.TargetDuplicates, report.Passed, *output)
	if !report.Passed {
		return errors.New("message reconciliation failed configured migration thresholds")
	}
	return nil
}

func writeReconciliationReport(path string, report migration.ReconciliationReport) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create reconciliation report: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(report)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode reconciliation report: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close reconciliation report: %w", closeErr)
	}
	return nil
}

func writeMigrationResult(directory string, result migration.Result) error {
	if err := os.Mkdir(directory, 0o750); err != nil {
		return fmt.Errorf("create migration output: %w", err)
	}
	for _, queue := range result.Queues {
		data, err := yaml.Marshal(queue)
		if err != nil {
			return fmt.Errorf("encode Queue %s: %w", queue.Metadata.Name, err)
		}
		path := filepath.Join(directory, queue.Metadata.Name+".yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write Queue %s: %w", queue.Metadata.Name, err)
		}
	}
	report, err := json.MarshalIndent(result.Report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode migration report: %w", err)
	}
	report = append(report, '\n')
	if err := os.WriteFile(filepath.Join(directory, "migration-report.json"), report, 0o600); err != nil {
		return fmt.Errorf("write migration report: %w", err)
	}
	return nil
}
