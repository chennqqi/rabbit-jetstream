package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

// exportQueue exports only one faithfully reconstructed declaration. It does
// not enumerate dependencies, read messages or mutate the saved declaration.
func (h *Handler) exportQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("queue")
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 64 || !topology.ValidQueueName(name) || len(name) > 256 {
		writeAPIError(w, 400, "invalid_query", "invalid export query or Queue name")
		return
	}
	for key, values := range query {
		if key != "include_labels" || len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			writeAPIError(w, 400, "invalid_query", "include_labels must be a single true or false value")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	declaration, err := h.client.Declaration(ctx, name)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	if declaration == nil || declaration.Queue != name || declaration.Plan.Queue != name || declaration.Revision != declaration.Plan.Revision {
		writeAPIError(w, 409, "export_declaration_unavailable", "declaration identity or revision does not match its plan")
		return
	}
	document, err := topology.QueueDocument(declaration.Plan)
	if err != nil {
		writeAPIError(w, 409, "export_declaration_unavailable", "declaration cannot be faithfully reconstructed")
		return
	}
	omitted := 0
	if query.Get("include_labels") != "true" {
		omitted = len(document.Metadata.Labels)
		document.Metadata.Labels = nil
	}
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil || len(body)+1 > 1<<20 {
		writeAPIError(w, 503, "export_limit", "export exceeds the one MiB response limit")
		return
	}
	if ctx.Err() != nil {
		writeAPIError(w, 503, "export_unavailable", "export canceled or deadline exceeded")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.queue.json\"", name))
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", declaration.KVRevision))
	w.Header().Set("X-RJS-Export-Omitted-Labels", strconv.Itoa(omitted))
	w.Header().Set("X-RJS-Export-Scope", "single-queue-declaration")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(body, '\n'))
}
