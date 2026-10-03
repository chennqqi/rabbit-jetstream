package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type auditWindowBackend interface {
	AuditWindow(context.Context, jetstream.AuditFilter, *uint64) (jetstream.AuditWindowPage, error)
}

func (h *Handler) auditWindow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorizeAudit(w, r) {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "invalid_query", "malformed audit query")
		return
	}
	var filter jetstream.AuditFilter
	fields := map[string]*string{"requestId": &filter.RequestID, "resource": &filter.Resource, "actor": &filter.Actor, "phase": &filter.Phase, "action": &filter.Action, "outcome": &filter.Outcome}
	fields["from"], fields["until"] = &filter.From, &filter.Until
	var before *uint64
	for key, entries := range values {
		if len(entries) != 1 {
			writeAPIError(w, 400, "invalid_query", "repeated audit query parameter")
			return
		}
		value := entries[0]
		if key == "before" {
			if value == "" || strings.IndexFunc(value, func(ch rune) bool { return ch < '0' || ch > '9' }) != -1 {
				writeAPIError(w, 400, "invalid_query", "invalid before cursor")
				return
			}
			number, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				writeAPIError(w, 400, "invalid_query", "before exceeds uint64")
				return
			}
			before = &number
			continue
		}
		field, ok := fields[key]
		if !ok || len(value) > 256 || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) != -1 {
			writeAPIError(w, 400, "invalid_query", "unsupported or invalid audit filter")
			return
		}
		*field = value
	}
	if filter.Phase != "" && filter.Phase != "intent" && filter.Phase != "outcome" {
		writeAPIError(w, 400, "invalid_query", "phase must be intent or outcome")
		return
	}
	backend, ok := h.audit.(auditWindowBackend)
	if _, _, err := filter.TimeBounds(); err != nil {
		writeAPIError(w, 400, "invalid_query", err.Error())
		return
	}
	if !ok {
		writeAPIError(w, 503, "audit_unavailable", "audit window is unavailable")
		return
	}
	page, err := backend.AuditWindow(ctx, filter, before)
	if err != nil {
		h.logBackendError("audit window read failed", err)
		writeAPIError(w, 503, "audit_unavailable", "audit window could not be read")
		return
	}
	writeJSON(w, 200, page)
}
