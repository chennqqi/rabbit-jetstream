package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/diagnostics"
	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type Backend interface {
	Ready(context.Context) error
	AccountInfo(context.Context) (*jetstream.Account, error)
	ServerURL() string
	ListStreams(context.Context) ([]jetstream.Stream, error)
	Stream(context.Context, string) (*jetstream.Stream, error)
	ListConsumers(context.Context, string) ([]jetstream.Consumer, error)
	Consumer(context.Context, string, string) (*jetstream.Consumer, error)
	QueueConsumers(context.Context, string) (*jetstream.QueueConsumerCollection, error)
	Preview(context.Context, topology.Plan, jetstream.ApplyPrecondition) (*jetstream.PlanPreview, error)
	PreviewDelete(context.Context, string, jetstream.ApplyPrecondition) (*jetstream.DeletePreview, error)
	Apply(context.Context, topology.Plan) (topology.ReconcileResult, error)
	ApplyConditional(context.Context, topology.Plan, jetstream.ApplyPrecondition) (topology.ReconcileResult, error)
	DeleteQueue(context.Context, string, bool) (topology.DeleteResult, error)
	DeleteQueueConditional(context.Context, string, bool, jetstream.ApplyPrecondition) (topology.DeleteResult, error)
	ListDeclarations(context.Context) ([]topology.Declaration, error)
	Declaration(context.Context, string) (*topology.Declaration, error)
}

type Monitor interface {
	Nodes(context.Context) monitoring.Snapshot
}

type ControllerMonitor interface{ Status() controller.Status }
type tenantControllerMonitor interface {
	StatusFor(context.Context) controller.Status
}
type tenantServerURL interface{ ServerURLFor(context.Context) string }

type AuditBackend interface {
	RecordAudit(context.Context, jetstream.AuditEvent) (uint64, error)
	ListAudit(context.Context, int, int) (jetstream.AuditPage, error)
}

type AuthConfig struct {
	RequireReadAuth bool
	OperatorTokens  []string
	AuditorTokens   []string
	Local           identity.PasswordIssuer
	LocalVerifier   identity.Verifier
	OIDC            identity.Verifier
	BrowserOIDC     BrowserOIDC
	DefaultTenant   string
	TenantIDs       []string
	LocalAccounts   identity.LocalAccountManager
	// TrustedProxyHops is the number of reverse proxies between the
	// management service and the network edge. Zero (default) means clients
	// connect directly and the socket peer address is authoritative.
	TrustedProxyHops int
}

type Handler struct {
	client           Backend
	monitor          Monitor
	logger           *slog.Logger
	name             string
	version          string
	started          time.Time
	auth             AuthConfig
	controller       ControllerMonitor
	metrics          *Metrics
	audit            AuditBackend
	console          ConsoleConfig
	consumerIndexes  sync.Map
	diagnostics      *diagnostics.Store
	history          HistoryBackend
	alerts           AlertBackend
	events           *eventHub
	eventContext     context.Context
	eventCancel      context.CancelFunc
	eventOnce        sync.Once
	loginLimiter     *loginLimiter
	trustedProxyHops int
}

type managedHandler struct {
	http.Handler
	diagnostics *diagnostics.Store
	cancel      context.CancelFunc
}

func (h *managedHandler) Close() { h.cancel(); h.diagnostics.Close() }

func New(client Backend, logger *slog.Logger, name, version string, monitor Monitor, adminTokens ...string) http.Handler {
	var adminToken string
	if len(adminTokens) > 0 {
		adminToken = adminTokens[0]
	}
	return newHandler(client, logger, name, version, monitor, nil, AuthConfig{OperatorTokens: tokenList(adminToken)})
}

func NewWithController(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, adminToken string) http.Handler {
	return NewWithControllerAuth(client, logger, name, version, monitor, control, AuthConfig{OperatorTokens: tokenList(adminToken)})
}

func NewWithControllerAuth(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, auth AuthConfig, console ...ConsoleConfig) http.Handler {
	return newHandler(client, logger, name, version, monitor, control, auth, console...)
}

