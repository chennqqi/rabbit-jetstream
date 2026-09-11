package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectionDetailExactQueryAndProjection(t *testing.T) {
	body := strings.ReplaceAll(strings.ReplaceAll(connectionFixture, `"limit":50`, `"limit":1`), `"total":201`, `"total":1`)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/prefix/connz" || r.URL.RawQuery != "auth=false&cid=18446744073709551615&limit=1&offset=0&sort=cid&state=open&subs=false" {
			t.Errorf("unexpected exact request: %s", r.URL)
		}
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	detail, err := New(server.URL+"/prefix", time.Second).ConnectionDetail(context.Background(), 0, "node", ^uint64(0))
	if err != nil || detail == nil {
		t.Fatalf("read detail: %v", err)
	}
	if calls.Load() != 1 || detail.NodeID != "node" || detail.Item.CID != ^uint64(0) || *detail.Item.InMessages != 9007199254740993 || detail.ReadAt.IsZero() {
		t.Fatalf("incorrect exact result: %+v", detail)
	}
	encoded, _ := json.Marshal(detail)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), `"total"`) {
		t.Fatalf("detail leaked metadata or ambiguous total: %s", encoded)
	}
}

func TestConnectionDetailMissingRequiresValidExactObservation(t *testing.T) {
	const found = `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":1,"total":1,"offset":0,"limit":1,"connections":[{"cid":42}]}`
	const missing = `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":0,"total":0,"offset":0,"limit":1,"connections":[]}`
	for _, test := range []struct {
		name, body string
		want       error
	}{
		{"missing", missing, ErrConnectionMissing},
		{"found", found, nil},
		{"wrong-node-empty", strings.Replace(missing, `"node"`, `"restarted"`, 1), ErrConnectionUnavailable},
		{"wrong-cid", strings.Replace(found, `"cid":42`, `"cid":43`, 1), ErrConnectionUnavailable},
		{"node-total-preserved", strings.Replace(found, `"total":1`, `"total":200`, 1), nil},
		{"missing-cid-with-other-connections", strings.Replace(missing, `"total":0`, `"total":200`, 1), ErrConnectionMissing},
		{"found-with-zero-total", strings.Replace(found, `"total":1`, `"total":0`, 1), ErrConnectionUnavailable},
		{"null-items", strings.Replace(missing, `[]`, `null`, 1), ErrConnectionUnavailable},
		{"wrong-offset", strings.Replace(found, `"offset":0`, `"offset":1`, 1), ErrConnectionUnavailable},
		{"wrong-limit", strings.Replace(found, `"limit":1`, `"limit":50`, 1), ErrConnectionUnavailable},
		{"negative-counter", strings.Replace(found, `"cid":42`, `"cid":42,"in_msgs":-1`, 1), ErrConnectionUnavailable},
		{"invalid-time", strings.Replace(found, `2026-09-10`, `2026-02-30`, 1), ErrConnectionUnavailable},
		{"trailing-json", found + `{}`, ErrConnectionUnavailable},
		{"oversized", strings.Repeat(" ", maxConnectionResponseBytes) + missing, ErrConnectionUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, test.body) }))
			defer server.Close()
			detail, err := New(server.URL, time.Second).ConnectionDetail(context.Background(), 0, "node", 42)
			if !errors.Is(err, test.want) || (detail == nil) != (test.want != nil) {
				t.Fatalf("detail=%+v error=%v want=%v", detail, err, test.want)
			}
		})
	}
}

func TestConnectionDetailBoundsRedirectAndCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/must-not-follow", http.StatusFound)
	}))
	defer server.Close()
	client := New(server.URL, time.Second)
	for _, request := range []struct {
		index int
		node  string
		cid   uint64
	}{{-1, "node", 1}, {1, "node", 1}, {0, "", 1}, {0, "bad/id", 1}, {0, "node", 0}} {
		if _, err := client.ConnectionDetail(context.Background(), request.index, request.node, request.cid); !errors.Is(err, ErrConnectionQuery) {
			t.Fatalf("invalid request: %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input dispatched")
	}
	if _, err := client.ConnectionDetail(context.Background(), 0, "node", 1); !errors.Is(err, ErrConnectionUnavailable) || calls.Load() != 1 {
		t.Fatal("redirect followed or accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ConnectionDetail(ctx, 0, "node", 1); !errors.Is(err, ErrConnectionUnavailable) || calls.Load() != 1 {
		t.Fatal("canceled request dispatched or accepted")
	}
}

func TestConnectionCIDPageHasFilteredTotalAndNoSensitiveProjection(t *testing.T) {
	for _, tc := range []struct {
		name, connections string
		total             int
	}{{"found", `[{"cid":42,"name":"secret","subscriptions_list":["private"]}]`, 1}, {"missing", `[]`, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery != "auth=false&cid=42&limit=1&offset=0&sort=cid&state=open&subs=false" {
					t.Errorf("query=%s", r.URL.RawQuery)
				}
				fmt.Fprintf(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":%d,"total":200,"offset":0,"limit":1,"connections":%s}`, tc.total, tc.connections)
			}))
			defer server.Close()
			page, err := New(server.URL, time.Second).ConnectionCIDPage(context.Background(), 0, "node", 42, 25)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(page)
			if page.Total != tc.total || page.Limit != 25 || strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "private") {
				t.Fatalf("page=%+v json=%s", page, encoded)
			}
		})
	}
}
