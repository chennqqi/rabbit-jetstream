package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
)

const diagnosticResponseLimit = 4 << 20

var diagnosticEndpoints = []struct {
	Path string
	File string
}{
	{"/healthz", "health.json"},
	{"/readyz", "readiness.json"},
	{"/api/v1/info", "info.json"},
	{"/api/v1/cluster", "cluster.json"},
	{"/api/v1/nodes", "nodes.json"},
	{"/api/v1/queues?limit=200", "queues.json"},
	{"/api/v1/streams?limit=200", "streams.json"},
	{"/api/v1/controller", "controller.json"},
	{"/metrics", "metrics.txt"},
}

type diagnosticManifest struct {
	Schema      string            `json:"schema"`
	GeneratedAt time.Time         `json:"generated_at"`
	BaseURL     string            `json:"base_url"`
	Entries     []diagnosticEntry `json:"entries"`
}

type diagnosticEntry struct {
	Path       string `json:"path"`
	File       string `json:"file,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Size       int    `json:"size,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Error      string `json:"error,omitempty"`
}

func collectDiagnostics(client *http.Client, baseURL, token, output string, now time.Time) error {
	if output == "" {
		return errors.New("diagnostics output path is required")
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("refusing to overwrite existing file %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect diagnostics output: %w", err)
	}
	directory := filepath.Dir(output)
	temporary, err := os.CreateTemp(directory, ".rjs-diagnostics-*.zip")
	if err != nil {
		return fmt.Errorf("create diagnostics bundle: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()

	archive := zip.NewWriter(temporary)
	manifest := diagnosticManifest{
		Schema:      "rabbit-jetstream.io/diagnostics/v1alpha1",
		GeneratedAt: now.UTC(),
		BaseURL:     publicURL(baseURL),
		Entries:     make([]diagnosticEntry, 0, len(diagnosticEndpoints)),
	}
	baseURL = strings.TrimRight(baseURL, "/")
	for _, endpoint := range diagnosticEndpoints {
		entry := diagnosticEntry{Path: endpoint.Path}
		request, requestErr := http.NewRequest(http.MethodGet, baseURL+endpoint.Path, nil)
		if requestErr == nil && token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		var response *http.Response
		if requestErr == nil {
			response, requestErr = client.Do(request)
		}
		if requestErr != nil {
			entry.Error = safeDiagnosticError(requestErr)
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, diagnosticResponseLimit+1))
		_ = response.Body.Close()
		entry.StatusCode = response.StatusCode
		if readErr != nil {
			entry.Error = "read response: " + safeDiagnosticError(readErr)
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		if len(body) > diagnosticResponseLimit {
			entry.Error = fmt.Sprintf("response exceeds %d bytes", diagnosticResponseLimit)
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		if strings.HasSuffix(endpoint.File, ".json") || strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "json") || json.Valid(body) {
			var redactErr error
			body, redactErr = redact.JSON(body)
			if redactErr != nil {
				entry.Error = "invalid JSON response omitted; redaction unavailable"
				manifest.Entries = append(manifest.Entries, entry)
				continue
			}
		}
		if err := writeDiagnosticFile(archive, endpoint.File, body); err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		entry.File = endpoint.File
		entry.Size = len(body)
		entry.SHA256 = hex.EncodeToString(digest[:])
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			entry.Error = response.Status
		}
		manifest.Entries = append(manifest.Entries, entry)
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode diagnostics manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := writeDiagnosticFile(archive, "manifest.json", manifestJSON); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("finalize diagnostics archive: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync diagnostics archive: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close diagnostics archive: %w", err)
	}
	// Link publishes the completed file without replacing any existing path.
	// A preflight Stat plus Rename is racy: another owner may create output
	// during collection. Staging in the output directory keeps this on one
	// filesystem. Unsupported hard links fail safely; never fall back to rename.
	if err := os.Link(temporaryName, output); err != nil {
		return fmt.Errorf("publish diagnostics archive: %w", err)
	}
	return nil
}

func writeDiagnosticFile(archive *zip.Writer, name string, value []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(time.Unix(0, 0).UTC())
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create diagnostics entry %s: %w", name, err)
	}
	if _, err := writer.Write(value); err != nil {
		return fmt.Errorf("write diagnostics entry %s: %w", name, err)
	}
	return nil
}

func publicURL(value string) string {
	return redact.URL(value)
}

func safeDiagnosticError(err error) string {
	message := err.Error()
	for _, word := range strings.Fields(message) {
		if sanitized := publicURL(strings.Trim(word, `"'(),`)); sanitized != strings.Trim(word, `"'(),`) {
			message = strings.ReplaceAll(message, strings.Trim(word, `"'(),`), sanitized)
		}
	}
	return message
}

func defaultDiagnosticOutput(now time.Time) string {
	var name bytes.Buffer
	fmt.Fprintf(&name, "rabbit-jetstream-diagnostics-%s.zip", now.UTC().Format("20060102T150405Z"))
	return name.String()
}
