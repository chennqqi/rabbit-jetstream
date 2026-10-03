package api

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type accessAccountRequest struct {
	Username      string                      `json:"username,omitempty"`
	Password      string                      `json:"password,omitempty"`
	Memberships   []identity.TenantMembership `json:"memberships"`
	Disabled      bool                        `json:"disabled"`
	PlatformAdmin bool                        `json:"platform_admin"`
}

func (h *Handler) accessTenants(w http.ResponseWriter, r *http.Request) {
	if !h.authorizePlatformAdmin(w, r) {
		return
	}
	items := append([]string(nil), h.auth.TenantIDs...)
	sort.Strings(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) accessAccounts(w http.ResponseWriter, r *http.Request) {
	if !h.authorizePlatformAdmin(w, r) {
		return
	}
	if h.auth.LocalAccounts == nil {
		writeAPIError(w, http.StatusNotFound, "local_account_management_disabled", "local account management is not configured")
		return
	}
	items, err := h.auth.LocalAccounts.ListAccounts(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "local_account_store_unavailable", "local account store is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) createAccessAccount(w http.ResponseWriter, r *http.Request) {
	if !h.authorizePlatformAdmin(w, r) {
		return
	}
	request, ok := h.readAccessAccount(w, r, "")
	if !ok {
		return
	}
	item, err := h.auth.LocalAccounts.CreateAccount(r.Context(), accountChange(request))
	if err != nil {
		h.writeAccountStoreError(w, http.StatusConflict, "account_create_rejected", "account could not be created", err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateAccessAccount(w http.ResponseWriter, r *http.Request) {
	if !h.authorizePlatformAdmin(w, r) {
		return
	}
	username := r.PathValue("username")
	request, ok := h.readAccessAccount(w, r, username)
	if !ok {
		return
	}
	item, err := h.auth.LocalAccounts.UpdateAccount(r.Context(), accountChange(request))
	if err != nil {
		h.writeAccountStoreError(w, http.StatusConflict, "account_update_rejected", "account could not be updated", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteAccessAccount(w http.ResponseWriter, r *http.Request) {
	if !h.authorizePlatformAdmin(w, r) {
		return
	}
	if h.auth.LocalAccounts == nil {
		writeAPIError(w, http.StatusNotFound, "local_account_management_disabled", "local account management is not configured")
		return
	}
	username := r.PathValue("username")
	if !validAccessName(username) || r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_account_request", "invalid account request")
		return
	}
	if err := h.auth.LocalAccounts.DeleteAccount(r.Context(), username); err != nil {
		h.writeAccountStoreError(w, http.StatusConflict, "account_delete_rejected", "account could not be deleted", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeAccountStoreError maps local account store outcomes to distinct
// responses: 404 unknown account, 409 existing username or protection policy,
// 400 store-side validation, 503 persistence failure. Anything unrecognized
// keeps the historical 409 conflict bucket so callers never misread an
// unknown failure as success-adjacent.
func (h *Handler) writeAccountStoreError(w http.ResponseWriter, conflictStatus int, conflictCode, conflictMessage string, err error) {
	switch {
	case errors.Is(err, identity.ErrAccountNotFound):
		writeAPIError(w, http.StatusNotFound, "account_not_found", "account does not exist")
	case errors.Is(err, identity.ErrAccountExists):
		writeAPIError(w, http.StatusConflict, "account_already_exists", "account already exists")
	case errors.Is(err, identity.ErrAccountPolicy):
		writeAPIError(w, http.StatusConflict, "account_policy_rejected", "account change violates an account protection policy")
	case errors.Is(err, identity.ErrAccountValidation):
		writeAPIError(w, http.StatusBadRequest, "invalid_account_request", "invalid account request")
	case errors.Is(err, identity.ErrAccountStoreUnavailable):
		h.logBackendError("local account store write failed", err)
		writeAPIError(w, http.StatusServiceUnavailable, "local_account_store_unavailable", "local account store is unavailable")
	default:
		h.logBackendError("local account store write failed", err)
		writeAPIError(w, conflictStatus, conflictCode, conflictMessage)
	}
}

func (h *Handler) readAccessAccount(w http.ResponseWriter, r *http.Request, pathUsername string) (accessAccountRequest, bool) {
	if h.auth.LocalAccounts == nil {
		writeAPIError(w, http.StatusNotFound, "local_account_management_disabled", "local account management is not configured")
		return accessAccountRequest{}, false
	}
	if r.URL.RawQuery != "" || strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])) != "application/json" {
		writeAPIError(w, http.StatusBadRequest, "invalid_account_request", "invalid account request")
		return accessAccountRequest{}, false
	}
	request, ok := decodeStrict[accessAccountRequest](w, r, 16<<10, "invalid_account_request", "invalid account request")
	if !ok {
		return accessAccountRequest{}, false
	}
	if pathUsername != "" {
		if request.Username != "" && request.Username != pathUsername {
			writeAPIError(w, http.StatusConflict, "account_name_mismatch", "URL and account usernames differ")
			return request, false
		}
		request.Username = pathUsername
	}
	if !validAccessName(request.Username) || len(request.Memberships) == 0 || len(request.Password) > 1024 {
		writeAPIError(w, http.StatusBadRequest, "invalid_account_request", "invalid account request")
		return request, false
	}
	known := map[string]bool{}
	for _, id := range h.auth.TenantIDs {
		known[id] = true
	}
	seen := map[string]bool{}
	for _, membership := range request.Memberships {
		id := membership.Tenant
		if !known[id] || seen[id] || (membership.Role != "operator" && membership.Role != "auditor") {
			writeAPIError(w, http.StatusBadRequest, "invalid_account_tenant", "account references an unavailable tenant")
			return request, false
		}
		seen[id] = true
	}
	return request, true
}

func accountChange(request accessAccountRequest) identity.LocalAccountChange {
	return identity.LocalAccountChange{Username: request.Username, Password: request.Password, Memberships: append([]identity.TenantMembership(nil), request.Memberships...), Disabled: request.Disabled, PlatformAdmin: request.PlatformAdmin}
}
func validAccessName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char == '-' || char == '_' || char == '.' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return false
		}
	}
	return true
}

func (h *Handler) authorizePlatformAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "access_api_disabled", "session API") {
		return false
	}
	principal, _ := r.Context().Value(principalKey{}).(identity.Principal)
	if principal.PlatformAdmin || strings.HasPrefix(principal.Actor, "token-sha256:") {
		return true
	}
	writeAPIError(w, http.StatusForbidden, "platform_admin_required", "platform administrator permission is required")
	return false
}
