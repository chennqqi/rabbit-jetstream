package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/management/internal/diagnostics"
	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

const (
	diagnosticRequestSchema  = "rjs.diagnostics-request.v1"
	diagnosticProfile        = "metadata-v1"
	diagnosticManifestSchema = "rjs.diagnostics-manifest.v1"
	diagnosticInputLimit     = 4 << 20
	diagnosticOutputLimit    = 8 << 20
)

type diagnosticRequest struct {
	Schema  string `json:"schema"`
	Profile string `json:"profile"`
}

type metadataCollector struct {
	h        *Handler
	terminal jetstream.AuditEvent
}

func (c metadataCollector) Collect(ctx context.Context) (diagnostics.Result, error) {
	result, err := c.h.collectDiagnosticMetadata(ctx)
	outcome, code := "completed", ""
	if err != nil {
		outcome, code = "failed", "collection_failed"
	}
	if result.Partial && err == nil {
		outcome = "partial"
	}
	if !c.h.recordDiagnosticAudit(c.terminal, "outcome", outcome, code, 0) {
		return diagnostics.Result{}, errors.New("persist terminal diagnostic audit event")
	}
	return result, err
}

func (h *Handler) createDiagnosticJob(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, map[string]bool{"operator": true}, "diagnostics_api_disabled", "diagnostics API") {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "diagnostic creation accepts no query parameters")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	var request diagnosticRequest
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Schema != diagnosticRequestSchema || request.Profile != diagnosticProfile {
		writeAPIError(w, 400, "invalid_diagnostic_request", "exact diagnostics request schema and metadata-v1 profile required")
		return
	}
	principal := r.Context().Value(principalKey{}).(identity.Principal)
	jobID := randomAuditID()
	intent := h.newDiagnosticAudit(r, "diagnostics.create", jobID)
	if !h.recordDiagnosticAudit(intent, "intent", "attempted", "", 0) {
		writeAPIError(w, 503, "audit_unavailable", "diagnostic collection rejected because audit intent could not be persisted")
		return
	}
	job, err := h.diagnostics.StartWithID(principal.Actor, jobID, metadataCollector{h: h, terminal: intent})
	if err != nil {
		code, status := "diagnostics_capacity", http.StatusTooManyRequests
		if errors.Is(err, diagnostics.ErrClosed) {
			code, status = "diagnostics_unavailable", http.StatusServiceUnavailable
		}
		_ = h.recordDiagnosticAudit(intent, "outcome", "rejected", code, status)
		writeAPIError(w, status, code, "diagnostic collection is temporarily unavailable")
		return
	}
	w.Header().Set("Location", "/api/v1/diagnostics/jobs/"+job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (h *Handler) diagnosticJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authorizeDiagnosticOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "diagnostic status accepts no query parameters")
		return
	}
	job, err := h.diagnostics.Get(principal.Actor, r.PathValue("id"))
	if err != nil {
		writeAPIError(w, 404, "diagnostic_job_not_found", "diagnostic job not found")
		return
	}
	writeJSON(w, 200, job)
}

func (h *Handler) cancelDiagnosticJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authorizeDiagnosticOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "diagnostic cancellation accepts no query parameters")
		return
	}
	id := r.PathValue("id")
	intent := h.newDiagnosticAudit(r, "diagnostics.cancel", id)
	if !h.recordDiagnosticAudit(intent, "intent", "attempted", "", 0) {
		writeAPIError(w, 503, "audit_unavailable", "cancellation rejected because audit intent could not be persisted")
		return
	}
	job, err := h.diagnostics.Cancel(principal.Actor, id)
	if err != nil {
		_ = h.recordDiagnosticAudit(intent, "outcome", "rejected", "diagnostic_job_not_found", 404)
		writeAPIError(w, 404, "diagnostic_job_not_found", "diagnostic job not found")
		return
	}
	if !h.recordDiagnosticAudit(intent, "outcome", "accepted", "", 200) {
		writeAPIError(w, 503, "audit_unavailable", "cancellation occurred but its outcome could not be persisted")
		return
	}
	writeJSON(w, 200, job)
}

