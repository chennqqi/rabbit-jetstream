package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNodesAggregatesMonitoringEndpoints(t *testing.T) {
	server := monitoringServer(t, false)
	defer server.Close()

	snapshot := New(server.URL, time.Second).Nodes(context.Background())
	if snapshot.Status != "available" || snapshot.Available != 1 || len(snapshot.Nodes) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	node := snapshot.Nodes[0]
	if node.Name != "nats-1" || node.JetStream.MetaLeader != "nats-2" || node.JetStream.MetaClusterSize != 3 {
		t.Fatalf("unexpected node: %#v", node)
	}
	if len(node.ConnectedPeers) != 2 || node.ConnectedPeers[0] != "nats-2" || node.ConnectedPeers[1] != "nats-3" {
		t.Fatalf("unexpected peers: %#v", node.ConnectedPeers)
	}
}

func TestNodesPreservesPartialFailures(t *testing.T) {
	server := monitoringServer(t, true)
	defer server.Close()
	client := New(server.URL+",http://127.0.0.1:1", 100*time.Millisecond)

	snapshot := client.Nodes(context.Background())
	if snapshot.Status != "degraded" || snapshot.Degraded != 1 || snapshot.Unavailable != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Nodes[0].Status != "unavailable" && snapshot.Nodes[1].Status != "unavailable" {
		t.Fatalf("missing unavailable node: %#v", snapshot.Nodes)
	}
}

func TestPublicEndpointRemovesCredentials(t *testing.T) {
	if got := publicEndpoint("https://user:secret@nats.example:8222"); got != "https://nats.example:8222" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestSafeErrorRemovesCredentials(t *testing.T) {
	endpoint := "https://user:secret@nats.example:8222"
	got := safeError(fmt.Errorf("Get %s/varz: failed", endpoint), endpoint)
	if strings.Contains(got, "secret") || strings.Contains(got, "user@") {
		t.Fatalf("error leaked credentials: %q", got)
	}
}

func monitoringServer(t *testing.T, failRoutes bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/varz":
			_, _ = w.Write([]byte(`{"server_id":"id-1","server_name":"nats-1","version":"2.14.1","go":"go1.26","now":"2026-08-01T00:00:00Z","uptime":"1h","mem":1024,"cpu":1.5,"cores":4,"connections":2,"subscriptions":10,"cluster":{"name":"rjs"},"jetstream":{"config":{}}}`))
		case "/routez":
			if failRoutes {
				http.Error(w, "failed", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"routes":[{"remote_name":"nats-3"},{"remote_name":"nats-2"},{"remote_name":"nats-2"}]}`))
		case "/jsz":
			_, _ = w.Write([]byte(`{"memory":11,"storage":22,"streams":3,"consumers":4,"messages":5,"meta_cluster":{"leader":"nats-2","cluster_size":3,"pending":0}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}
