package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
	"github.com/nats-io/nats.go"
)

func TestEvidenceWriterPersistsCanonicalObservationAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.ndjson")
	w, err := newEvidenceWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.write("message-1", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got migration.Observation
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "message-1" || got.Size != 7 || got.SHA256 != "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5" {
		t.Fatalf("observation=%+v", got)
	}
	if _, err := newEvidenceWriter(path); err == nil {
		t.Fatal("existing evidence was overwritten")
	}
}

func TestShadowCaptureRejectsWorkQueueRetention(t *testing.T) {
	if err := validateShadowRetention(nats.WorkQueuePolicy); err == nil {
		t.Fatal("WorkQueue shadow capture was accepted")
	}
	if err := validateShadowRetention(nats.LimitsPolicy); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRejectsUnsafeUsageBeforeCreatingEvidence(t *testing.T) {
	for _, args := range [][]string{{}, {"unknown"}, {"rabbitmq"}, {"jetstream"}, {"rabbitmq", "--url", "amqp://localhost", "--queue", "q", "--output", "x", "--count", "1", "--timeout", "0s"}} {
		if err := runCapture(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("args=%v succeeded", args)
		}
	}
}

func TestCaptureConnectionFailuresAreSafeAndDoNotCreateEvidence(t *testing.T) {
	for _, test := range []struct {
		args   []string
		output string
	}{
		{[]string{"rabbitmq", "--url", "amqp://user:secret@127.0.0.1:1/", "--queue", "shadow", "--count", "1", "--timeout", "1s"}, "rabbit.ndjson"},
		{[]string{"jetstream", "--url", "nats://user:secret@127.0.0.1:1", "--stream", "SHADOW", "--filter", "shadow.events", "--count", "1", "--timeout", "1s"}, "nats.ndjson"},
	} {
		path := filepath.Join(t.TempDir(), test.output)
		args := append(test.args, "--output", path)
		err := runCapture(args, &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("args=%v err=%v", args, err)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("evidence created after connection failure: %v", statErr)
		}
	}
}

func TestConnectionErrorsRedactCredentials(t *testing.T) {
	err := sanitizedConnectionError("RabbitMQ", "amqp://alice:secret@example.test/vhost", bytes.ErrTooLarge)
	if strings.Contains(err.Error(), "alice") || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "amqp://example.test/vhost") {
		t.Fatalf("error=%s", err)
	}
}