func (h *Handler) downloadDiagnosticJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authorizeDiagnosticOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || r.Header.Get("Range") != "" {
		writeAPIError(w, 400, "invalid_download_request", "query and range requests are not supported")
		return
	}
	id := r.PathValue("id")
	archive, err := h.diagnostics.Download(principal.Actor, id)
	if errors.Is(err, diagnostics.ErrNotFound) {
		writeAPIError(w, 404, "diagnostic_job_not_found", "diagnostic job not found")
		return
	}
	if err != nil {
		writeAPIError(w, 409, "diagnostic_not_ready", "diagnostic bundle is not available")
		return
	}
	intent := h.newDiagnosticAudit(r, "diagnostics.download", id)
	if !h.recordDiagnosticAudit(intent, "intent", "attempted", "", 0) || !h.recordDiagnosticAudit(intent, "outcome", "authorized", "", 200) {
		writeAPIError(w, 503, "audit_unavailable", "download rejected because authorization audit could not be persisted")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rabbit-jetstream-diagnostics-%s.zip"`, id))
	w.Header().Set("Content-Length", fmt.Sprint(len(archive)))
	w.WriteHeader(200)
	_, writeErr := w.Write(archive)
	completed := h.recordDiagnosticAudit(intent, "delivery", map[bool]string{true: "write_failed", false: "server_write_completed"}[writeErr != nil], map[bool]string{true: "response_write_failed", false: ""}[writeErr != nil], 200)
	if !completed && h.logger != nil {
		h.logger.Error("diagnostic delivery audit failed after response started", "job_id", id)
	}
}

