package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJetStreamEnablementRequiresEvidence(t *testing.T) {
	for _, tc := range []struct{ name, varFields, jsFields, want, status string }{
		{"missing", "", "", "", "available"},
		{"null", `,"jetstream":null`, `,"disabled":null`, "", "available"},
		{"empty object", `,"jetstream":{}`, "", "", "available"},
		{"null config", `,"jetstream":{"config":null}`, "", "", "available"},
		{"active config", `,"jetstream":{"config":{}}`, "", "true", "available"},
		{"explicit disabled", "", `,"disabled":true`, "false", "available"},
		{"explicit enabled", "", `,"disabled":false`, "true", "available"},
		{"conflict", `,"jetstream":{"config":{}}`, `,"disabled":true`, "", "degraded"},
		{"invalid config", `,"jetstream":{"config":42}`, "", "", "unavailable"},
		{"invalid disabled retains config", `,"jetstream":{"config":{}}`, `,"disabled":"false"`, "true", "degraded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/varz":
					fmt.Fprint(w, `{"server_id":"node"`+tc.varFields+`}`)
				case "/routez":
					fmt.Fprint(w, `{"server_id":"node","num_routes":0,"routes":[]}`)
				case "/jsz":
					fmt.Fprint(w, `{"server_id":"node"`+tc.jsFields+`}`)
				}
			}))
			defer server.Close()
			node := New(server.URL, time.Second).Nodes(context.Background()).Nodes[0]
			body, err := json.Marshal(node.JetStream)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["enabled"]) != tc.want || node.Status != tc.status {
				t.Fatalf("enabled=%s status=%s want=%s/%s", fields["enabled"], node.Status, tc.want, tc.status)
			}
		})
	}
}
