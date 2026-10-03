package api

import (
	"context"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func (h *Handler) previewQueue(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
		return
	}
	if !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	defer r.Body.Close()
	queue, err := topology.ParseQueue(http.MaxBytesReader(w, r.Body, 1<<20))
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	preview, err := h.client.Preview(ctx, plan, precondition)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	// A blocked plan is a successful read of an unsafe proposed transition.
	// No audit intent/outcome is recorded because no apply has been attempted.
	writeJSON(w, http.StatusOK, preview)
}
