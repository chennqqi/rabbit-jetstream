package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config describes the management plane runtime configuration.
type Config struct {
	Name               string
	HTTPAddr           string
	NATSURL            string
	NATSUser           string
	NATSPassword       string
	NATSCreds          string
	NATSMonitorURLs    string
	AdminToken         string
	AdminTokens        []string
	AuditTokens        []string
	OIDCIssuer         string
	OIDCAudience       string
	OIDCRoleClaim      string
	OIDCOperatorRole   string
	OIDCAuditorRole    string
	OIDCAllowInsecure  bool
	OTLPTraceEndpoint  string
	OTELSampleRatio    float64
	OTELAllowInsecure  bool
	MetadataBucket     string
	MetadataReplicas   int
	InstanceID         string
	ControllerEnabled  bool
	ControllerInterval time.Duration
	ControllerLeaseTTL time.Duration
	LogLevel           string
	ConnectTimeout     time.Duration
	ShutdownTimeout    time.Duration
}

func FromEnv() Config {
	controllerInterval := positiveDuration("RJS_CONTROLLER_INTERVAL", 5*time.Second)
	controllerLeaseTTL := positiveDuration("RJS_CONTROLLER_LEASE_TTL", 15*time.Second)
	if controllerLeaseTTL < 2*controllerInterval {
		controllerLeaseTTL = 3 * controllerInterval
	}
	return Config{
		Name:               env("RJS_NAME", "rabbit-jetstream"),
		HTTPAddr:           env("RJS_HTTP_ADDR", ":8223"),
		NATSURL:            env("RJS_NATS_URL", "nats://127.0.0.1:4222"),
		NATSUser:           os.Getenv("RJS_NATS_USER"),
		NATSPassword:       os.Getenv("RJS_NATS_PASSWORD"),
		NATSCreds:          os.Getenv("RJS_NATS_CREDS"),
		NATSMonitorURLs:    env("RJS_NATS_MONITOR_URLS", "http://127.0.0.1:8222"),
		AdminToken:         os.Getenv("RJS_ADMIN_TOKEN"),
		AdminTokens:        tokens(os.Getenv("RJS_ADMIN_TOKEN"), os.Getenv("RJS_ADMIN_TOKENS")),
		AuditTokens:        tokens("", os.Getenv("RJS_AUDIT_TOKENS")),
		OIDCIssuer:         os.Getenv("RJS_OIDC_ISSUER"),
		OIDCAudience:       os.Getenv("RJS_OIDC_AUDIENCE"),
		OIDCRoleClaim:      env("RJS_OIDC_ROLE_CLAIM", "roles"),
		OIDCOperatorRole:   env("RJS_OIDC_OPERATOR_ROLE", "rabbit-jetstream-operator"),
		OIDCAuditorRole:    env("RJS_OIDC_AUDITOR_ROLE", "rabbit-jetstream-auditor"),
		OIDCAllowInsecure:  boolean("RJS_OIDC_ALLOW_INSECURE_ISSUER", false),
		OTLPTraceEndpoint:  os.Getenv("RJS_OTEL_TRACES_ENDPOINT"),
		OTELSampleRatio:    ratio("RJS_OTEL_SAMPLE_RATIO", 0.1),
		OTELAllowInsecure:  boolean("RJS_OTEL_ALLOW_INSECURE", false),
		MetadataBucket:     env("RJS_METADATA_BUCKET", "RJS_META"),
		MetadataReplicas:   replicas("RJS_METADATA_REPLICAS", 1),
		InstanceID:         env("RJS_INSTANCE_ID", defaultInstanceID()),
		ControllerEnabled:  boolean("RJS_CONTROLLER_ENABLED", true),
		ControllerInterval: controllerInterval,
		ControllerLeaseTTL: controllerLeaseTTL,
		LogLevel:           env("RJS_LOG_LEVEL", "info"),
		ConnectTimeout:     duration("RJS_CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout:    duration("RJS_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
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
