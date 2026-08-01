package config

import (
	"testing"
	"time"
)

func TestFromEnvIncludesMonitoringEndpoints(t *testing.T) {
	t.Setenv("RJS_NATS_URL", "nats://nats-1:4222")
	t.Setenv("RJS_NATS_MONITOR_URLS", "http://nats-1:8222,http://nats-2:8222")
	t.Setenv("RJS_CONNECT_TIMEOUT", "3s")
	t.Setenv("RJS_ADMIN_TOKEN", "secret")
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
	if cfg.MetadataBucket != "TEST_META" || cfg.MetadataReplicas != 3 {
		t.Fatalf("metadata config = %#v", cfg)
	}
	if cfg.InstanceID != "management-2" || cfg.ControllerInterval != 2*time.Second || cfg.ControllerLeaseTTL != 6*time.Second {
		t.Fatalf("controller config = %#v", cfg)
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
