package config

import (
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config describes the management plane runtime configuration.
type Config struct {
	Name             string
	HTTPAddr         string
	NATSURL          string
	NATSUser         string
	NATSPassword     string
	NATSCreds        string
	NATSMonitorURLs  string
	AdminToken       string
	MetadataBucket   string
	MetadataReplicas int
	LogLevel         string
	ConnectTimeout   time.Duration
	ShutdownTimeout  time.Duration
}

func FromEnv() Config {
	return Config{
		Name:             env("RJS_NAME", "rabbit-jetstream"),
		HTTPAddr:         env("RJS_HTTP_ADDR", ":8223"),
		NATSURL:          env("RJS_NATS_URL", "nats://127.0.0.1:4222"),
		NATSUser:         os.Getenv("RJS_NATS_USER"),
		NATSPassword:     os.Getenv("RJS_NATS_PASSWORD"),
		NATSCreds:        os.Getenv("RJS_NATS_CREDS"),
		NATSMonitorURLs:  env("RJS_NATS_MONITOR_URLS", "http://127.0.0.1:8222"),
		AdminToken:       os.Getenv("RJS_ADMIN_TOKEN"),
		MetadataBucket:   env("RJS_METADATA_BUCKET", "RJS_META"),
		MetadataReplicas: replicas("RJS_METADATA_REPLICAS", 1),
		LogLevel:         env("RJS_LOG_LEVEL", "info"),
		ConnectTimeout:   duration("RJS_CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout:  duration("RJS_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
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
