package api

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) protectResourceReads(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// The browser OIDC config is pre-login PKCE metadata (client id,
		// authorization endpoint, scopes); the login page must read it before
		// any credential exists, so it stays public like the schema contracts.
		public := path == "/api/v1/openapi.yaml" || path == "/api/v1/native-sdk-contract.json" || path == "/api/v1/oidc/config"
		ownAuthorization := path == "/api/v1/session" || path == "/api/v1/audit" || strings.HasPrefix(path, "/api/v1/audit/")
		readMethod := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodPost && strings.HasSuffix(path, "/connections/search")
		if h.auth.RequireReadAuth && readMethod && strings.HasPrefix(path, "/api/v1/") && !public && !ownAuthorization {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
			if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "read_api_disabled", "resource read API") {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
