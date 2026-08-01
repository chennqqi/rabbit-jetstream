package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
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

type AuditBackend interface {
	RecordAudit(context.Context, jetstream.AuditEvent) (uint64, error)
	ListAudit(context.Context, int, int) (jetstream.AuditPage, error)
}

type AuthConfig struct {
	OperatorTokens []string
	AuditorTokens  []string
}

type Handler struct {
	client     Backend
	monitor    Monitor
	logger     *slog.Logger
	name       string
	version    string
	started    time.Time
	auth       AuthConfig
	controller ControllerMonitor
	metrics    *Metrics
	audit      AuditBackend
}

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

func NewWithControllerAuth(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, auth AuthConfig) http.Handler {
	return newHandler(client, logger, name, version, monitor, control, auth)
}

func newHandler(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, auth AuthConfig) http.Handler {
	audit, _ := client.(AuditBackend)
	auth.OperatorTokens = cleanTokens(auth.OperatorTokens)
	auth.AuditorTokens = cleanTokens(auth.AuditorTokens)
	h := &Handler{client: client, monitor: monitor, logger: logger, name: name, version: version, started: time.Now(), auth: auth, controller: control, metrics: NewMetrics(), audit: audit}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("GET /api/v1/info", h.info)
	mux.HandleFunc("GET /api/v1/cluster", h.cluster)
	mux.HandleFunc("GET /api/v1/nodes", h.nodes)
	mux.HandleFunc("GET /api/v1/streams", h.streams)
	mux.HandleFunc("GET /api/v1/streams/{stream}", h.stream)
	mux.HandleFunc("GET /api/v1/streams/{stream}/consumers", h.consumers)
	mux.HandleFunc("PUT /api/v1/queues/{queue}", h.applyQueue)
	mux.HandleFunc("DELETE /api/v1/queues/{queue}", h.deleteQueue)
	mux.HandleFunc("GET /api/v1/queues", h.queues)
	mux.HandleFunc("GET /api/v1/queues/{queue}", h.queue)
	mux.HandleFunc("GET /api/v1/controller", h.controllerStatus)
	mux.HandleFunc("GET /api/v1/audit", h.auditEvents)
	mux.HandleFunc("GET /metrics", h.prometheus)
	mux.Handle("GET /admin/", adminui.Handler())
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
	})
	return h.logging(h.instrument(mux))
}

func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	declaration, err := h.client.Declaration(ctx, r.PathValue("queue"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", declaration.KVRevision))
	writeJSON(w, http.StatusOK, declaration)
}

func (h *Handler) controllerStatus(w http.ResponseWriter, _ *http.Request) {
	if h.controller == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "controller_unavailable", "controller is not configured")
		return
	}
	writeJSON(w, http.StatusOK, h.controller.Status())
}

func (h *Handler) queues(w http.ResponseWriter, r *http.Request) {
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListDeclarations(ctx)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page(items, offset, limit))
}

func (h *Handler) deleteQueue(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
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
	if err != nil {
		status, code := backendErrorDetails(err)
		if !h.recordAuditOutcome(w, intent, "failed", status, code, "") {
			return
		}
		writeBackendError(w, err)
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
	defer r.Body.Close()
	queue, err := topology.ParseQueue(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_queue", err.Error())
		return
	}
	if queue.Metadata.Name != r.PathValue("queue") {
		writeAPIError(w, http.StatusConflict, "name_mismatch", "URL and document Queue names differ")
		return
	}
	plan, err := topology.BuildPlan(*queue)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_queue", err.Error())
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
	if err != nil {
		status, code := backendErrorDetails(err)
		if !h.recordAuditOutcome(w, intent, "failed", status, code, plan.Revision) {
			return
		}
		writeBackendError(w, err)
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
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", err.Error())
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

func (h *Handler) authorizeWrite(w http.ResponseWriter, r *http.Request) bool {
	return h.authorize(w, r, h.auth.OperatorTokens, "write_api_disabled", "write API")
}

func (h *Handler) authorizeAudit(w http.ResponseWriter, r *http.Request) bool {
	tokens := make([]string, 0, len(h.auth.OperatorTokens)+len(h.auth.AuditorTokens))
	tokens = append(tokens, h.auth.OperatorTokens...)
	tokens = append(tokens, h.auth.AuditorTokens...)
	return h.authorize(w, r, tokens, "audit_api_disabled", "audit API")
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, tokens []string, disabledCode, capability string) bool {
	if len(tokens) == 0 {
		writeAPIError(w, http.StatusNotFound, disabledCode, capability+" is disabled")
		return false
	}
	const prefix = "Bearer "
	provided := r.Header.Get("Authorization")
	if !strings.HasPrefix(provided, prefix) || !matchesToken(strings.TrimPrefix(provided, prefix), tokens) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
		return false
	}
	return true
}

func matchesToken(provided string, tokens []string) bool {
	providedDigest := sha256.Sum256([]byte(provided))
	matched := 0
	for _, token := range tokens {
		tokenDigest := sha256.Sum256([]byte(token))
		matched |= subtle.ConstantTimeCompare(providedDigest[:], tokenDigest[:])
	}
	return matched == 1
}

func tokenList(token string) []string {
	if token == "" {
		return nil
	}
	return []string{token}
}

func cleanTokens(tokens []string) []string {
	cleaned := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != "" {
			cleaned = append(cleaned, token)
		}
	}
	return cleaned
}

func (h *Handler) recordAuditIntent(w http.ResponseWriter, r *http.Request, action, resource, revision string, force bool) (jetstream.AuditEvent, bool) {
	if h.audit == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", "write rejected because audit backend is unavailable")
		return jetstream.AuditEvent{}, false
	}
	requestID := auditRequestID(r)
	w.Header().Set("X-Request-ID", requestID)
	event := jetstream.AuditEvent{ID: randomAuditID(), RequestID: requestID, Time: time.Now().UTC(), Phase: "intent", Action: action, ResourceKind: "Queue", ResourceName: resource, Actor: auditActor(r), ActorRole: "operator", SourceIP: remoteIP(r.RemoteAddr), Outcome: "attempted", Revision: revision, Force: force}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := h.audit.RecordAudit(ctx, event); err != nil {
		h.logger.Error("audit intent failed; mutation rejected", "request_id", requestID, "action", action, "queue", resource, "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", "write rejected because audit intent could not be persisted")
		return jetstream.AuditEvent{}, false
	}
	return event, true
}

