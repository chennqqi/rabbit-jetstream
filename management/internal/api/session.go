package api

import (
	"context"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type sessionResponse struct {
	Actor              string              `json:"actor"`
	Role               string              `json:"role"`
	Permissions        []string            `json:"permissions"`
	ExpiresAt          *time.Time          `json:"expires_at"`
	Tenants            []string            `json:"tenants,omitempty"`
	TenantRoles        map[string]string   `json:"tenant_roles,omitempty"`
	TenantPermissions  map[string][]string `json:"tenant_permissions,omitempty"`
	ResourceReadPolicy string              `json:"resource_read_policy"`
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
	platformAdmin := principal.PlatformAdmin || (principal.Role == "operator" && len(principal.Tenants) == 0)
	result := sessionResponse{Actor: principal.Actor, Role: principal.Role, Permissions: permissionsForRole(principal.Role, platformAdmin), Tenants: append([]string(nil), principal.Tenants...), TenantRoles: principal.TenantRoles, ResourceReadPolicy: "anonymous"}
	if len(principal.TenantRoles) > 0 {
		result.TenantPermissions = make(map[string][]string, len(principal.TenantRoles))
		for id, role := range principal.TenantRoles {
			result.TenantPermissions[id] = permissionsForRole(role, platformAdmin)
		}
	}
	if h.auth.RequireReadAuth {
		result.ResourceReadPolicy = "authenticated"
	}
	if !principal.ExpiresAt.IsZero() {
		expiry := principal.ExpiresAt.UTC()
		result.ExpiresAt = &expiry
	}
	writeJSON(w, http.StatusOK, result)
}

func permissionsForRole(role string, platformAdmin bool) []string {
	result := []string{"resources:read", "audit:read", "history:read"}
	if role == "operator" {
		result = append(result, "queue:preview", "queue:apply", "queue:delete", "diagnostics:create", "diagnostics:download")
	}
	if platformAdmin {
		result = append(result, "access:manage")
	}
	return result
}
