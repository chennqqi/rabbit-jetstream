package monitoring

import (
	"context"
	"encoding/json"
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
	for _, source := range []string{"varz", "routez", "jsz"} {
		if !node.Sources[source].Available || node.Sources[source].ReadAt.IsZero() {
			t.Fatalf("missing source observation: %+v", node.Sources)
		}
	}
	if node.Name != "nats-1" || node.JetStream.MetaLeader != "nats-2" || node.JetStream.MetaClusterSize == nil || *node.JetStream.MetaClusterSize != 3 {
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

func TestNodeSourceFailuresCannotBecomeZeroMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/varz" {
			fmt.Fprint(w, `{"server_id":"N123","jetstream":{"config":{}}}`)
			return
		}
		http.Error(w, "unavailable", 503)
	}))
	defer server.Close()
	node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
	if !node.Sources["varz"].Available || node.Sources["routez"].Available || node.Sources["jsz"].Available || node.Status != "degraded" {
		t.Fatalf("incorrect availability: %+v", node)
	}
	if node.Sources["jsz"].ReadAt.IsZero() {
		t.Fatal("missing failed-read timestamp")
	}
	server.Close()
	node = New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
	if node.Sources["varz"].Available || len(node.Sources) != 1 || node.Status != "unavailable" {
		t.Fatalf("unattempted source claimed: %+v", node)
	}
}

func TestVarzMetricPresenceSurvivesSerialization(t *testing.T) {
	for _, payload := range []string{`{"server_id":"N123"}`, `{"server_id":"N123","mem":0,"cpu":0,"cores":0,"connections":0,"subscriptions":0,"slow_consumers":0,"in_msgs":0,"out_msgs":0}`} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/varz" {
					fmt.Fprint(w, payload)
				} else {
					fmt.Fprint(w, `{}`)
				}
			}))
			defer server.Close()
			node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
			body, err := json.Marshal(node)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			present := strings.Contains(payload, `"mem"`)
			for _, key := range []string{"memory_bytes", "cpu_percent", "cores", "connections", "subscriptions", "slow_consumers", "in_messages", "out_messages"} {
				value, ok := fields[key]
				if ok != present || (ok && string(value) != "0") {
					t.Fatalf("%s=%s present=%v, expected presence=%v", key, value, ok, present)
				}
			}
		})
	}
}

func TestJSzMetricPresenceSurvivesSerialization(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		status        int
		available     bool
		want          map[string]string
	}{
		{"missing", `{}`, 200, true, nil},
		{"null", `{"memory":null,"storage":null,"streams":null,"consumers":null,"messages":null,"meta_cluster":null}`, 200, true, nil},
		{"zero", `{"memory":0,"storage":0,"streams":0,"consumers":0,"messages":0,"meta_cluster":{"cluster_size":0,"pending":0}}`, 200, true, map[string]string{"memory_bytes": "0", "storage_bytes": "0", "streams": "0", "consumers": "0", "messages": "0", "meta_cluster_size": "0", "meta_pending": "0"}},
		{"partial exact", `{"memory":18446744073709551615,"meta_cluster":{"pending":0}}`, 200, true, map[string]string{"memory_bytes": "18446744073709551615", "meta_pending": "0"}},
		{"failed", `unavailable`, 503, false, nil},
		{"invalid after partial", `{"memory":12,"messages":"invalid"}`, 200, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/jsz" {
					w.WriteHeader(tc.status)
					payload := tc.payload
					if strings.HasPrefix(payload, "{") {
						payload = `{"server_id":"N123"` + strings.TrimPrefix(payload, "{")
						if len(tc.payload) > 2 {
							payload = `{"server_id":"N123",` + strings.TrimPrefix(tc.payload, "{")
						}
					}
					fmt.Fprint(w, payload)
				} else {
					fmt.Fprint(w, `{"server_id":"N123"}`)
				}
			}))
			defer server.Close()
			node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
			if node.Sources["jsz"].Available != tc.available {
				t.Fatalf("source availability: %+v", node.Sources)
			}
			body, err := json.Marshal(node.JetStream)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"memory_bytes", "storage_bytes", "streams", "consumers", "messages", "meta_cluster_size", "meta_pending"} {
				value, present := fields[key]
				want, wanted := tc.want[key]
				if present != wanted || wanted && string(value) != want {
					t.Fatalf("%s=%s present=%v, want %s present=%v", key, value, present, want, wanted)
				}
			}
		})
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
			_, _ = w.Write([]byte(`{"server_id":"id-1","num_routes":3,"routes":[{"remote_name":"nats-3"},{"remote_name":"nats-2"},{"remote_name":"nats-2"}]}`))
		case "/jsz":
			_, _ = w.Write([]byte(`{"server_id":"id-1","memory":11,"storage":22,"streams":3,"consumers":4,"messages":5,"meta_cluster":{"leader":"nats-2","cluster_size":3,"pending":0}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}
