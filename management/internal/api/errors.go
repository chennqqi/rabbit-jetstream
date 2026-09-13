package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) writeBackendError(w http.ResponseWriter, err error) {
	status, code := backendErrorDetails(err)
	h.logBackendError("JetStream request failed", err)
	writeAPIError(w, status, code, backendPublicMessage(code))
}

func (h *Handler) logBackendError(message string, err error, attributes ...any) {
	if h.logger != nil {
		_, code := backendErrorDetails(err)
		h.logger.Warn(message, append(attributes, "error_kind", code)...)
	}
}

func backendPublicMessage(code string) string {
	switch code {
	case "dlq_dependency_cycle":
		return "DLQ dependency cycle"
	case "conflict":
		return "resource conflict"
	case "not_found":
		return "resource not found"
	default:
		return "JetStream is unavailable"
	}
}

func backendErrorDetails(err error) (int, string) {
	if errors.Is(err, jetstream.ErrDeadLetterCycle) {
		return http.StatusBadRequest, "dlq_dependency_cycle"
	}
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
