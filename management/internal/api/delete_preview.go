package api

import (
	"context"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func (h *Handler) previewDeleteQueue(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
		return
	}
	if !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	name := r.PathValue("queue")
	if !topology.ValidQueueName(name) || r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "valid Queue name and no query parameters required")
		return
	}
	precondition, err := applyPrecondition(r)
	if err != nil || precondition.CreateOnly || precondition.ExpectedRevision == nil {
		writeAPIError(w, http.StatusPreconditionRequired, "precondition_required", "original declaration If-Match required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	preview, err := h.client.PreviewDelete(ctx, name, precondition)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	w.Header().Set("ETag", preview.BaseRevision)
	writeJSON(w, http.StatusOK, preview)
}
