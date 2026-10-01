package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNodeRequiresVarzIdentity(t *testing.T) {
	for _, payload := range []string{`{}`, `null`, `{"server_id":""}`, `{"server_id":"  "}`} {
		t.Run(payload, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, payload)
			}))
			defer server.Close()
			node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
			if calls.Load() != 1 || node.Status != "unavailable" || node.ID != "" || node.Sources["varz"].Available || len(node.Sources) != 1 {
				t.Fatalf("unanchored observations accepted: calls=%d node=%+v", calls.Load(), node)
			}
		})
	}
}

func TestNodeRejectsUnboundSourcesIndependently(t *testing.T) {
	for _, source := range []string{"routez", "jsz"} {
		for _, identity := range []string{"", "other", " ", "node "} {
			t.Run(source+"/"+identity, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					id := "node"
					if r.URL.Path == "/"+source {
						id = identity
					}
					switch r.URL.Path {
					case "/varz":
						fmt.Fprint(w, `{"server_id":"node","mem":123}`)
					case "/routez":
						fmt.Fprintf(w, `{"server_id":%q,"num_routes":1,"routes":[{"remote_name":"peer"}]}`, id)
					case "/jsz":
						fmt.Fprintf(w, `{"server_id":%q,"disabled":true,"memory":456,"meta_cluster":{"leader":"leader"}}`, id)
					}
				}))
				defer server.Close()
				node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
				if node.Status != "degraded" || node.ID != "node" || node.MemoryBytes == nil || *node.MemoryBytes != 123 || node.Sources[source].Available || node.Sources[source].ReadAt.IsZero() {
					t.Fatalf("invalid identity binding: %+v", node)
				}
				if source == "routez" {
					if len(node.ConnectedPeers) != 0 || !node.Sources["jsz"].Available || node.JetStream.MemoryBytes == nil || *node.JetStream.MemoryBytes != 456 {
						t.Fatalf("source isolation failed: %+v", node)
					}
				} else if node.JetStream.Enabled != nil || node.JetStream.MemoryBytes != nil || node.JetStream.MetaLeader != "" || !node.Sources["routez"].Available || len(node.ConnectedPeers) != 1 {
					t.Fatalf("source isolation failed: %+v", node)
				}
			})
		}
	}
}
