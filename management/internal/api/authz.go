package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

func (h *Handler) authorizeWrite(w http.ResponseWriter, r *http.Request) bool {
	return h.authorize(w, r, map[string]bool{"operator": true}, "write_api_disabled", "write API")
}

func (h *Handler) authorizeAudit(w http.ResponseWriter, r *http.Request) bool {
	return h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "audit_api_disabled", "audit API")
}

type principalKey struct{}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, allowed map[string]bool, disabledCode, capability string) bool {
	if len(h.auth.OperatorTokens) == 0 && len(h.auth.AuditorTokens) == 0 && h.auth.LocalVerifier == nil && h.auth.OIDC == nil {
		writeAPIError(w, http.StatusNotFound, disabledCode, capability+" is disabled")
		return false
	}
	const prefix = "Bearer "
	provided := r.Header.Get("Authorization")
	if !strings.HasPrefix(provided, prefix) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
		return false
	}
	raw := strings.TrimPrefix(provided, prefix)
	principal := identity.Principal{}
	if matchesToken(raw, h.auth.OperatorTokens) {
		principal = identity.Principal{Actor: tokenActor(raw), Role: "operator"}
	} else if matchesToken(raw, h.auth.AuditorTokens) {
		principal = identity.Principal{Actor: tokenActor(raw), Role: "auditor"}
	} else if h.auth.LocalVerifier != nil {
		var verifyErr error
		principal, verifyErr = h.auth.LocalVerifier.Verify(r.Context(), raw)
		if errors.Is(verifyErr, identity.ErrTokenExpired) {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="token expired"`)
			writeAPIError(w, http.StatusUnauthorized, "token_expired", "the access token has expired; sign in again")
			return false
		}
	}
	if principal.Actor == "" && h.auth.OIDC != nil {
		principal, _ = h.auth.OIDC.Verify(r.Context(), raw)
	}
	if principal.Actor == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
		return false
	}
	tenantID, ok := authorizeTenant(w, r, principal, capability == "session API", h.auth.DefaultTenant)
	if !ok {
		return false
	}
	if tenantID != "" {
		principal.Role = principal.RoleForTenant(tenantID)
	} else if len(principal.Tenants) > 0 {
		principal.Role = principal.RoleForTenant(principal.Tenants[0])
	}
	if !allowed[principal.Role] {
		writeAPIError(w, http.StatusForbidden, "forbidden", "authenticated identity lacks the required role")
		return false
	}
	ctx := context.WithValue(r.Context(), principalKey{}, principal)
	if tenantID != "" {
		ctx = tenant.WithContext(ctx, tenantID)
	}
	*r = *r.WithContext(ctx)
	return true
}

func authorizeTenant(w http.ResponseWriter, r *http.Request, principal identity.Principal, optional bool, defaultTenant string) (string, bool) {
	values := r.Header.Values("X-RJS-Tenant")
	if len(values) > 1 {
		writeAPIError(w, http.StatusBadRequest, "invalid_tenant", "exactly one tenant header is allowed")
		return "", false
	}
	available := principal.Tenants
	if len(available) == 0 {
		fallback := defaultTenant
		if fallback == "" {
			fallback = "local"
		}
		available = []string{fallback}
	}
	requested := ""
	if len(values) == 1 {
		requested = strings.TrimSpace(values[0])
		if requested == "" || len(requested) > 128 || strings.IndexFunc(requested, func(char rune) bool {
			return !(char == '-' || char == '_' || char == '.' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z')
		}) >= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_tenant", "tenant header is invalid")
			return "", false
		}
	}
	if requested == "" {
		if optional {
			return "", true
		}
		if len(available) != 1 {
			writeAPIError(w, http.StatusBadRequest, "tenant_required", "select one tenant for this request")
			return "", false
		}
		return available[0], true
	}
	for _, allowed := range available {
		if subtle.ConstantTimeCompare([]byte(requested), []byte(allowed)) == 1 {
			return requested, true
		}
	}
	writeAPIError(w, http.StatusForbidden, "tenant_forbidden", "authenticated identity is not a member of this tenant")
	return "", false
}

func matchesToken(provided string, tokens []string) bool {
	providedDigest := sha256.Sum256([]byte(provided))
	matched := 0
	for _, token := range tokens {
		tokenDigest := sha256.Sum256([]byte(token))
		matched |= subtle.ConstantTimeCompare(providedDigest[:], tokenDigest[:])
	}
	return matched == 1
}

func tokenList(token string) []string {
	if token == "" {
		return nil
	}
	return []string{token}
}

func cleanTokens(tokens []string) []string {
	cleaned := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != "" {
			cleaned = append(cleaned, token)
		}
	}
	return cleaned
}

func tokenActor(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "token-sha256:" + hex.EncodeToString(digest[:])
}

func remoteIP(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return value
}

// clientIP resolves the originating client address for rate limiting and
// audit attribution. Each trusted proxy appends the address it received the
// connection from, so with trustedProxyHops proxies the chain holds exactly
// hops entries: the client plus the interior proxies it passed through. The
// client is therefore the entry that is trustedProxyHops positions from the
// right. Anything absent, shorter than the trust chain, or not a literal IP
// falls back to the socket peer address, which is always a safe attribution.
// Hop-count mode assumes only the trusted proxies can reach the origin
// directly; clients must not be able to bypass the edge and spoof the header.
func (h *Handler) clientIP(r *http.Request) string {
	peer := remoteIP(r.RemoteAddr)
	if h.trustedProxyHops <= 0 {
		return peer
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if len(forwarded) < h.trustedProxyHops {
		return peer
	}
	value := strings.TrimSpace(forwarded[len(forwarded)-h.trustedProxyHops])
	if value == "" || net.ParseIP(value) == nil {
		return peer
	}
	return value
}
