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
	if len(args) < 1 || args[0] != "rabbitmq-definitions" {
		return errors.New("usage: rjsctl migrate rabbitmq-definitions --output DIRECTORY [--vhost VHOST] [--replicas N] [--allow-lossy] FILE")
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
