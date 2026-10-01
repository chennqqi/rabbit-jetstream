package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type auditRequestBackend interface {
	AuditRequest(context.Context, string, *uint64) (jetstream.AuditRequestPage, error)
}

func (h *Handler) auditRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorizeAudit(w, r) {
		return
	}
	id := r.PathValue("requestID")
	if id == "" || len(id) > 128 || strings.IndexFunc(id, func(ch rune) bool {
		return !(ch == '-' || ch == '_' || ch == '.' || ch == ':' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z')
	}) != -1 {
		writeAPIError(w, 400, "invalid_request_id", "invalid audit request identifier")
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "invalid_query", "invalid audit query")
		return
	}
	var before *uint64
	for key, values := range query {
		if key != "before" || len(values) != 1 || values[0] == "" || strings.IndexFunc(values[0], func(ch rune) bool { return ch < '0' || ch > '9' }) != -1 {
			writeAPIError(w, 400, "invalid_query", "only one unsigned before cursor is supported")
			return
		}
		value, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil {
			writeAPIError(w, 400, "invalid_query", "before cursor exceeds uint64")
			return
		}
		before = &value
	}
	backend, ok := h.audit.(auditRequestBackend)
	if !ok {
		writeAPIError(w, 503, "audit_unavailable", "audit request evidence is unavailable")
		return
	}
	page, err := backend.AuditRequest(ctx, id, before)
	if err != nil {
		h.logBackendError("audit request evidence read failed", err, "request_id", id)
		writeAPIError(w, 503, "audit_unavailable", "audit request evidence could not be read")
		return
	}
	writeJSON(w, 200, page)
}
