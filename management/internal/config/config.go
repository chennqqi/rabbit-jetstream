package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config describes the management plane runtime configuration.
type Config struct {
	Name                      string
	DeploymentProfile         string
	ReleaseManifest           string
	HTTPAddr                  string
	TrustedProxyHops          int
	LocalDemo                 bool
	NATSURL                   string
	NATSUser                  string
	NATSPassword              string
	NATSCreds                 string
	NATSTLSCA                 string
	NATSTLSCert               string
	NATSTLSKey                string
	NATSTLSServerName         string
	NATSTLSInsecure           bool
	NATSMonitorURLs           string
	AdminToken                string
	AdminTokens               []string
	AuditTokens               []string
	LocalAccountsFile         string
	LocalAuthSigningKey       string
	LocalAuthTTL              time.Duration
	TenantsFile               string
	OIDCIssuer                string
	OIDCAudience              string
	OIDCRoleClaim             string
	OIDCOperatorRole          string
	OIDCAuditorRole           string
	OIDCAllowInsecure         bool
	OIDCBrowserClientID       string
	OIDCBrowserRedirectOrigin string
	PrometheusURL             string
	PrometheusPublicURL       string
	PrometheusToken           string
	PrometheusInsecure        bool
	OTLPTraceEndpoint         string
	OTLPMetricEndpoint        string
	OTELMetricInterval        time.Duration
	OTELSampleRatio           float64
	OTELAllowInsecure         bool
	MetadataBucket            string
	MetadataReplicas          int
	InstanceID                string
	ControllerEnabled         bool
	ControllerInterval        time.Duration
	ControllerLeaseTTL        time.Duration
	LogLevel                  string
	ConnectTimeout            time.Duration
	ShutdownTimeout           time.Duration
}