func (h *Handler) authorizeDiagnosticOwner(w http.ResponseWriter, r *http.Request) (identity.Principal, bool) {
	if !h.authorize(w, r, map[string]bool{"operator": true}, "diagnostics_api_disabled", "diagnostics API") {
		return identity.Principal{}, false
	}
	principal := r.Context().Value(principalKey{}).(identity.Principal)
	if principal.Actor == "" {
		writeAPIError(w, 403, "forbidden", "authenticated identity is not suitable for job ownership")
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *Handler) newDiagnosticAudit(r *http.Request, action, id string) jetstream.AuditEvent {
	requestID := auditRequestID(r)
	principal, _ := r.Context().Value(principalKey{}).(identity.Principal)
	return jetstream.AuditEvent{ID: randomAuditID(), RequestID: requestID, Time: time.Now().UTC(), Action: action, ResourceKind: "DiagnosticJob", ResourceName: id, Actor: principal.Actor, ActorRole: principal.Role, SourceIP: remoteIP(r.RemoteAddr)}
}

func (h *Handler) recordDiagnosticAudit(base jetstream.AuditEvent, phase, outcome, code string, status int) bool {
	if h.audit == nil {
		return false
	}
	event := base
	event.Time = time.Now().UTC()
	event.Phase = phase
	event.Outcome = outcome
	event.ErrorCode = code
	event.HTTPStatus = status
	if phase != "intent" {
		event.ID = randomAuditID()
		event.IntentID = base.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := h.audit.RecordAudit(ctx, event); err != nil {
		if h.logger != nil {
			h.logger.Error("diagnostic audit failed", "action", event.Action, "phase", phase, "error_kind", "audit_unavailable")
		}
		return false
	}
	return true
}

func (h *Handler) collectDiagnosticMetadata(ctx context.Context) (diagnostics.Result, error) {
	now := time.Now().UTC()
	manifest := diagnostics.Manifest{Schema: diagnosticManifestSchema, GeneratedAt: now, Entries: []diagnostics.ManifestEntry{}}
	files := map[string][]byte{}
	partial := false
	add := func(source, file string, value any, err error) {
		entry := diagnostics.ManifestEntry{Source: source, CollectedAt: time.Now().UTC()}
		if err != nil {
			entry.Error = "source unavailable"
			partial = true
			manifest.Entries = append(manifest.Entries, entry)
			return
		}
		raw, marshalErr := json.MarshalIndent(value, "", "  ")
		if marshalErr == nil {
			raw = append(raw, '\n')
			raw, marshalErr = redact.JSON(raw)
		}
		if marshalErr != nil || len(raw) > diagnosticInputLimit {
			entry.Error = "source omitted because safe bounded encoding failed"
			partial = true
			manifest.Entries = append(manifest.Entries, entry)
			return
		}
		sum := sha256.Sum256(raw)
		entry.File, entry.Size, entry.SHA256 = file, len(raw), hex.EncodeToString(sum[:])
		files[file] = raw
		manifest.Entries = append(manifest.Entries, entry)
	}
	info, _ := debug.ReadBuildInfo()
	add("management-build", "management-build.json", buildInfo(h.version, h.console.RuntimeRevision, h.console.RuntimeClean, info), nil)
	add("server-capabilities", "server-capabilities.json", h.capabilitiesContract(), nil)
	account, err := h.client.AccountInfo(ctx)
	add("jetstream-account", "jetstream-account.json", map[string]any{"serverUrl": redact.URL(h.client.ServerURL()), "account": account}, err)
	if h.controller == nil {
		add("controller", "controller.json", nil, errors.New("unavailable"))
	} else {
		add("controller", "controller.json", h.controller.Status(), nil)
	}
	if h.monitor == nil {
		add("nodes", "nodes.json", nil, errors.New("unavailable"))
	} else {
		add("nodes", "nodes.json", diagnosticNodes(h.monitor.Nodes(ctx)), nil)
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return diagnostics.Result{}, err
	}
	manifestJSON = append(manifestJSON, '\n')
	files["manifest.json"] = manifestJSON
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	order := []string{"management-build.json", "server-capabilities.json", "jetstream-account.json", "controller.json", "nodes.json", "manifest.json"}
	for _, name := range order {
		body, exists := files[name]
		if !exists {
			continue
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		header.SetModTime(time.Unix(0, 0).UTC())
		writer, createErr := archive.CreateHeader(header)
		if createErr != nil {
			return diagnostics.Result{}, createErr
		}
		if _, createErr = writer.Write(body); createErr != nil {
			return diagnostics.Result{}, createErr
		}
		if buffer.Len() > diagnosticOutputLimit {
			return diagnostics.Result{}, errors.New("diagnostic archive exceeds limit")
		}
	}
	if err = archive.Close(); err != nil {
		return diagnostics.Result{}, err
	}
	if buffer.Len() > diagnosticOutputLimit {
		return diagnostics.Result{}, errors.New("diagnostic archive exceeds limit")
	}
	return diagnostics.Result{Archive: buffer.Bytes(), Partial: partial, Manifest: manifest}, nil
}

func diagnosticNodes(snapshot monitoring.Snapshot) monitoring.Snapshot {
	result := snapshot
	result.Nodes = append([]monitoring.Node(nil), snapshot.Nodes...)
	for index := range result.Nodes {
		result.Nodes[index].Endpoint = redact.URL(result.Nodes[index].Endpoint)
		if len(result.Nodes[index].Errors) > 0 {
			result.Nodes[index].Errors = []string{"one or more monitoring sources unavailable"}
		}
		result.Nodes[index].Sources = cloneDiagnosticSources(result.Nodes[index].Sources)
		result.Nodes[index].ConnectedPeers = append([]string(nil), result.Nodes[index].ConnectedPeers...)
	}
	return result
}

func cloneDiagnosticSources(input map[string]monitoring.SourceObservation) map[string]monitoring.SourceObservation {
	if input == nil {
		return nil
	}
	result := make(map[string]monitoring.SourceObservation, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

var _ diagnostics.Collector = metadataCollector{}
