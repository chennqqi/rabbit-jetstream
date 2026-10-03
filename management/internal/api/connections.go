package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type connectionMonitor interface {
	NodeConnections(context.Context, string, int, int) (*monitoring.ConnectionPage, error)
}

type connectionSearchMonitor interface {
	NodeConnectionCIDPage(context.Context, string, uint64, int) (*monitoring.ConnectionPage, error)
}

type connectionIdentitySearchMonitor interface {
	NodeConnectionIdentityPage(context.Context, string, string, string, int, int) (*monitoring.ConnectionPage, error)
}

func (h *Handler) nodeConnectionIdentitySearch(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "identity search does not accept URL query parameters")
		return
	}
	if media := r.Header.Get("Content-Type"); media != "application/json" {
		writeAPIError(w, 415, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	var request struct {
		Kind   string `json:"kind"`
		Value  string `json:"value"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(w, 400, "invalid_query", "invalid identity search request")
		return
	}
	validKind := request.Kind == "name" || request.Kind == "user" || request.Kind == "account" || request.Kind == "mqtt_client"
	if !validKind || request.Value == "" || len(request.Value) > 256 || strings.IndexFunc(request.Value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 || request.Offset < 0 || request.Offset > 1_000_000 || request.Limit < 1 || request.Limit > 200 {
		writeAPIError(w, 400, "invalid_query", "invalid identity search request")
		return
	}
	search, ok := h.monitor.(connectionIdentitySearchMonitor)
	if !ok {
		writeAPIError(w, 503, "connections_unavailable", "connection search is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := search.NodeConnectionIdentityPage(ctx, r.PathValue("node"), request.Kind, request.Value, request.Offset, request.Limit)
	if errors.Is(err, monitoring.ErrConnectionSearchLimit) {
		writeAPIError(w, 422, "connection_search_limit", "connection search exceeds the 1000-connection safety boundary")
		return
	}
	if err != nil {
		h.writeConnectionError(w, err)
		return
	}
	if page == nil {
		writeAPIError(w, 503, "connections_unavailable", "connection observation unavailable")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) nodeConnections(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "invalid_query", "malformed query encoding")
		return
	}
	offset, limit := 0, 50
	var cid uint64
	cidSet := false
	for key, values := range query {
		if len(values) != 1 || (key != "offset" && key != "limit" && key != "cid") {
			writeAPIError(w, 400, "invalid_query", "only one offset, limit and cid are supported")
			return
		}
		for _, digit := range values[0] {
			if digit < '0' || digit > '9' {
				writeAPIError(w, 400, "invalid_query", "pagination requires unsigned decimal integers")
				return
			}
		}
		if key == "cid" {
			parsed, parseErr := strconv.ParseUint(values[0], 10, 64)
			if parseErr != nil || parsed == 0 {
				writeAPIError(w, 400, "invalid_query", "cid must be a nonzero uint64")
				return
			}
			cid, cidSet = parsed, true
			continue
		}
		value, err := strconv.Atoi(values[0])
		if err != nil || (key == "offset" && value > 1_000_000) || (key == "limit" && (value < 1 || value > 200)) {
			writeAPIError(w, 400, "invalid_query", "pagination out of range")
			return
		}
		if key == "offset" {
			offset = value
		} else {
			limit = value
		}
	}
	if cidSet && offset != 0 {
		writeAPIError(w, 400, "invalid_query", "cid search requires offset zero")
		return
	}
	monitor, ok := h.monitor.(connectionMonitor)
	if !ok {
		writeAPIError(w, 503, "connections_unavailable", "connection monitoring is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var page *monitoring.ConnectionPage
	if cidSet {
		search, searchOK := h.monitor.(connectionSearchMonitor)
		if !searchOK {
			writeAPIError(w, 503, "connections_unavailable", "connection search is unavailable")
			return
		}
		page, err = search.NodeConnectionCIDPage(ctx, r.PathValue("node"), cid, limit)
	} else {
		page, err = monitor.NodeConnections(ctx, r.PathValue("node"), offset, limit)
	}
	if err != nil {
		h.writeConnectionError(w, err)
		return
	}
	if page == nil {
		writeAPIError(w, 503, "connections_unavailable", "connection observation unavailable")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) writeConnectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, monitoring.ErrConnectionQuery):
		writeAPIError(w, 400, "invalid_query", "invalid node connection query")
	case errors.Is(err, monitoring.ErrConnectionNodeMissing):
		writeAPIError(w, 404, "not_found", "node not found in configured monitoring endpoints")
	case errors.Is(err, monitoring.ErrConnectionNodeAmbiguous):
		writeAPIError(w, 409, "node_identity_ambiguous", "multiple configured endpoints report this node identity")
	case errors.Is(err, monitoring.ErrConnectionEndpointLimit):
		writeAPIError(w, 503, "connection_lookup_limit", "configured endpoint count exceeds connection lookup limit")
	default:
		writeAPIError(w, 503, "connections_unavailable", "connection observation unavailable")
	}
}
