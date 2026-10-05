package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

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
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/queues/orders" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("If-None-Match") != "*" {
			t.Fatalf("If-None-Match = %q", r.Header.Get("If-None-Match"))
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

func TestQueueApplyUsesExistingETagForUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.yaml")
	if err := os.WriteFile(path, []byte(queueFile), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("ETag", `"23"`)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.Header.Get("If-Match") != `"23"` {
			t.Fatalf("If-Match=%q", r.Header.Get("If-Match"))
		}
		_, _ = w.Write([]byte(`{"queue":"orders","revision":"abc","status":"noop","blocked":false,"operations":[]}`))
	}))
	defer server.Close()
	if err := run([]string{"queue", "apply", "--url", server.URL, "--token", "secret", path}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestQueueDeleteUsesAuthenticatedConfirmedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/queues/orders" || r.URL.Query().Get("force") != "true" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-RJS-Confirm-Queue") != "orders" {
			t.Fatal("missing delete authorization or confirmation")
		}
		if r.Header.Get("If-None-Match") != "*" {
			t.Fatal("missing delete precondition")
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

func TestAuditListUsesAdminTokenAndPagination(t *testing.T) {
	t.Setenv("RJS_ADMIN_TOKEN", "environment-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/audit" || r.URL.Query().Get("offset") != "2" || r.URL.Query().Get("limit") != "25" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer environment-secret" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"audit-1"}],"total":3,"offset":2,"limit":25}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run([]string{"audit", "list", "--url", server.URL, "--offset", "2", "--limit", "25"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"id": "audit-1"`) {
		t.Fatalf("output=%s", output.String())
	}
}

func TestAuditListRejectsInvalidArgumentsAndAPIError(t *testing.T) {
	t.Setenv("RJS_ADMIN_TOKEN", "")
	for _, args := range [][]string{{"audit"}, {"audit", "unknown"}, {"audit", "list"}, {"audit", "list", "--token", "x", "--offset", "-1"}, {"audit", "list", "--token", "x", "--limit", "201"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("args=%v unexpectedly succeeded", args)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if err := run([]string{"audit", "list", "--url", server.URL, "--token", "x"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err=%v", err)
	}
}

func TestTopLevelCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}} {
		var output bytes.Buffer
		if err := run(args, &output, &output); err != nil || !strings.Contains(output.String(), "Usage:") {
			t.Fatalf("args=%v output=%q err=%v", args, output.String(), err)
		}
	}
	var output bytes.Buffer
	if err := run([]string{"version"}, &output, &output); err != nil || !strings.Contains(output.String(), version) {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
	for _, args := range [][]string{{"unknown"}, {"queue"}, {"queue", "unknown"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("args=%v unexpectedly succeeded", args)
		}
	}
}

func TestStatusSuccessAndFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/info" {
				t.Fatalf("path=%s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"version":"test"}`))
		}))
		defer server.Close()
		var output bytes.Buffer
		if err := run([]string{"status", "--url", server.URL}, &output, &output); err != nil || !strings.Contains(output.String(), `"version": "test"`) {
			t.Fatalf("output=%q err=%v", output.String(), err)
		}
	})
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"bad status", "unavailable", http.StatusServiceUnavailable},
		{"bad json", "{", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			if err := run([]string{"status", "--url", server.URL}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
				t.Fatal("status unexpectedly succeeded")
			}
		})
	}
}

func TestQueueRevisionHeadersFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		etag   string
	}{
		{"missing etag", http.StatusOK, ""},
		{"backend failure", http.StatusServiceUnavailable, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("ETag", test.etag)
				w.WriteHeader(test.status)
			}))
			defer server.Close()
			if _, _, err := queueRevisionHeaders(server.Client(), server.URL, "secret"); err == nil {
				t.Fatal("lookup unexpectedly succeeded")
			}
		})
	}
}

func TestQueueRevisionHeadersSendsBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("ETag", `"1"`)
	}))
	defer server.Close()
	ifMatch, ifNoneMatch, err := queueRevisionHeaders(server.Client(), server.URL, "secret")
	if err != nil || ifMatch != `"1"` || ifNoneMatch != "" {
		t.Fatalf("ifMatch=%q ifNoneMatch=%q err=%v", ifMatch, ifNoneMatch, err)
	}
}

func TestReadObservedTopologyFindsConsumer(t *testing.T) {
	plan, err := topology.BuildPlan(*mustReadQueue(t))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/consumers"):
			_, _ = w.Write([]byte(`{"items":[{"name":"other"},{"name":"` + plan.Consumer.Name + `"}]}`))
		default:
			_, _ = w.Write([]byte(`{"name":"` + plan.Stream.Name + `"}`))
		}
	}))
	defer server.Close()
	observed, err := readObservedTopology(server.URL, "secret", plan)
	if err != nil || observed.Stream == nil || observed.Consumer == nil || observed.Consumer.Name != plan.Consumer.Name {
		t.Fatalf("observed=%#v err=%v", observed, err)
	}
}

func TestReadObservedTopologyRejectsBackendFailures(t *testing.T) {
	plan, err := topology.BuildPlan(*mustReadQueue(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		_, err := readObservedTopology(server.URL, "secret", plan)
		server.Close()
		if err == nil {
			t.Fatalf("status %d unexpectedly succeeded", status)
		}
	}
}

func TestReadErrorBodyFailure(t *testing.T) {
	if got := readErrorBody(failingReader{}); got != "unreadable response" {
		t.Fatalf("got=%q", got)
	}
}

func mustReadQueue(t *testing.T) *topology.Queue {
	t.Helper()
	queue, err := topology.ParseQueue(strings.NewReader(queueFile))
	if err != nil {
		t.Fatal(err)
	}
	return queue
}
