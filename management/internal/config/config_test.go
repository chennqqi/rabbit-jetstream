package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestFromEnvIncludesMonitoringEndpoints(t *testing.T) {
	t.Setenv("RJS_NATS_URL", "nats://nats-1:4222")
	t.Setenv("RJS_NATS_MONITOR_URLS", "http://nats-1:8222,http://nats-2:8222")
	t.Setenv("RJS_CONNECT_TIMEOUT", "3s")
	t.Setenv("RJS_ADMIN_TOKEN", "secret")
	t.Setenv("RJS_ADMIN_TOKENS", "next-secret, secret")
	t.Setenv("RJS_AUDIT_TOKENS", " auditor-one, auditor-two ")
	t.Setenv("RJS_OIDC_ISSUER", "https://id.example.com")
	t.Setenv("RJS_OIDC_AUDIENCE", "management")
	t.Setenv("RJS_OIDC_ROLE_CLAIM", "groups")
	t.Setenv("RJS_OIDC_OPERATOR_ROLE", "platform-ops")
	t.Setenv("RJS_OIDC_AUDITOR_ROLE", "platform-audit")
	t.Setenv("RJS_OIDC_ALLOW_INSECURE_ISSUER", "true")
	t.Setenv("RJS_OTEL_TRACES_ENDPOINT", "https://collector.example/v1/traces")
	t.Setenv("RJS_OTEL_SAMPLE_RATIO", "0.25")
	t.Setenv("RJS_OTEL_ALLOW_INSECURE", "true")
	t.Setenv("RJS_METADATA_BUCKET", "TEST_META")
	t.Setenv("RJS_METADATA_REPLICAS", "3")
	t.Setenv("RJS_INSTANCE_ID", "management-2")
	t.Setenv("RJS_CONTROLLER_INTERVAL", "2s")
	t.Setenv("RJS_CONTROLLER_LEASE_TTL", "3s")
	cfg := FromEnv()
	if cfg.NATSURL != "nats://nats-1:4222" || cfg.NATSMonitorURLs != "http://nats-1:8222,http://nats-2:8222" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.ConnectTimeout != 3*time.Second {
		t.Fatalf("connect timeout = %s", cfg.ConnectTimeout)
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("admin token was not loaded")
	}
	if len(cfg.AdminTokens) != 2 || cfg.AdminTokens[0] != "secret" || cfg.AdminTokens[1] != "next-secret" || len(cfg.AuditTokens) != 2 {
		t.Fatalf("role tokens were not normalized: admins=%v auditors=%v", cfg.AdminTokens, cfg.AuditTokens)
	}
	if cfg.OIDCIssuer != "https://id.example.com" || cfg.OIDCAudience != "management" || cfg.OIDCRoleClaim != "groups" || cfg.OIDCOperatorRole != "platform-ops" || cfg.OIDCAuditorRole != "platform-audit" || !cfg.OIDCAllowInsecure {
		t.Fatalf("OIDC config = %#v", cfg)
	}
	if cfg.OTLPTraceEndpoint != "https://collector.example/v1/traces" || cfg.OTELSampleRatio != 0.25 || !cfg.OTELAllowInsecure {
		t.Fatalf("telemetry config = %#v", cfg)
	}
	if cfg.MetadataBucket != "TEST_META" || cfg.MetadataReplicas != 3 {
		t.Fatalf("metadata config = %#v", cfg)
	}
	if cfg.InstanceID != "management-2" || cfg.ControllerInterval != 2*time.Second || cfg.ControllerLeaseTTL != 6*time.Second {
		t.Fatalf("controller config = %#v", cfg)
	}
}

func TestTokensDropsEmptyAndDuplicateValues(t *testing.T) {
	got := tokens(" old ", "new,,old, new")
	if len(got) != 2 || got[0] != "old" || got[1] != "new" {
		t.Fatalf("tokens=%v", got)
	}
}

func TestLogLevels(t *testing.T) {
	for _, test := range []struct {
		value string
		want  slog.Level
	}{{"debug", slog.LevelDebug}, {"warn", slog.LevelWarn}, {"warning", slog.LevelWarn}, {"error", slog.LevelError}, {"unknown", slog.LevelInfo}} {
		if got := LogLevel(test.value); got != test.want {
			t.Fatalf("LogLevel(%q)=%v want=%v", test.value, got, test.want)
		}
	}
}

func TestInvalidBooleanAndReplicasUseDefaults(t *testing.T) {
	t.Setenv("RJS_CONTROLLER_ENABLED", "not-a-bool")
	t.Setenv("RJS_METADATA_REPLICAS", "2")
	t.Setenv("RJS_CONTROLLER_INTERVAL", "-1s")
	cfg := FromEnv()
	if !cfg.ControllerEnabled || cfg.MetadataReplicas != 1 || cfg.ControllerInterval != 5*time.Second {
		t.Fatalf("cfg=%#v", cfg)
	}
}

func TestControllerCanBeDisabled(t *testing.T) {
	t.Setenv("RJS_CONTROLLER_ENABLED", "false")
	if FromEnv().ControllerEnabled {
		t.Fatal("controller is enabled")
	}
}

func TestInvalidDurationUsesSafeDefault(t *testing.T) {
	t.Setenv("RJS_CONNECT_TIMEOUT", "invalid")
	if got := FromEnv().ConnectTimeout; got != 5*time.Second {
		t.Fatalf("connect timeout = %s", got)
	}
}

func TestInvalidSampleRatioUsesSafeDefault(t *testing.T) {
	for _, value := range []string{"invalid", "-1", "2"} {
		t.Setenv("RJS_OTEL_SAMPLE_RATIO", value)
		if got := FromEnv().OTELSampleRatio; got != 0.1 {
			t.Fatalf("value=%q ratio=%v", value, got)
		}
	}
}
