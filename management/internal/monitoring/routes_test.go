package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestRouteObservationsRequireCompleteCollection(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		available    bool
		peers        []string
	}{
		{"missing", ``, false, nil},
		{"null", `,"num_routes":0,"routes":null`, false, nil},
		{"missing count", `,"routes":[]`, false, nil},
		{"negative count", `,"num_routes":-1,"routes":[]`, false, nil},
		{"short list", `,"num_routes":1,"routes":[]`, false, nil},
		{"long list", `,"num_routes":0,"routes":[{"remote_name":"peer"}]`, false, nil},
		{"null row", `,"num_routes":1,"routes":[null]`, false, nil},
		{"unnamed row", `,"num_routes":1,"routes":[{"remote_name":"  "}]`, false, nil},
		{"partial named rows", `,"num_routes":2,"routes":[{"remote_name":"peer"},{}]`, false, nil},
		{"empty", `,"num_routes":0,"routes":[]`, true, []string{}},
		{"pooled routes and same local name", `,"num_routes":3,"routes":[{"remote_name":"peer"},{"remote_name":"node"},{"remote_name":"peer"}]`, true, []string{"node", "peer"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/varz":
					fmt.Fprint(w, `{"server_id":"id","server_name":"node"}`)
				case "/routez":
					fmt.Fprint(w, `{"server_id":"id"`+tc.fields+`}`)
				case "/jsz":
					fmt.Fprint(w, `{"server_id":"id","memory":0}`)
				}
			}))
			defer server.Close()
			node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
			if node.Sources["routez"].Available != tc.available || node.Sources["routez"].ReadAt.IsZero() || !node.Sources["jsz"].Available || node.JetStream.MemoryBytes == nil {
				t.Fatalf("incorrect source isolation: %+v", node)
			}
			if tc.available {
				if node.Status != "available" || !reflect.DeepEqual(node.ConnectedPeers, tc.peers) {
					t.Fatalf("peers=%v, node=%+v", node.ConnectedPeers, node)
				}
			} else if node.Status != "degraded" || len(node.ConnectedPeers) != 0 {
				t.Fatalf("incomplete peers exposed: %+v", node)
			}
		})
	}
}
