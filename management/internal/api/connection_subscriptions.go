package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type connectionSubscriptionsMonitor interface {
	NodeConnectionSubscriptions(context.Context, string, uint64) (*monitoring.ConnectionSubscriptions, error)
}

func (h *Handler) nodeConnectionSubscriptions(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("cid")
	valid := raw != "" && r.URL.RawQuery == ""
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			valid = false
		}
	}
	cid, err := strconv.ParseUint(raw, 10, 64)
	if !valid || err != nil || cid == 0 {
		writeAPIError(w, 400, "invalid_query", "a nonzero uint64 CID and no query parameters are required")
		return
	}
	monitor, ok := h.monitor.(connectionSubscriptionsMonitor)
	if !ok {
		writeAPIError(w, 503, "connections_unavailable", "connection subscription monitoring is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := monitor.NodeConnectionSubscriptions(ctx, r.PathValue("node"), cid)
	if err != nil {
		switch {
		case errors.Is(err, monitoring.ErrConnectionMissing):
			writeAPIError(w, 404, "connection_not_found", "open connection not found on the observed node")
		case errors.Is(err, monitoring.ErrConnectionNodeMissing):
			writeAPIError(w, 404, "not_found", "node not found in configured monitoring endpoints")
		case errors.Is(err, monitoring.ErrConnectionNodeAmbiguous):
			writeAPIError(w, 409, "node_identity_ambiguous", "multiple configured endpoints report this node identity")
		case errors.Is(err, monitoring.ErrConnectionQuery):
			writeAPIError(w, 400, "invalid_query", "invalid node connection query")
		case errors.Is(err, monitoring.ErrConnectionSubscriptionLimit):
			writeAPIError(w, 422, "subscription_limit_exceeded", "connection has more than 1000 subscriptions; bounded expansion refused")
		case errors.Is(err, monitoring.ErrConnectionEndpointLimit):
			writeAPIError(w, 503, "connection_lookup_limit", "configured endpoint count exceeds connection lookup limit")
		default:
			writeAPIError(w, 503, "connections_unavailable", "connection subscription observation unavailable")
		}
		return
	}
	if result == nil {
		writeAPIError(w, 503, "connections_unavailable", "connection subscription observation unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