func FromEnv() Config {
	controllerInterval := positiveDuration("RJS_CONTROLLER_INTERVAL", 5*time.Second)
	controllerLeaseTTL := positiveDuration("RJS_CONTROLLER_LEASE_TTL", 15*time.Second)
	if controllerLeaseTTL < 2*controllerInterval {
		controllerLeaseTTL = 3 * controllerInterval
	}
	return Config{
		Name:                      env("RJS_NAME", "rabbit-jetstream"),
		DeploymentProfile:         env("RJS_DEPLOYMENT_PROFILE", "unknown"),
		ReleaseManifest:           os.Getenv("RJS_RELEASE_MANIFEST"),
		HTTPAddr:                  env("RJS_HTTP_ADDR", ":8223"),
		TrustedProxyHops:          nonNegativeInt("RJS_TRUSTED_PROXY_HOPS", 0),
		LocalDemo:                 boolean("RJS_LOCAL_DEMO", false),
		NATSURL:                   env("RJS_NATS_URL", "nats://127.0.0.1:4222"),
		NATSUser:                  os.Getenv("RJS_NATS_USER"),
		NATSPassword:              os.Getenv("RJS_NATS_PASSWORD"),
		NATSCreds:                 os.Getenv("RJS_NATS_CREDS"),
		NATSTLSCA:                 os.Getenv("RJS_NATS_TLS_CA"),
		NATSTLSCert:               os.Getenv("RJS_NATS_TLS_CERT"),
		NATSTLSKey:                os.Getenv("RJS_NATS_TLS_KEY"),
		NATSTLSServerName:         os.Getenv("RJS_NATS_TLS_SERVER_NAME"),
		NATSTLSInsecure:           boolean("RJS_NATS_TLS_INSECURE_SKIP_VERIFY", false),
		NATSMonitorURLs:           env("RJS_NATS_MONITOR_URLS", "http://127.0.0.1:8222"),
		AdminToken:                os.Getenv("RJS_ADMIN_TOKEN"),
		AdminTokens:               tokens(os.Getenv("RJS_ADMIN_TOKEN"), os.Getenv("RJS_ADMIN_TOKENS")),
		AuditTokens:               tokens("", os.Getenv("RJS_AUDIT_TOKENS")),
		LocalAccountsFile:         os.Getenv("RJS_LOCAL_ACCOUNTS_FILE"),
		LocalAuthSigningKey:       os.Getenv("RJS_LOCAL_AUTH_SIGNING_KEY"),
		LocalAuthTTL:              positiveDuration("RJS_LOCAL_AUTH_TTL", 15*time.Minute),
		TenantsFile:               os.Getenv("RJS_TENANTS_FILE"),
		OIDCIssuer:                os.Getenv("RJS_OIDC_ISSUER"),
		OIDCAudience:              os.Getenv("RJS_OIDC_AUDIENCE"),
		OIDCRoleClaim:             env("RJS_OIDC_ROLE_CLAIM", "roles"),
		OIDCOperatorRole:          env("RJS_OIDC_OPERATOR_ROLE", "rabbit-jetstream-operator"),
		OIDCAuditorRole:           env("RJS_OIDC_AUDITOR_ROLE", "rabbit-jetstream-auditor"),
		OIDCAllowInsecure:         boolean("RJS_OIDC_ALLOW_INSECURE_ISSUER", false),
		OIDCBrowserClientID:       os.Getenv("RJS_OIDC_BROWSER_CLIENT_ID"),
		OIDCBrowserRedirectOrigin: os.Getenv("RJS_OIDC_BROWSER_REDIRECT_ORIGIN"),
		PrometheusURL:             os.Getenv("RJS_PROMETHEUS_URL"),
		PrometheusPublicURL:       os.Getenv("RJS_PROMETHEUS_PUBLIC_URL"),
		PrometheusToken:           os.Getenv("RJS_PROMETHEUS_TOKEN"),
		PrometheusInsecure:        boolean("RJS_PROMETHEUS_ALLOW_INSECURE", false),
		OTLPTraceEndpoint:         os.Getenv("RJS_OTEL_TRACES_ENDPOINT"),
		OTLPMetricEndpoint:        os.Getenv("RJS_OTEL_METRICS_ENDPOINT"),
		OTELMetricInterval:        positiveDuration("RJS_OTEL_METRIC_INTERVAL", 30*time.Second),
		OTELSampleRatio:           ratio("RJS_OTEL_SAMPLE_RATIO", 0.1),
		OTELAllowInsecure:         boolean("RJS_OTEL_ALLOW_INSECURE", false),
		MetadataBucket:            env("RJS_METADATA_BUCKET", "RJS_META"),
		MetadataReplicas:          replicas("RJS_METADATA_REPLICAS", 1),
		InstanceID:                env("RJS_INSTANCE_ID", defaultInstanceID()),
		ControllerEnabled:         boolean("RJS_CONTROLLER_ENABLED", true),
		ControllerInterval:        controllerInterval,
		ControllerLeaseTTL:        controllerLeaseTTL,
		LogLevel:                  env("RJS_LOG_LEVEL", "info"),
		ConnectTimeout:            duration("RJS_CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout:           duration("RJS_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func (c Config) ValidateDeploymentProfile() error {
	if c.DeploymentProfile != "" && c.DeploymentProfile != "unknown" && c.DeploymentProfile != "standalone" && c.DeploymentProfile != "cluster" {
		return fmt.Errorf("RJS_DEPLOYMENT_PROFILE must be unknown, standalone, or cluster")
	}
	return nil
}

// ValidateHTTPAccess rejects anonymous demo exposure before opening services.
// Literal IPs avoid DNS resolution/rebinding ambiguity (localhost included).
func (c Config) ValidateHTTPAccess() error {
	if !c.LocalDemo {
		return nil
	}
	host, _, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("RJS_LOCAL_DEMO requires an explicit loopback IP and port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("RJS_LOCAL_DEMO requires a literal loopback bind address, got %q", c.HTTPAddr)
	}
	return nil
}

func (c Config) ValidateLocalAuth() error {
	if c.LocalAccountsFile == "" {
		if c.LocalAuthSigningKey != "" {
			return errors.New("RJS_LOCAL_AUTH_SIGNING_KEY requires RJS_LOCAL_ACCOUNTS_FILE")
		}
		return nil
	}
	if len(c.LocalAuthSigningKey) < 32 {
		return errors.New("RJS_LOCAL_AUTH_SIGNING_KEY must contain at least 32 bytes when local accounts are enabled")
	}
	if c.LocalAuthTTL <= 0 || c.LocalAuthTTL > 24*time.Hour {
		return errors.New("RJS_LOCAL_AUTH_TTL must be greater than zero and at most 24h")
	}
	return nil
}

func (c Config) ValidateTenancy() error {
	if c.TenantsFile != "" && c.PrometheusURL != "" {
		return errors.New("RJS_PROMETHEUS_URL is not tenant-scoped and cannot be combined with RJS_TENANTS_FILE")
	}
	return nil
}

func ratio(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil || value < 0 || value > 1 {
		return fallback
	}
	return value
}

func tokens(legacy, values string) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, value := range append([]string{legacy}, strings.Split(values, ",")...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func positiveDuration(key string, fallback time.Duration) time.Duration {
	value := duration(key, fallback)
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultInstanceID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

func boolean(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// nonNegativeInt follows the lenient config convention: an absent or invalid
// value falls back to the default instead of failing startup.
func nonNegativeInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func replicas(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "1" {
		return 1
	}
	if value == "3" {
		return 3
	}
	if value == "5" {
		return 5
	}
	return fallback
}

func LogLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
