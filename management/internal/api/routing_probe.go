package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func (h *Handler) queueRoutingProbe(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("queue")
	if !topology.ValidQueueName(name) || len(name) > 256 || len(r.URL.RawQuery) > 4096 {
		writeAPIError(w, 400, "invalid_query", "invalid Queue name or oversized query")
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "invalid_query", "malformed query encoding")
		return
	}
	for key, values := range query {
		if len(values) != 1 || len(values[0]) > 1024 || (key != "subject" && key != "exchange" && key != "type" && key != "routingKey") {
			writeAPIError(w, 400, "invalid_query", "unsupported, repeated or oversized query parameter")
			return
		}
	}
	_, literal := query["subject"]
	_, exchange := query["exchange"]
	_, kind := query["type"]
	_, key := query["routingKey"]
	if (literal && (query.Get("subject") == "" || exchange || kind || key)) || (!literal && (!exchange || !kind)) {
		writeAPIError(w, 400, "invalid_query", "supply either a nonempty subject or an exchange and type")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	declaration, err := h.client.Declaration(ctx, name)
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	if declaration == nil || declaration.Queue != name || declaration.Plan.Queue != name || declaration.Revision != declaration.Plan.Revision {
		writeAPIError(w, 409, "routing_declaration_unavailable", "declaration identity or revision does not match its plan")
		return
	}
	document, err := topology.QueueDocument(declaration.Plan)
	if err != nil {
		writeAPIError(w, 409, "routing_declaration_unavailable", "declaration cannot be faithfully reconstructed")
		return
	}
	result, err := topology.ProbeRouting(*document, topology.RoutingProbe{
		Subject: query.Get("subject"), Exchange: query.Get("exchange"), Type: query.Get("type"), RoutingKey: query.Get("routingKey"),
	})
	if err != nil {
		writeAPIError(w, 400, "invalid_query", err.Error())
		return
	}
	// Bound the response before committing headers; never truncate match evidence.
	body, err := json.Marshal(result)
	if err != nil || len(body) > 1<<20 {
		writeAPIError(w, 503, "routing_probe_limit", "routing evidence exceeds the response limit")
		return
	}
	if ctx.Err() != nil {
		writeAPIError(w, 503, "routing_probe_unavailable", "routing read deadline exceeded or canceled")
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", declaration.KVRevision))
	writeJSON(w, http.StatusOK, json.RawMessage(body))
}
