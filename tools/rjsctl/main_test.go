package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const queueFile = `apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata: {name: orders}
spec:
  subjects: [orders.>]
  replicas: 3
`

func TestQueueValidateAndDiff(t *testing.T) {
	directory := t.TempDir()
	current := filepath.Join(directory, "current.yaml")
	desired := filepath.Join(directory, "desired.yaml")
	if err := os.WriteFile(current, []byte(queueFile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desired, []byte(strings.Replace(queueFile, "replicas: 3", "replicas: 1", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"queue", "validate", current}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "valid Queue orders") {
		t.Fatalf("output = %q", output.String())
	}
	output.Reset()
	if err := run([]string{"queue", "diff", current, desired}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"path": "spec.replicas"`) || !strings.Contains(output.String(), `"impact": "disruptive"`) {
		t.Fatalf("output = %q", output.String())
	}
	output.Reset()
	if err := run([]string{"queue", "plan", current}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"name": "RJSQ_orders"`) || !strings.Contains(output.String(), `"retention": "workqueue"`) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestQueueValidateRejectsInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(path, []byte(strings.Replace(queueFile, "replicas: 3", "replicas: 2", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"queue", "validate", path}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("validation succeeded")
	}
}

func TestQueueReconcilePlansCreateWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.yaml")
	if err := os.WriteFile(path, []byte(queueFile), 0o600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/streams/RJSQ_orders" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run([]string{"queue", "reconcile", "--url", server.URL, path}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || !strings.Contains(output.String(), `"status": "ready"`) || strings.Count(output.String(), `"action": "create"`) != 2 {
		t.Fatalf("requests = %d, output = %s", requests, output.String())
	}
}
