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

func TestQueueApplyUsesAuthenticatedPut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.yaml")
	if err := os.WriteFile(path, []byte(queueFile), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/queues/orders" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queue":"orders","revision":"abc","status":"ready","blocked":false,"operations":[]}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run([]string{"queue", "apply", "--url", server.URL, "--token", "secret", path}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"queue": "orders"`) {
		t.Fatalf("output = %s", output.String())
	}
}

func TestQueueApplyRequiresToken(t *testing.T) {
	t.Setenv("RJS_ADMIN_TOKEN", "")
	path := filepath.Join(t.TempDir(), "queue.yaml")
	if err := os.WriteFile(path, []byte(queueFile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"queue", "apply", path}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("apply succeeded without token")
	}
}

func TestQueueDeleteUsesAuthenticatedConfirmedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/queues/orders" || r.URL.Query().Get("force") != "true" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-RJS-Confirm-Queue") != "orders" {
			t.Fatal("missing delete authorization or confirmation")
		}
		_, _ = w.Write([]byte(`{"queue":"orders","stream":"RJSQ_orders","status":"deleted","blocked":false,"forced":true,"messages":1}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run([]string{"queue", "delete", "--url", server.URL, "--token", "secret", "--confirm", "orders", "--force", "orders"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"status": "deleted"`) {
		t.Fatalf("output = %s", output.String())
	}
}

func TestQueueDeleteRejectsMissingConfirmation(t *testing.T) {
	if err := run([]string{"queue", "delete", "--token", "secret", "orders"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("delete succeeded without confirmation")
	}
}

func TestQueueListReadsDeclarations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/queues" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[{"queue":"orders","revision":"abc"}],"total":1,"offset":0,"limit":200}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run([]string{"queue", "list", "--url", server.URL}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"revision": "abc"`) {
		t.Fatalf("output = %s", output.String())
	}
}
