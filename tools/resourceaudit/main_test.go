package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditSummarizesThreeHealthyNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.ndjson")
	writeSamples(t, path, false)
	result, err := audit(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Samples != 6 || len(result.Nodes) != 3 || result.Nodes["nats-1"].MaxMemoryBytes != 101 || result.MinHostFreeBytes != 999 {
		t.Fatalf("summary = %#v", result)
	}
}

func TestAuditRejectsUnhealthyAndUnevenSamples(t *testing.T) {
	for _, mutate := range []func(*sample){
		func(value *sample) { healthy := false; value.Healthy = &healthy },
		func(value *sample) { value.Server.SlowConsumers = 1 },
		func(value *sample) { value.Server.StalledClients = 1 },
	} {
		path := filepath.Join(t.TempDir(), "resources.ndjson")
		writeSamples(t, path, false)
		lines, _ := os.ReadFile(path)
		parts := bytes.Split(bytes.TrimSpace(lines), []byte{'\n'})
		var value sample
		if err := json.Unmarshal(parts[0], &value); err != nil {
			t.Fatal(err)
		}
		mutate(&value)
		parts[0], _ = json.Marshal(value)
		if err := os.WriteFile(path, append(bytes.Join(parts, []byte{'\n'}), '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := audit(path); err == nil {
			t.Fatal("invalid samples were accepted")
		}
	}
	path := filepath.Join(t.TempDir(), "resources.ndjson")
	writeSamples(t, path, true)
	if _, err := audit(path); err == nil || !strings.Contains(err.Error(), "counts differ") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRequiresInputAndUsesExclusiveOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(nil, &stdout, &stderr); err == nil {
		t.Fatal("missing input accepted")
	}
	path := filepath.Join(t.TempDir(), "resources.ndjson")
	writeSamples(t, path, false)
	output := filepath.Join(t.TempDir(), "summary.json")
	if err := run([]string{"-input", path, "-output", output}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-input", path, "-output", output}, &stdout, &stderr); err == nil {
		t.Fatal("existing output overwritten")
	}
}

func writeSamples(t *testing.T, path string, uneven bool) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	start := time.Now().UTC().Add(-time.Minute)
	for round := 0; round < 2; round++ {
		for node := 1; node <= 3; node++ {
			if uneven && round == 1 && node == 3 {
				continue
			}
			healthy := true
			value := sample{CapturedAt: start.Add(time.Duration(round) * time.Minute), Node: "nats-" + string(rune('0'+node)), Healthy: &healthy, HostFreeBytes: 1000 - int64(round)}
			value.Server.Mem = int64(100 + round)
			value.Server.CPU = float64(node)
			value.Server.JetStream.Stats.Storage = int64(round)
			if err := json.NewEncoder(file).Encode(value); err != nil {
				t.Fatal(err)
			}
		}
	}
}
