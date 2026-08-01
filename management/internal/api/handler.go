package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type Backend interface {
	Ready(context.Context) error
	AccountInfo(context.Context) (*jetstream.Account, error)
	ServerURL() string
	ListStreams(context.Context) ([]jetstream.Stream, error)
	Stream(context.Context, string) (*jetstream.Stream, error)
	ListConsumers(context.Context, string) ([]jetstream.Consumer, error)
}

type Handler struct {
	client  Backend
	logger  *slog.Logger
	name    string
	version string
	started time.Time
}

func New(client Backend, logger *slog.Logger, name, version string) http.Handler {
	h := &Handler{client: client, logger: logger, name: name, version: version, started: time.Now()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("GET /api/v1/info", h.info)
	mux.HandleFunc("GET /api/v1/cluster", h.cluster)
	mux.HandleFunc("GET /api/v1/streams", h.streams)
	mux.HandleFunc("GET /api/v1/streams/{stream}", h.stream)
	mux.HandleFunc("GET /api/v1/streams/{stream}/consumers", h.consumers)
	return h.logging(mux)
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
		"nats_url":  h.client.ServerURL(),
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
	writeJSON(w, http.StatusOK, map[string]any{"server_url": h.client.ServerURL(), "account": account})
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
	if errors.Is(err, jetstream.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeAPIError(w, http.StatusServiceUnavailable, "jetstream_unavailable", err.Error())
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
