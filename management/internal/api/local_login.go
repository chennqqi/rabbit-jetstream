package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type localLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type localLoginResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Actor       string    `json:"actor"`
	Role        string    `json:"role"`
	Tenants     []string  `json:"tenants"`
}

func (h *Handler) localLogin(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "login accepts no query parameters")
		return
	}
	if h.auth.Local == nil {
		writeAPIError(w, http.StatusNotFound, "local_auth_disabled", "local account login is not configured")
		return
	}
	if contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); contentType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "invalid_content_type", "login requires application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request localLoginRequest
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Username == "" || len(request.Username) > 128 || request.Password == "" || len(request.Password) > 1024 {
		writeAPIError(w, http.StatusBadRequest, "invalid_login_request", "invalid login request")
		return
	}
	ip := remoteIP(r.RemoteAddr)
	if allowed, retry := h.loginLimiter.allowed(ip, request.Username); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retry.Round(time.Second)/time.Second))))
		writeAPIError(w, http.StatusTooManyRequests, "login_rate_limited", "too many login attempts; retry later")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, principal, err := h.auth.Local.Issue(ctx, request.Username, request.Password)
	if err != nil {
		h.loginLimiter.failed(ip, request.Username)
		// Never reveal whether the username exists, the password was wrong, or
		// the password verifier rejected an account record.
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is invalid")
		return
	}
	if token == "" || principal.Actor == "" || principal.ExpiresAt.IsZero() || (principal.Role != "operator" && principal.Role != "auditor") || len(principal.Tenants) == 0 {
		writeAPIError(w, http.StatusServiceUnavailable, "local_auth_unavailable", "local account login is unavailable")
		return
	}
	h.loginLimiter.succeeded(ip, request.Username)
	response := localLoginResponse{AccessToken: token, TokenType: "Bearer", ExpiresAt: principal.ExpiresAt.UTC(), Actor: principal.Actor, Role: principal.Role, Tenants: append([]string(nil), principal.Tenants...)}
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, response)
}
