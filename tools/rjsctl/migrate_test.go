package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestMigrateRabbitMQDefinitionsWritesValidatedQueuesAndReport(t *testing.T) {
	output := filepath.Join(t.TempDir(), "converted")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"migrate", "rabbitmq-definitions", "--output", output, "../../tests/fixtures/rabbitmq-definitions.json"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "converted 3/3") {
		t.Fatalf("stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	for _, name := range []string{"orders", "orders_dlq", "audit"} {
		file, err := os.Open(filepath.Join(output, name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		queue, parseErr := topology.ParseQueue(file)
		_ = file.Close()
		if parseErr != nil || queue.Metadata.Name != name {
			t.Fatalf("name=%s queue=%#v err=%v", name, queue, parseErr)
		}
	}
	report, err := os.ReadFile(filepath.Join(output, "migration-report.json"))
	if err != nil || !strings.Contains(string(report), `"compatible": true`) {
		t.Fatalf("report=%s err=%v", report, err)
	}
}

func TestMigrateRabbitMQStrictAndLossyModes(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "definitions.json")
	if err := os.WriteFile(input, []byte(`{"queues":[{"name":"temporary","vhost":"/","durable":false,"auto_delete":true,"arguments":{}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	strict := filepath.Join(directory, "strict")
	if err := run([]string{"migrate", "rabbitmq-definitions", "--output", strict, input}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("strict migration succeeded")
	}
	if _, err := os.Stat(filepath.Join(strict, "migration-report.json")); err != nil {
		t.Fatal("strict migration did not write its report")
	}
	lossy := filepath.Join(directory, "lossy")
	if err := run([]string{"migrate", "rabbitmq-definitions", "--allow-lossy", "--output", lossy, input}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateRabbitMQRejectsUsageAndExistingOutput(t *testing.T) {
	for _, args := range [][]string{{"migrate"}, {"migrate", "unknown"}, {"migrate", "rabbitmq-definitions"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("args=%v succeeded", args)
		}
	}
	output := t.TempDir()
	if err := run([]string{"migrate", "rabbitmq-definitions", "--output", output, "../../tests/fixtures/rabbitmq-definitions.json"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("existing output directory succeeded")
	}
}
