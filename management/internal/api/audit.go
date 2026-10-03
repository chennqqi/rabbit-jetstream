package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

func (h *Handler) recordAuditIntent(w http.ResponseWriter, r *http.Request, action, resource, revision string, force bool) (jetstream.AuditEvent, bool) {
	if h.audit == nil {
		writeMutationError(w, http.StatusServiceUnavailable, "audit_unavailable", "write rejected because audit backend is unavailable", "audit_intent", "")
		return jetstream.AuditEvent{}, false
	}
	requestID := auditRequestID(r)
	w.Header().Set("X-Request-ID", requestID)
	principal, _ := r.Context().Value(principalKey{}).(identity.Principal)
	tenantID, _ := tenant.FromContext(r.Context())
	event := jetstream.AuditEvent{ID: randomAuditID(), RequestID: requestID, Time: time.Now().UTC(), Phase: "intent", Action: action, ResourceKind: "Queue", ResourceName: resource, Actor: principal.Actor, ActorRole: principal.Role, Tenant: tenantID, SourceIP: h.clientIP(r), Outcome: "attempted", Revision: revision, Force: force}
	ctx, cancel := context.WithTimeout(tenant.WithContext(context.Background(), tenantID), 3*time.Second)
	defer cancel()
	if _, err := h.audit.RecordAudit(ctx, event); err != nil {
		if h.logger != nil {
			h.logger.Error("audit intent failed; mutation rejected", "request_id", requestID, "action", action, "queue", resource, "error_kind", "audit_unavailable")
		}
		writeMutationError(w, http.StatusServiceUnavailable, "audit_unavailable", "write rejected because audit intent could not be persisted", "audit_intent", event.ID)
		return jetstream.AuditEvent{}, false
	}
	h.events.publish(tenantID, "audit")
	return event, true
}

func (h *Handler) recordAuditOutcome(w http.ResponseWriter, intent jetstream.AuditEvent, outcome string, status int, code, revision string) bool {
	event := intent
	event.ID = randomAuditID()
	event.IntentID = intent.ID
	event.Time = time.Now().UTC()
	event.Phase = "outcome"
	event.Outcome = outcome
	event.HTTPStatus = status
	event.ErrorCode = code
	if revision != "" {
		event.Revision = revision
	}
	ctx, cancel := context.WithTimeout(tenant.WithContext(context.Background(), intent.Tenant), 3*time.Second)
	defer cancel()
	if _, err := h.audit.RecordAudit(ctx, event); err != nil {
		if h.logger != nil {
			h.logger.Error("audit outcome failed after mutation attempt", "request_id", intent.RequestID, "intent_id", intent.ID, "outcome", outcome, "error_kind", "audit_unavailable")
		}
		writeMutationError(w, http.StatusServiceUnavailable, "audit_unavailable", "mutation outcome could not be persisted; inspect resource state before retrying", "audit_outcome", intent.ID)
		return false
	}
	h.events.publish(intent.Tenant, "audit")
	return true
}

func auditRequestID(r *http.Request) string {
	value := r.Header.Get("X-Request-ID")
	if value != "" && len(value) <= 128 && strings.IndexFunc(value, func(ch rune) bool {
		return !(ch == '-' || ch == '_' || ch == '.' || ch == ':' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z')
	}) == -1 {
		return value
	}
	value = randomAuditID()
	r.Header.Set("X-Request-ID", value)
	return value
}

func randomAuditID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(value)
}

func preconditionRevision(value jetstream.ApplyPrecondition) string {
	if value.CreateOnly {
		return "create"
	}
	if value.ExpectedRevision != nil {
		return strconv.FormatUint(*value.ExpectedRevision, 10)
	}
	return ""
}
