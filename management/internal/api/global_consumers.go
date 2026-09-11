package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

func parseGlobalConsumerQuery(r *http.Request) (globalConsumerQuery, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return globalConsumerQuery{}, fmt.Errorf("malformed query encoding")
	}
	allowed := map[string]bool{"q": true, "queue": true, "stream": true, "mode": true, "state": true, "sort": true, "order": true, "offset": true, "limit": true, "generation": true}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return globalConsumerQuery{}, fmt.Errorf("unsupported or repeated query parameter %s", key)
		}
	}
	query := globalConsumerQuery{search: strings.ToLower(strings.TrimSpace(values.Get("q"))), queue: values.Get("queue"), stream: values.Get("stream"), mode: values.Get("mode"), state: values.Get("state"), generation: values.Get("generation"), limit: 50}
	for _, value := range []string{query.search, query.queue, query.stream, query.generation} {
		if len(value) > 256 {
			return globalConsumerQuery{}, fmt.Errorf("query value exceeds 256 bytes")
		}
	}
	if query.mode != "" && query.mode != "pull" && query.mode != "push" {
		return globalConsumerQuery{}, fmt.Errorf("mode must be pull or push")
	}
	if query.state != "" && query.state != "present" && query.state != "missing" && query.state != "mismatched" {
		return globalConsumerQuery{}, fmt.Errorf("invalid state")
	}
	if field := values.Get("sort"); field != "" && field != "identity" {
		return globalConsumerQuery{}, fmt.Errorf("sort must be identity")
	}
	order := values.Get("order")
	if order != "" && order != "asc" && order != "desc" {
		return globalConsumerQuery{}, fmt.Errorf("order must be asc or desc")
	}
	query.descending = order == "desc"
	parse := func(key string, fallback, maximum int) (int, error) {
		raw := values.Get(key)
		if raw == "" {
			return fallback, nil
		}
		if raw == "" || strings.Trim(raw, "0123456789") != "" {
			return 0, fmt.Errorf("%s must be an unsigned integer", key)
		}
		value, err := strconv.ParseUint(raw, 10, 53)
		if err != nil || value > uint64(maximum) {
			return 0, fmt.Errorf("%s exceeds limit", key)
		}
		return int(value), nil
	}
	if query.offset, err = parse("offset", 0, 1_000_000); err != nil {
		return globalConsumerQuery{}, err
	}
	if query.limit, err = parse("limit", 50, 200); err != nil || query.limit < 1 {
		if err == nil {
			err = fmt.Errorf("limit must be positive")
		}
		return globalConsumerQuery{}, err
	}
	return query, nil
}

func (h *Handler) globalConsumers(w http.ResponseWriter, r *http.Request) {
	query, err := parseGlobalConsumerQuery(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	index := h.consumerIndex(r.Context())
	status := index.status()
	if status.Generation == nil {
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}
	if query.generation != "" && query.generation != status.Generation.ID {
		writeAPIError(w, http.StatusConflict, "consumer_generation_changed", "Consumer generation changed; reset pagination")
		return
	}
	items := queryGlobalConsumers(status.Generation.Rows, query)
	total := len(items)
	start := query.offset
	if start > total {
		start = total
	}
	end := start + query.limit
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, struct {
		State        string              `json:"state"`
		GenerationID string              `json:"generation_id"`
		StartedAt    time.Time           `json:"started_at"`
		CompletedAt  time.Time           `json:"completed_at"`
		FailedAt     *time.Time          `json:"failed_at,omitempty"`
		Items        []globalConsumerRow `json:"items"`
		Total        int                 `json:"total"`
		Offset       int                 `json:"offset"`
		Limit        int                 `json:"limit"`
	}{status.State, status.Generation.ID, status.Generation.StartedAt, status.Generation.CompletedAt, status.FailedAt, items[start:end], total, query.offset, query.limit})
}

func (h *Handler) refreshGlobalConsumers(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "query parameters are not supported")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), globalConsumerCollectLimit+time.Second)
	defer cancel()
	index := h.consumerIndex(ctx)
	generation, busy, err := index.refresh(ctx, h.client, time.Now)
	if busy {
		writeJSON(w, http.StatusAccepted, index.status())
		return
	}
	if err != nil {
		h.logger.Warn("global Consumer collection failed")
		writeJSON(w, http.StatusServiceUnavailable, index.status())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		State        string    `json:"state"`
		GenerationID string    `json:"generation_id"`
		Total        int       `json:"total"`
		CompletedAt  time.Time `json:"completed_at"`
	}{"ready", generation.ID, len(generation.Rows), generation.CompletedAt})
}

func (h *Handler) consumerIndex(ctx context.Context) *globalConsumerIndex {
	id, _ := tenant.FromContext(ctx)
	value, _ := h.consumerIndexes.LoadOrStore(id, &globalConsumerIndex{})
	return value.(*globalConsumerIndex)
}
