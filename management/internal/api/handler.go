package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

type Handler struct {
	client     Backend
	monitor    Monitor
	logger     *slog.Logger
	name       string
	version    string
	started    time.Time
	adminToken string
	controller ControllerMonitor
	metrics    *Metrics
}

func New(client Backend, logger *slog.Logger, name, version string, monitor Monitor, adminTokens ...string) http.Handler {
	var adminToken string
	if len(adminTokens) > 0 {
		adminToken = adminTokens[0]
	}
	return newHandler(client, logger, name, version, monitor, nil, adminToken)
}

func NewWithController(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, adminToken string) http.Handler {
	return newHandler(client, logger, name, version, monitor, control, adminToken)
}

func newHandler(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, adminToken string) http.Handler {
	h := &Handler{client: client, monitor: monitor, logger: logger, name: name, version: version, started: time.Now(), adminToken: adminToken, controller: control, metrics: NewMetrics()}
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.client.DeleteQueueConditional(ctx, name, force, precondition)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	status := http.StatusOK
	if result.Blocked {
		status = http.StatusConflict
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.client.ApplyConditional(ctx, plan, precondition)
	if err != nil {
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
	writeJSON(w, status, result)
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
	if h.adminToken == "" {
		writeAPIError(w, http.StatusNotFound, "write_api_disabled", "write API is disabled")
		return false
	}
	want := "Bearer " + h.adminToken
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(want)) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
		return false
	}
	return true
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
	if errors.Is(err, jetstream.ErrConflict) {
		writeAPIError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	if errors.Is(err, jetstream.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeAPIError(w, http.StatusServiceUnavailable, "jetstream_unavailable", err.Error())
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
