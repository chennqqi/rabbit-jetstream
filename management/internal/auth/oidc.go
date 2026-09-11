package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type OIDCConfig struct {
	Issuer, Audience, RoleClaim, OperatorRole, AuditorRole string
	BrowserClientID, BrowserRedirectOrigin                 string
	AllowInsecureIssuer                                    bool
}

type OIDCVerifier struct {
	verifier                             *oidc.IDTokenVerifier
	roleClaim, operatorRole, auditorRole string
	browser                              *BrowserConfig
	oauth                                oauth2.Endpoint
}

type BrowserConfig struct {
	AuthorizationEndpoint, ClientID, CallbackURL, LogoutEndpoint string
	Scopes                                                       []string
}

var pkcePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

func NewOIDC(ctx context.Context, cfg OIDCConfig) (*OIDCVerifier, error) {
	issuer, err := url.Parse(cfg.Issuer)
	if err != nil || issuer.Host == "" || (issuer.Scheme != "https" && !(cfg.AllowInsecureIssuer && issuer.Scheme == "http")) {
		return nil, errors.New("OIDC issuer must be an absolute HTTPS URL (HTTP requires RJS_OIDC_ALLOW_INSECURE_ISSUER=true)")
	}
	if cfg.Audience == "" || cfg.RoleClaim == "" || cfg.OperatorRole == "" || cfg.AuditorRole == "" {
		return nil, errors.New("OIDC audience, role claim, operator role, and auditor role are required")
	}
	provider, err := oidc.NewProvider(ctx, strings.TrimSuffix(cfg.Issuer, "/"))
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	endpoint := provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInParams
	result := &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Audience}), roleClaim: cfg.RoleClaim, operatorRole: cfg.OperatorRole, auditorRole: cfg.AuditorRole, oauth: endpoint}
	if (cfg.BrowserClientID == "") != (cfg.BrowserRedirectOrigin == "") {
		return nil, errors.New("OIDC browser client ID and redirect origin must be configured together")
	}
	if cfg.BrowserClientID != "" {
		if cfg.BrowserClientID != cfg.Audience {
			return nil, errors.New("OIDC browser client ID must equal the verified audience for ID-token bearer flow")
		}
		origin, parseErr := url.Parse(cfg.BrowserRedirectOrigin)
		if parseErr != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || (origin.Scheme != "https" && !(cfg.AllowInsecureIssuer && origin.Scheme == "http")) {
			return nil, errors.New("OIDC browser redirect origin must be an HTTPS origin")
		}
		if err := validateProviderEndpoint(result.oauth.AuthURL, cfg.AllowInsecureIssuer); err != nil {
			return nil, fmt.Errorf("OIDC authorization endpoint: %w", err)
		}
		if err := validateProviderEndpoint(result.oauth.TokenURL, cfg.AllowInsecureIssuer); err != nil {
			return nil, fmt.Errorf("OIDC token endpoint: %w", err)
		}
		var claims struct {
			EndSessionEndpoint string `json:"end_session_endpoint"`
		}
		_ = provider.Claims(&claims)
		if claims.EndSessionEndpoint != "" {
			if err := validateProviderEndpoint(claims.EndSessionEndpoint, cfg.AllowInsecureIssuer); err != nil {
				return nil, fmt.Errorf("OIDC end-session endpoint: %w", err)
			}
		}
		result.browser = &BrowserConfig{AuthorizationEndpoint: result.oauth.AuthURL, ClientID: cfg.BrowserClientID, CallbackURL: strings.TrimSuffix(origin.String(), "/") + "/admin/oidc/callback", LogoutEndpoint: claims.EndSessionEndpoint, Scopes: []string{oidc.ScopeOpenID, "profile"}}
	}
	return result, nil
}

func validateProviderEndpoint(raw string, allowHTTP bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http")) {
		return errors.New("must be an absolute HTTPS URL without credentials or fragment")
	}
	return nil
}
func (v *OIDCVerifier) BrowserLogin() *BrowserConfig {
	if v.browser == nil {
		return nil
	}
	copy := *v.browser
	copy.Scopes = append([]string(nil), v.browser.Scopes...)
	return &copy
}
func (v *OIDCVerifier) ExchangeBrowserCode(ctx context.Context, code, verifier string) (string, error) {
	if v.browser == nil {
		return "", errors.New("browser OIDC is disabled")
	}
	if len(code) < 1 || len(code) > 4096 || strings.IndexFunc(code, func(r rune) bool { return r < ' ' || r == 127 }) >= 0 || !pkcePattern.MatchString(verifier) {
		return "", errors.New("invalid OIDC callback values")
	}
	config := oauth2.Config{ClientID: v.browser.ClientID, Endpoint: v.oauth, RedirectURL: v.browser.CallbackURL, Scopes: v.browser.Scopes}
	token, err := config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return "", errors.New("OIDC code exchange failed")
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", errors.New("OIDC token response did not contain an ID token")
	}
	principal, err := v.Verify(ctx, raw)
	if err != nil || principal.Role == "" {
		return "", errors.New("OIDC ID token is not authorized")
	}
	return raw, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, raw string) (identity.Principal, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return identity.Principal{}, err
	}
	var claims map[string]json.RawMessage
	if err := token.Claims(&claims); err != nil {
		return identity.Principal{}, fmt.Errorf("decode OIDC claims: %w", err)
	}
	roles, err := claimStrings(claims[v.roleClaim])
	if err != nil {
		return identity.Principal{}, fmt.Errorf("decode OIDC role claim: %w", err)
	}
	principal := identity.Principal{Actor: "oidc:" + token.Issuer + "#" + token.Subject, ExpiresAt: token.Expiry}
	for _, role := range roles {
		if role == v.operatorRole {
			principal.Role = "operator"
			return principal, nil
		}
	}
	for _, role := range roles {
		if role == v.auditorRole {
			principal.Role = "auditor"
			break
		}
	}
	return principal, nil
}

func claimStrings(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{one}, nil
	}
	return nil, errors.New("claim must be a string or string array")
}