func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	declaration, err := h.client.Declaration(ctx, r.PathValue("queue"))
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", declaration.KVRevision))
	document, documentErr := topology.QueueDocument(declaration.Plan)
	response := struct {
		*topology.Declaration
		Document      *topology.Queue `json:"document"`
		DocumentError string          `json:"document_error,omitempty"`
	}{Declaration: declaration, Document: document}
	if documentErr != nil {
		response.DocumentError = documentErr.Error()
	}
	if declaration.Queue != declaration.Plan.Queue || declaration.Revision != declaration.Plan.Revision {
		response.Document = nil
		response.DocumentError = "declaration identity or content revision does not match its plan"
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) controllerStatus(w http.ResponseWriter, r *http.Request) {
	if h.controller == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "controller_unavailable", "controller is not configured")
		return
	}
	writeJSON(w, http.StatusOK, h.controllerStatusFor(r.Context()))
}

func (h *Handler) queues(w http.ResponseWriter, r *http.Request) {
	query, err := parseListQuery(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListDeclarations(ctx)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	items = queryList(items, query, func(item topology.Declaration) string { return item.Queue })
	streams, streamErr := h.client.ListStreams(ctx)
	observed := observeQueueList(items, streams, streamErr)
	writeJSON(w, http.StatusOK, page(observed, offset, limit))
}

func (h *Handler) deleteQueue(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
		return
	}
	if !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	name := r.PathValue("queue")
	if !topology.ValidQueueName(name) {
		writeAPIError(w, http.StatusBadRequest, "invalid_queue_name", "invalid Queue name")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-RJS-Confirm-Queue")), []byte(name)) != 1 {
		writeAPIError(w, http.StatusBadRequest, "confirmation_required", "X-RJS-Confirm-Queue must exactly match the Queue name")
		return
	}
	force, err := strconv.ParseBool(r.URL.Query().Get("force"))
	if err != nil && r.URL.Query().Get("force") != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_force", "force must be true or false")
		return
	}
	precondition, err := applyPrecondition(r)
	if err != nil {
		writeAPIError(w, http.StatusPreconditionRequired, "precondition_required", err.Error())
		return
	}
	intent, ok := h.recordAuditIntent(w, r, "queue.delete", name, preconditionRevision(precondition), force)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.client.DeleteQueueConditional(ctx, name, force, precondition)
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("rjs.queue.name", name), attribute.Bool("rjs.queue.force", force))
	if err != nil {
		status, code := backendErrorDetails(err)
		if !h.recordAuditOutcome(w, intent, "failed", status, code, "") {
			return
		}
		h.logBackendError("Queue deletion failed", err, "queue", name, "request_id", intent.RequestID)
		writeMutationError(w, status, code, backendPublicMessage(code), "backend", intent.ID)
		return
	}
	status := http.StatusOK
	if result.Blocked {
		status = http.StatusConflict
	}
	outcome := "succeeded"
	if result.Blocked {
		outcome = "blocked"
	}
	if !h.recordAuditOutcome(w, intent, outcome, status, "", "") {
		return
	}
	writeJSON(w, status, result)
}

