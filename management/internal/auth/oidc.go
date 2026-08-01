package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type OIDCConfig struct {
	Issuer, Audience, RoleClaim, OperatorRole, AuditorRole string
	AllowInsecureIssuer                                    bool
}

type OIDCVerifier struct {
	verifier                             *oidc.IDTokenVerifier
	roleClaim, operatorRole, auditorRole string
}

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
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Audience}), roleClaim: cfg.RoleClaim, operatorRole: cfg.OperatorRole, auditorRole: cfg.AuditorRole}, nil
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
	principal := identity.Principal{Actor: "oidc:" + token.Issuer + "#" + token.Subject}
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