func (h *Handler) recordAuditOutcome(w http.ResponseWriter, intent jetstream.AuditEvent, outcome string, status int, code, revision string) bool {
	event := intent
	event.ID = randomAuditID()
	event.IntentID = intent.ID
	event.Time = time.Now().UTC()
	event.Phase = "outcome"
	event.Outcome = outcome
	event.HTTPStatus = status
	event.ErrorCode = code
	if revision != "" {
		event.Revision = revision
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := h.audit.RecordAudit(ctx, event); err != nil {
		h.logger.Error("audit outcome failed after mutation attempt", "request_id", intent.RequestID, "intent_id", intent.ID, "outcome", outcome, "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "audit_unavailable", "mutation outcome could not be persisted; inspect resource state before retrying")
		return false
	}
	return true
}

func auditRequestID(r *http.Request) string {
	value := r.Header.Get("X-Request-ID")
	if value != "" && len(value) <= 128 && strings.IndexFunc(value, func(ch rune) bool {
		return !(ch == '-' || ch == '_' || ch == '.' || ch == ':' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z')
	}) == -1 {
		return value
	}
	return randomAuditID()
}

func randomAuditID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(value)
}

func auditActor(r *http.Request) string {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	digest := sha256.Sum256([]byte(token))
	return "token-sha256:" + hex.EncodeToString(digest[:])
}

func remoteIP(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return value
}

func preconditionRevision(value jetstream.ApplyPrecondition) string {
	if value.CreateOnly {
		return "create"
	}
	if value.ExpectedRevision != nil {
		return strconv.FormatUint(*value.ExpectedRevision, 10)
	}
	return ""
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
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) info(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	info, err := h.client.AccountInfo(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": h.name, "version": h.version, "uptime_seconds": int64(time.Since(h.started).Seconds()),
		"nats_url":  redact.URL(h.client.ServerURL()),
		"jetstream": map[string]any{"memory_used": info.MemoryUsed, "storage_used": info.StorageUsed, "streams": info.Streams, "consumers": info.Consumers},
	})
}

func (h *Handler) cluster(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	account, err := h.client.AccountInfo(ctx)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server_url": redact.URL(h.client.ServerURL()), "account": account})
}

func (h *Handler) streams(w http.ResponseWriter, r *http.Request) {
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListStreams(ctx)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page(items, offset, limit))
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.client.Stream(ctx, r.PathValue("stream"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) consumers(w http.ResponseWriter, r *http.Request) {
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.client.ListConsumers(ctx, r.PathValue("stream"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page(items, offset, limit))
}

func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		h.logger.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func pagination(r *http.Request) (int, int, error) {
	offset, err := queryInt(r, "offset", 0)
	if err != nil || offset < 0 {
		return 0, 0, errors.New("offset must be a non-negative integer")
	}
	limit, err := queryInt(r, "limit", 50)
	if err != nil || limit < 1 || limit > 200 {
		return 0, 0, errors.New("limit must be an integer between 1 and 200")
	}
	return offset, limit, nil
}

func queryInt(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func page[T any](items []T, offset, limit int) map[string]any {
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return map[string]any{"items": items[offset:end], "total": total, "offset": offset, "limit": limit}
}

func writeBackendError(w http.ResponseWriter, err error) {
	status, code := backendErrorDetails(err)
	writeAPIError(w, status, code, err.Error())
}

func backendErrorDetails(err error) (int, string) {
	if errors.Is(err, jetstream.ErrConflict) {
		return http.StatusConflict, "conflict"
	}
	if errors.Is(err, jetstream.ErrNotFound) {
		return http.StatusNotFound, "not_found"
	}
	return http.StatusServiceUnavailable, "jetstream_unavailable"
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
