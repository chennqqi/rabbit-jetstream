package api

import (
	"context"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func (h *Handler) queueSchema(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "schema_api_disabled", "Queue schema") {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "Queue schema accepts no query parameters")
		return
	}
	if !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/schema+json")
	w.Header().Set("ETag", topology.QueueSchemaETag())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(topology.QueueSchema())
}
