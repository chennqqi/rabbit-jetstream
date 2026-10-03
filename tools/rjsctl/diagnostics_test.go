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

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
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

func TestCollectDiagnosticsRefusesOutputCreatedDuringCollection(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "raced.zip")
	created := false
	transport := diagnosticRoundTripper(func(request *http.Request) (*http.Response, error) {
		if !created {
			created = true
			if err := os.WriteFile(output, []byte("concurrent-owner-data"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: request}, nil
	})
	if err := collectDiagnostics(&http.Client{Transport: transport}, "http://example.test", output, time.Now()); err == nil {
		t.Fatal("publication replaced a concurrently created output")
	}
	value, err := os.ReadFile(output)
	if err != nil || string(value) != "concurrent-owner-data" {
		t.Fatal("concurrent owner's output was changed")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "raced.zip" {
		t.Fatal("temporary archive was not cleaned up")
	}
}

func TestCollectDiagnosticsOmitsMalformedJSONAndPreservesExactCounters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/nodes" {
			// Known JSON sources must not evade redaction through a wrong MIME type.
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"password":"must-not-be-archived"`))
			return
		}
		_, _ = w.Write([]byte(`{"pending":18446744073709551615,"revision":9007199254740993,"secret":"must-not-be-archived"}`))
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "safe.zip")
	if err := collectDiagnostics(server.Client(), server.URL, output, time.Now()); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := zipFiles(t, archive.File)
	if files["nodes.json"] != nil || len(files) != len(diagnosticEndpoints) {
		t.Fatal("invalid source was included or other sources were skipped")
	}
	for _, file := range files {
		if strings.Contains(string(readZipFile(t, file)), "must-not-be-archived") {
			t.Fatal("sensitive response leaked into archive")
		}
	}
	info := string(readZipFile(t, files["info.json"]))
	if !strings.Contains(info, "18446744073709551615") || !strings.Contains(info, "9007199254740993") {
		t.Fatal("diagnostic counters lost precision")
	}
	var manifest diagnosticManifest
	if err := json.Unmarshal(readZipFile(t, files["manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range manifest.Entries {
		if entry.Path == "/api/v1/nodes" {
			found = true
			if entry.StatusCode != 503 || entry.File != "" || entry.SHA256 != "" || entry.Size != 0 || entry.Error != "invalid JSON response omitted; redaction unavailable" {
				t.Fatal("omitted source manifest is misleading")
			}
		}
	}
	if !found {
		t.Fatal("omitted source missing from manifest")
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
	malformed := []byte("not-json")
	if got, err := redact.JSON(malformed); err == nil || got != nil {
		t.Fatal("malformed JSON must not be returned")
	}
	redacted, err := redact.JSON([]byte(`[{"password":"secret","urls":["https://user:pass@example.test"]}]`))
	if err != nil {
		t.Fatal(err)
	}
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
