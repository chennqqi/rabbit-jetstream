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

func TestConnectionSubscriptionsPreservesExactIdentityAndCounters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cid") != "7" || r.URL.Query().Get("auth") != "false" {
			t.Fatal("unsafe query", r.URL.RawQuery)
		}
		if r.URL.Query().Get("subs") == "false" {
			fmt.Fprint(w, `{"server_id":"node","now":"2026-01-01T00:00:00Z","num_connections":1,"total":1,"offset":0,"limit":1,"connections":[{"cid":7,"subscriptions":2}]}`)
			return
		}
		fmt.Fprint(w, `{"server_id":"node","now":"2026-01-01T00:00:01Z","num_connections":1,"connections":[{"cid":7,"subscriptions":2,"subscriptions_list_detail":[{"subject":"orders.*","qgroup":"workers","sid":"02","msgs":9223372036854775806,"max":9,"cid":7},{"subject":"orders.*","sid":"001","msgs":0,"cid":7}]}]}`)
	}))
	defer server.Close()
	result, err := New(server.URL, time.Second).ConnectionSubscriptions(context.Background(), 0, "node", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].SID != "001" || result.Items[1].SID != "02" || result.Items[0].Subject != result.Items[1].Subject || result.Items[0].Maximum != nil || result.Items[1].Messages != 9223372036854775806 {
		t.Fatalf("items=%#v", result.Items)
	}
}

func TestConnectionSubscriptionsRejectsLimitChurnAndDuplicateSID(t *testing.T) {
	for _, test := range []struct {
		name, preflight, detail string
		limit                   bool
	}{
		{"preflight limit", "1001", "", true},
		{"growth limit", "1", `{"cid":7,"subscriptions":1001}`, true},
		{"count churn", "1", `{"cid":7,"subscriptions":0}`, false},
		{"duplicate SID", "2", `{"cid":7,"subscriptions":2,"subscriptions_list_detail":[{"subject":"a","sid":"x","msgs":0,"cid":7},{"subject":"b","sid":"x","msgs":0,"cid":7}]}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("subs") == "false" {
					fmt.Fprintf(w, `{"server_id":"node","now":"2026-01-01T00:00:00Z","num_connections":1,"total":1,"offset":0,"limit":1,"connections":[{"cid":7,"subscriptions":%s}]}`, test.preflight)
					return
				}
				fmt.Fprintf(w, `{"server_id":"node","now":"2026-01-01T00:00:01Z","num_connections":1,"connections":[%s]}`, test.detail)
			}))
			defer server.Close()
			_, err := New(server.URL, time.Second).ConnectionSubscriptions(context.Background(), 0, "node", 7)
			if err == nil || test.limit != strings.Contains(err.Error(), "limit") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
