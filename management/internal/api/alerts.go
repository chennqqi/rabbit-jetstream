package api

import (
	"context"
	"net/http"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

type AlertBackend interface {
	AlertRules(context.Context, time.Time) (prometheusbackend.AlertSnapshot, error)
}

func (h *Handler) operationalAlerts(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "alerts_api_disabled", "operational alerts") {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "alerts accepts no query parameters")
		return
	}
	if h.alerts == nil {
		writeAPIError(w, http.StatusNotFound, "alerts_api_disabled", "alert backend is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	result, err := h.alerts.AlertRules(ctx, time.Now().UTC())
	if err != nil {
		h.logBackendError("operational alert query failed", err)
		writeAPIError(w, http.StatusServiceUnavailable, "alerts_unavailable", "operational alerts are unavailable or incompatible")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
