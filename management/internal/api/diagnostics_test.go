package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/diagnostics"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

func TestDiagnosticJobAPIIsOperatorOwnedAuditedAndBounded(t *testing.T) {
	backend := &fakeBackend{account: jetstream.Account{Streams: 2, Consumers: 3}, serverURL: "nats://user:secret@nats-1:4222"}
	nodes := monitoring.Snapshot{Nodes: []monitoring.Node{{Endpoint: "http://monitor:secret@node:8222", Errors: []string{"Get http://monitor:secret@node:8222: private failure"}}}}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "candidate", fakeMonitor{snapshot: nodes}, fakeController{status: controller.Status{}}, AuthConfig{OperatorTokens: []string{"alice", "bob"}, AuditorTokens: []string{"audit"}})
	if closer, ok := handler.(interface{ Close() }); ok {
		defer closer.Close()
	}

	for _, test := range []struct {
		token, body string
		status      int
	}{
		{"", `{"schema":"rjs.diagnostics-request.v1","profile":"metadata-v1"}`, 401},
		{"audit", `{"schema":"rjs.diagnostics-request.v1","profile":"metadata-v1"}`, 403},
		{"alice", `{"schema":"wrong","profile":"metadata-v1"}`, 400},
		{"alice", `{"schema":"rjs.diagnostics-request.v1","profile":"metadata-v1","url":"https://evil"}`, 400},
	} {
		recorder := performDiagnosticRequest(t, handler, http.MethodPost, "/api/v1/diagnostics/jobs", test.token, test.body)
		if recorder.Code != test.status {
			t.Fatalf("token=%q status=%d body=%s", test.token, recorder.Code, recorder.Body.String())
		}
	}

	created := performDiagnosticRequest(t, handler, http.MethodPost, "/api/v1/diagnostics/jobs", "alice", `{"schema":"rjs.diagnostics-request.v1","profile":"metadata-v1"}`)
	if created.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", created.Code, created.Body.String())
	}
	var job diagnostics.Job
	if err := json.Unmarshal(created.Body.Bytes(), &job); err != nil || len(job.ID) != 32 || job.State != diagnostics.StateCollecting {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	if created.Header().Get("Location") != "/api/v1/diagnostics/jobs/"+job.ID {
		t.Fatalf("location=%q", created.Header().Get("Location"))
	}

	other := performDiagnosticRequest(t, handler, http.MethodGet, "/api/v1/diagnostics/jobs/"+job.ID, "bob", "")
	if other.Code != 404 {
		t.Fatalf("other owner status=%d body=%s", other.Code, other.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := performDiagnosticRequest(t, handler, http.MethodGet, "/api/v1/diagnostics/jobs/"+job.ID, "alice", "")
		if status.Code != 200 {
			t.Fatalf("status=%d body=%s", status.Code, status.Body.String())
		}
		if err := json.Unmarshal(status.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.State == diagnostics.StateReady || job.State == diagnostics.StatePartial {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if job.State != diagnostics.StateReady {
		t.Fatalf("job=%#v", job)
	}
	if job.Manifest == nil || job.Manifest.Schema != diagnosticManifestSchema || len(job.Manifest.Entries) != 5 {
		t.Fatalf("manifest=%#v", job.Manifest)
	}

	download := performDiagnosticRequest(t, handler, http.MethodGet, "/api/v1/diagnostics/jobs/"+job.ID+"/download", "alice", "")
	if download.Code != 200 || download.Header().Get("Content-Type") != "application/zip" || download.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", download.Code, download.Header(), download.Body.String())
	}
	reader, err := zip.NewReader(bytes.NewReader(download.Body.Bytes()), int64(download.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
		if file.Mode().Perm() != 0600 {
			t.Fatalf("mode %s=%o", file.Name, file.Mode().Perm())
		}
		entry, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(entry)
		_ = entry.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(content), "secret") || strings.Contains(string(content), "private failure") {
			t.Fatalf("%s leaked sensitive fixture: %s", file.Name, content)
		}
	}
	sort.Strings(names)
	want := []string{"controller.json", "jetstream-account.json", "management-build.json", "manifest.json", "nodes.json", "server-capabilities.json"}
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("files=%v", names)
	}

	rangeRequest := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/jobs/"+job.ID+"/download", nil)
	rangeRequest.Header.Set("Authorization", "Bearer alice")
	rangeRequest.Header.Set("Range", "bytes=0-1")
	rangeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(rangeRecorder, rangeRequest)
	if rangeRecorder.Code != 400 {
		t.Fatalf("range status=%d", rangeRecorder.Code)
	}

	actions := map[string]int{}
	for _, event := range backend.auditEvents {
		actions[event.Action]++
	}
	if actions["diagnostics.create"] < 2 || actions["diagnostics.download"] < 3 {
		t.Fatalf("audit actions=%v events=%#v", actions, backend.auditEvents)
	}
}

func TestDiagnosticCreationFailsClosedWhenAuditUnavailable(t *testing.T) {
	backend := &fakeBackend{auditErr: io.ErrClosedPipe}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}})
	if closer, ok := handler.(interface{ Close() }); ok {
		defer closer.Close()
	}
	response := performDiagnosticRequest(t, handler, http.MethodPost, "/api/v1/diagnostics/jobs", "operator", `{"schema":"rjs.diagnostics-request.v1","profile":"metadata-v1"}`)
	if response.Code != 503 || !strings.Contains(response.Body.String(), "audit_unavailable") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func performDiagnosticRequest(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
