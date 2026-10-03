package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestMonitoringRejectsTruncatedBodyAfterValidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"server_id":"node","mem":99}`
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)+10))
		fmt.Fprint(w, payload)
	}))
	defer server.Close()
	node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
	if node.Status != "unavailable" || node.ID != "" || node.MemoryBytes != nil || node.Sources["varz"].Available {
		t.Fatalf("truncated response accepted: %+v", node)
	}
}

func TestMonitoringRejectsTrailingJSONBeforePublishing(t *testing.T) {
	for _, source := range []string{"varz", "routez", "jsz"} {
		for _, suffix := range []string{" {}", " null", " []", " trailing-garbage", " \n\t"} {
			t.Run(source+suffix, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/varz":
						fmt.Fprint(w, `{"server_id":"node","mem":99}`)
					case "/routez":
						fmt.Fprint(w, `{"server_id":"node","num_routes":1,"routes":[{"remote_name":"peer"}]}`)
					case "/jsz":
						fmt.Fprint(w, `{"server_id":"node","memory":123}`)
					}
					if r.URL.Path == "/"+source {
						fmt.Fprint(w, suffix)
					}
				}))
				defer server.Close()
				node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
				valid := suffix == " \n\t"
				if node.Sources[source].Available != valid || node.Sources[source].ReadAt.IsZero() {
					t.Fatalf("invalid source publication: %+v", node)
				}
				if valid {
					if node.Status != "available" {
						t.Fatalf("valid whitespace rejected: %+v", node)
					}
					return
				}
				switch source {
				case "varz":
					if node.ID != "" || node.MemoryBytes != nil || node.Status != "unavailable" || len(node.Sources) != 1 {
						t.Fatalf("partial varz leaked: %+v", node)
					}
				case "routez":
					if len(node.ConnectedPeers) != 0 || !node.Sources["jsz"].Available || node.Status != "degraded" {
						t.Fatalf("partial routes leaked: %+v", node)
					}
				case "jsz":
					if node.JetStream.MemoryBytes != nil || !node.Sources["routez"].Available || node.Status != "degraded" {
						t.Fatalf("partial jsz leaked: %+v", node)
					}
				}
			})
		}
	}
}
