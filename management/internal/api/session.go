package api

import (
	"context"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type sessionResponse struct {
	Actor              string     `json:"actor"`
	Role               string     `json:"role"`
	Permissions        []string   `json:"permissions"`
	ExpiresAt          *time.Time `json:"expires_at"`
	ResourceReadPolicy string     `json:"resource_read_policy"`
}

// session reports authorization, not backend availability or a login cookie.
func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "session_api_disabled", "session API") {
		return
	}
	principal := r.Context().Value(principalKey{}).(identity.Principal)
	result := sessionResponse{Actor: principal.Actor, Role: principal.Role, Permissions: []string{"resources:read", "audit:read", "history:read"}, ResourceReadPolicy: "anonymous"}
	if h.auth.RequireReadAuth {
		result.ResourceReadPolicy = "authenticated"
	}
	if principal.Role == "operator" {
		result.Permissions = append(result.Permissions, "queue:preview", "queue:apply", "queue:delete", "diagnostics:create", "diagnostics:download")
	}
	if !principal.ExpiresAt.IsZero() {
		expiry := principal.ExpiresAt.UTC()
		result.ExpiresAt = &expiry
	}
	writeJSON(w, http.StatusOK, result)
}
