package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
)

type BrowserOIDC interface {
	BrowserLogin() *managementauth.BrowserConfig
	ExchangeBrowserCode(context.Context, string, string) (string, error)
}

func (h *Handler) browserOIDCConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "OIDC config accepts no query parameters")
		return
	}
	config := h.auth.BrowserOIDC
	if config == nil || config.BrowserLogin() == nil {
		writeAPIError(w, http.StatusNotFound, "browser_oidc_disabled", "browser OIDC is not configured")
		return
	}
	value := config.BrowserLogin()
	writeJSON(w, http.StatusOK, struct {
		Schema                string   `json:"schema"`
		AuthorizationEndpoint string   `json:"authorizationEndpoint"`
		ClientID              string   `json:"clientId"`
		CallbackURL           string   `json:"callbackUrl"`
		LogoutEndpoint        string   `json:"logoutEndpoint,omitempty"`
		Scopes                []string `json:"scopes"`
	}{"rjs.browser-oidc.v1", value.AuthorizationEndpoint, value.ClientID, value.CallbackURL, value.LogoutEndpoint, value.Scopes})
}

func (h *Handler) browserOIDCToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "OIDC token exchange accepts no query parameters")
		return
	}
	if h.auth.BrowserOIDC == nil || h.auth.BrowserOIDC.BrowserLogin() == nil {
		writeAPIError(w, http.StatusNotFound, "browser_oidc_disabled", "browser OIDC is not configured")
		return
	}
	var request struct {
		Schema   string `json:"schema"`
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Schema != "rjs.browser-oidc-callback.v1" {
		writeAPIError(w, http.StatusBadRequest, "invalid_oidc_callback", "invalid OIDC callback request")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	token, err := h.auth.BrowserOIDC.ExchangeBrowserCode(ctx, request.Code, request.Verifier)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "oidc_exchange_failed", "OIDC callback could not be verified")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Schema string `json:"schema"`
		Token  string `json:"token"`
	}{"rjs.browser-oidc-token.v1", token})
}
