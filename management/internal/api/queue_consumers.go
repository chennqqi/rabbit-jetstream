package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

func (h *Handler) queueConsumers(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "invalid_query", "malformed query encoding")
		return
	}
	for key, values := range query {
		if len(values) != 1 || (key != "q" && key != "mode" && key != "order" && key != "offset" && key != "limit") {
			err = fmt.Errorf("unsupported or repeated query parameter %s", key)
			break
		}
	}
	q, mode, order := strings.ToLower(strings.TrimSpace(query.Get("q"))), query.Get("mode"), query.Get("order")
	if len(q) > 256 || (mode != "" && mode != "pull" && mode != "push") || (order != "" && order != "asc" && order != "desc") {
		err = fmt.Errorf("invalid q, mode or order")
	}
	if err != nil {
		writeAPIError(w, 400, "invalid_query", err.Error())
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeAPIError(w, 400, "invalid_pagination", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	collection, err := h.client.QueueConsumers(ctx, r.PathValue("queue"))
	if err != nil {
		h.writeBackendError(w, err)
		return
	}
	items := make([]jetstream.QueueConsumer, 0, len(collection.Items))
	for _, item := range collection.Items {
		itemMode := ""
		var subjects []string
		if item.Observed != nil {
			itemMode, subjects = item.Observed.Mode, item.Observed.FilterSubjects
			if len(subjects) == 0 && item.Observed.FilterSubject != "" {
				subjects = []string{item.Observed.FilterSubject}
			}
		} else if item.Expected != nil {
			itemMode, subjects = item.Expected.Mode, item.Expected.FilterSubjects
		}
		match := strings.Contains(strings.ToLower(item.Name), q)
		for _, subject := range subjects {
			match = match || strings.Contains(strings.ToLower(subject), q)
		}
		if match && (mode == "" || mode == itemMode) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if order == "desc" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
	body := page(items, offset, limit)
	body["queue"], body["stream"] = collection.Queue, collection.Stream
	body["declaration_revision"] = collection.DeclarationRevision
	body["stream_status"], body["stream_ownership"] = collection.StreamStatus, collection.StreamOwnership
	writeJSON(w, http.StatusOK, body)
}
