package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type diagnosticRoundTripper func(*http.Request) (*http.Response, error)

func (f diagnosticRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCollectDiagnosticsCreatesRedactedVerifiableBundle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if r.URL.Path == "/metrics" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("rjs_up 1\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nats_url":"nats://user:password@nats:4222","admin_token":"top-secret","status":"ok"}`))
	}))
	defer server.Close()

	output := filepath.Join(t.TempDir(), "diagnostics.zip")
	now := time.Date(2026, 8, 1, 12, 30, 0, 0, time.FixedZone("test", 8*60*60))
	if err := collectDiagnostics(server.Client(), "http://user:secret@"+strings.TrimPrefix(server.URL, "http://"), output, now); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := zipFiles(t, archive.File)
	if len(files) != len(diagnosticEndpoints)+1 {
		t.Fatalf("files=%v", files)
	}
	info := readZipFile(t, files["info.json"])
	if strings.Contains(string(info), "top-secret") || strings.Contains(string(info), "password") || !strings.Contains(string(info), "[REDACTED]") || !strings.Contains(string(info), "nats://nats:4222") {
		t.Fatalf("diagnostic JSON was not safely redacted: %s", info)
	}
	var manifest diagnosticManifest
	if err := json.Unmarshal(readZipFile(t, files["manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BaseURL != server.URL || !manifest.GeneratedAt.Equal(now.UTC()) || manifest.Schema == "" {
		t.Fatalf("manifest=%#v", manifest)
	}
	for _, entry := range manifest.Entries {
		if entry.File == "" {
			continue
		}
		body := readZipFile(t, files[entry.File])
		digest := sha256.Sum256(body)
		if entry.SHA256 != hex.EncodeToString(digest[:]) || entry.Size != len(body) {
			t.Fatalf("invalid manifest entry %#v", entry)
		}
	}
	var readiness diagnosticEntry
	for _, entry := range manifest.Entries {
		if entry.Path == "/readyz" {
			readiness = entry
		}
	}
	if readiness.StatusCode != http.StatusServiceUnavailable || readiness.Error == "" || readiness.File == "" {
		t.Fatalf("readiness entry=%#v", readiness)
	}
}

func TestCollectDiagnosticsRefusesOverwrite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "existing.zip")
	if err := os.WriteFile(output, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := collectDiagnostics(http.DefaultClient, "http://127.0.0.1", output, time.Now()); err == nil {
		t.Fatal("existing bundle was overwritten")
	}
	value, err := os.ReadFile(output)
	if err != nil || string(value) != "keep" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}

func TestCollectDiagnosticsRecordsRequestAndSizeFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = w.Write(make([]byte, diagnosticResponseLimit+1))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	transport := diagnosticRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v1/nodes" {
			return nil, errors.New("query https://user:secret@example.test/nodes failed")
		}
		return http.DefaultTransport.RoundTrip(request)
	})
	output := filepath.Join(t.TempDir(), "partial.zip")
	if err := collectDiagnostics(&http.Client{Transport: transport}, server.URL, output, time.Now()); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := zipFiles(t, archive.File)
	var manifest diagnosticManifest
	if err := json.Unmarshal(readZipFile(t, files["manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Entries {
		switch entry.Path {
		case "/api/v1/nodes":
			if entry.Error == "" || strings.Contains(entry.Error, "secret") || entry.File != "" {
				t.Fatalf("node entry=%#v", entry)
			}
		case "/metrics":
			if !strings.Contains(entry.Error, "exceeds") || entry.File != "" {
				t.Fatalf("metrics entry=%#v", entry)
			}
		}
	}
}

func TestCollectDiagnosticsRejectsMissingOutput(t *testing.T) {
	if err := collectDiagnostics(http.DefaultClient, "http://127.0.0.1", "", time.Now()); err == nil {
		t.Fatal("empty output succeeded")
	}
	missing := filepath.Join(t.TempDir(), "missing", "bundle.zip")
	if err := collectDiagnostics(http.DefaultClient, "http://127.0.0.1", missing, time.Now()); err == nil {
		t.Fatal("missing directory succeeded")
	}
}

func TestDiagnosticsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "bundle.zip")
	var stdout strings.Builder
	if err := run([]string{"diagnostics", "collect", "--url", server.URL, "--output", output}, &stdout, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), output) {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"diagnostics"}, {"diagnostics", "unknown"}, {"diagnostics", "collect", "extra"}} {
		if err := run(args, io.Discard, io.Discard); err == nil {
			t.Fatalf("args=%v succeeded", args)
		}
	}
}

func TestRedactionHelpers(t *testing.T) {
	if got := publicURL("nats://user:pass@nats:4222/path"); got != "nats://nats:4222/path" {
		t.Fatalf("URL=%q", got)
	}
	if got := publicURL("plain value"); got != "plain value" {
		t.Fatalf("plain=%q", got)
	}
	if !sensitiveDiagnosticKey("apiCredential") || sensitiveDiagnosticKey("status") == true {
		t.Fatal("key classification mismatch")
	}
	malformed := []byte("not-json")
	if got := redactJSON(malformed); string(got) != string(malformed) {
		t.Fatalf("malformed=%q", got)
	}
	redacted := redactJSON([]byte(`[{"password":"secret","urls":["https://user:pass@example.test"]}]`))
	if strings.Contains(string(redacted), "secret") || strings.Contains(string(redacted), "user:pass") {
		t.Fatalf("redacted=%s", redacted)
	}
}

func zipFiles(t *testing.T, files []*zip.File) map[string]*zip.File {
	t.Helper()
	result := make(map[string]*zip.File, len(files))
	for _, file := range files {
		result[file.Name] = file
	}
	return result
}

func readZipFile(t *testing.T, file *zip.File) []byte {
	t.Helper()
	if file == nil {
		t.Fatal("ZIP entry is missing")
	}
	reader, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	value, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
