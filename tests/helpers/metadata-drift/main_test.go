package main

import "testing"

func TestRejectNonFixtureEndpointsAndModes(t *testing.T) {
	for _, endpoint := range []string{"", "nats://localhost:4222", "nats://example.com:4222", "nats://127.0.0.1", "nats://user:secret@127.0.0.1:4222", "nats://127.0.0.1:4222/path", "nats://127.0.0.1:4222?x=y", "tls://127.0.0.1:4222"} {
		if err := run(endpoint, "inject"); err == nil {
			t.Fatalf("accepted nonfixture endpoint %q", endpoint)
		}
	}
	if err := run("nats://127.0.0.1:4222", "unknown"); err == nil {
		t.Fatal("accepted unknown mode")
	}
}
