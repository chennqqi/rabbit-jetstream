package api

import (
	"context"
	"net/http"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

type HistoryBackend interface {
	QueryRange(context.Context, prometheusbackend.MetricID, string, prometheusbackend.Window, time.Time) (prometheusbackend.Range, error)
}

func (h *Handler) metricHistory(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "history_api_disabled", "metric history API") {
		return
	}
	if h.history == nil {
		writeAPIError(w, http.StatusNotFound, "history_api_disabled", "metric history backend is not configured")
		return
	}
	query := r.URL.Query()
	for key := range query {
		if key != "metric" && key != "window" && key != "queue" {
			writeAPIError(w, http.StatusBadRequest, "invalid_history_query", "unsupported history query parameter")
			return
		}
		if len(query[key]) != 1 {
			writeAPIError(w, http.StatusBadRequest, "invalid_history_query", "history parameters must not repeat")
			return
		}
	}
	metric, window, queue := prometheusbackend.MetricID(query.Get("metric")), prometheusbackend.Window(query.Get("window")), query.Get("queue")
	if metric == "" || window == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_history_query", "metric and window are required")
		return
	}
	if err := prometheusbackend.ValidateQuery(metric, queue, window); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_history_query", "unsupported metric, window, or Queue identity")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	result, err := h.history.QueryRange(ctx, metric, queue, window, time.Now().UTC())
	if err != nil {
		h.logBackendError("metric history query failed", err)
		writeAPIError(w, http.StatusServiceUnavailable, "history_unavailable", "metric history is unavailable or incompatible")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