func (h *Handler) applyQueue(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
		return
	}
	if !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	defer r.Body.Close()
	queue, err := topology.ParseQueue(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeQueueValidationError(w, err)
		return
	}
	if queue.Metadata.Name != r.PathValue("queue") {
		writeAPIError(w, http.StatusConflict, "name_mismatch", "URL and document Queue names differ")
		return
	}
	plan, err := topology.BuildPlan(*queue)
	if err != nil {
		writeQueueValidationError(w, err)
		return
	}
	precondition, err := applyPrecondition(r)
	if err != nil {
		writeAPIError(w, http.StatusPreconditionRequired, "precondition_required", err.Error())
		return
	}
	intent, ok := h.recordAuditIntent(w, r, "queue.apply", plan.Queue, preconditionRevision(precondition), false)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.client.ApplyConditional(ctx, plan, precondition)
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("rjs.queue.name", plan.Queue), attribute.String("rjs.queue.revision", plan.Revision))
	if err != nil {
		status, code := backendErrorDetails(err)
		if !h.recordAuditOutcome(w, intent, "failed", status, code, plan.Revision) {
			return
		}
		h.logBackendError("Queue apply failed", err, "queue", plan.Queue, "request_id", intent.RequestID)
		writeMutationError(w, status, code, backendPublicMessage(code), "backend", intent.ID)
		return
	}
	if declaration, declarationErr := h.client.Declaration(ctx, plan.Queue); declarationErr == nil {
		w.Header().Set("ETag", fmt.Sprintf("\"%d\"", declaration.KVRevision))
	}
	status := http.StatusOK
	if result.Blocked {
		status = http.StatusConflict
	}
	outcome := "succeeded"
	if result.Blocked {
		outcome = "blocked"
	}
	if !h.recordAuditOutcome(w, intent, outcome, status, "", result.Revision) {
		return
	}
	writeJSON(w, status, result)
}

func (h *Handler) auditEvents(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAudit(w, r) {
		return
	}
	if h.audit == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit backend is not configured")
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.audit.ListAudit(ctx, offset, limit)
	if err != nil {
		h.logBackendError("audit read failed", err)
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit data is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func applyPrecondition(r *http.Request) (jetstream.ApplyPrecondition, error) {
	match, none := r.Header.Get("If-Match"), r.Header.Get("If-None-Match")
	if none == "*" && match == "" {
		return jetstream.ApplyPrecondition{CreateOnly: true}, nil
	}
	if match != "" && none == "" {
		value := strings.Trim(match, "\"")
		revision, err := strconv.ParseUint(value, 10, 64)
		if err == nil && revision > 0 {
			return jetstream.ApplyPrecondition{ExpectedRevision: &revision}, nil
		}
	}
	return jetstream.ApplyPrecondition{}, errors.New("use If-None-Match: * to create or If-Match with the current KV revision to update")
}

func (h *Handler) nodes(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "monitoring_unavailable", "NATS monitoring is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, h.monitor.Nodes(ctx))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.client.Ready(ctx); err != nil {
		h.logBackendError("readiness check failed", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) info(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	info, err := h.client.AccountInfo(ctx)
	if err != nil {
		h.logBackendError("account info read failed", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "JetStream is unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": h.name, "version": h.version, "uptime_seconds": int64(time.Since(h.started).Seconds()),
		"nats_url":  redact.URL(h.serverURL(r.Context())),
		"jetstream": map[string]any{"memory_used": info.MemoryUsed, "storage_used": info.StorageUsed, "streams": info.Streams, "consumers": info.Consumers},
	})
}

func (h *Handler) cluster(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	account, err := h.client.AccountInfo(ctx)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server_url": redact.URL(h.serverURL(r.Context())), "account": account})
}

func (h *Handler) serverURL(ctx context.Context) string {
	if routed, ok := h.client.(tenantServerURL); ok {
		return routed.ServerURLFor(ctx)
	}
	return h.client.ServerURL()
}

func (h *Handler) controllerStatusFor(ctx context.Context) controller.Status {
	if routed, ok := h.controller.(tenantControllerMonitor); ok {
		return routed.StatusFor(ctx)
	}
	return h.controller.Status()
}

func (h *Handler) streams(w http.ResponseWriter, r *http.Request) {
	query, err := parseListQuery(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListStreams(ctx)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	items = queryList(items, query, func(item jetstream.Stream) string { return item.Name })
	writeJSON(w, http.StatusOK, page(items, offset, limit))
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.client.Stream(ctx, r.PathValue("stream"))
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) consumer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.client.Consumer(ctx, r.PathValue("stream"), r.PathValue("consumer"))
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) consumers(w http.ResponseWriter, r *http.Request) {
	query, mode, err := parseStreamConsumerQuery(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListConsumers(ctx, r.PathValue("stream"))
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page(queryStreamConsumers(items, query, mode), offset, limit))
}
