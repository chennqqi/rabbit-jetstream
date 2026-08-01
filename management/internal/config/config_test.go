package config

import (
	"testing"
	"time"
)

func TestFromEnvIncludesMonitoringEndpoints(t *testing.T) {
	t.Setenv("RJS_NATS_URL", "nats://nats-1:4222")
	t.Setenv("RJS_NATS_MONITOR_URLS", "http://nats-1:8222,http://nats-2:8222")
	t.Setenv("RJS_CONNECT_TIMEOUT", "3s")
	cfg := FromEnv()
	if cfg.NATSURL != "nats://nats-1:4222" || cfg.NATSMonitorURLs != "http://nats-1:8222,http://nats-2:8222" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.ConnectTimeout != 3*time.Second {
		t.Fatalf("connect timeout = %s", cfg.ConnectTimeout)
	}
}

func TestInvalidDurationUsesSafeDefault(t *testing.T) {
	t.Setenv("RJS_CONNECT_TIMEOUT", "invalid")
	if got := FromEnv().ConnectTimeout; got != 5*time.Second {
		t.Fatalf("connect timeout = %s", got)
	}
}
