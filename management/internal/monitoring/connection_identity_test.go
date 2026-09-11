package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectionIdentityPageFiltersBeforePagingAndRedacts(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if q.Get("auth") != "true" || q.Get("user") != "private user" || q.Get("limit") != "200" || q.Get("offset") != "0" || q.Get("subs") != "false" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":2,"total":999,"offset":0,"limit":200,"connections":[{"cid":7,"authorized_user":"private user","name":"secret"},{"cid":9,"account":"secret"}]}`)
	}))
	defer server.Close()
	page, err := New(server.URL, time.Second).ConnectionIdentityPage(context.Background(), 0, "node", "user", "private user", 1, 25)
	if err != nil || calls.Load() != 2 || page.Total != 2 || len(page.Items) != 1 || page.Items[0].CID != 9 {
		t.Fatalf("page=%+v calls=%d err=%v", page, calls.Load(), err)
	}
	encoded := fmt.Sprintf("%+v", page)
	if strings.Contains(encoded, "private") || strings.Contains(encoded, "secret") {
		t.Fatalf("identity leaked: %s", encoded)
	}
}

func TestConnectionIdentityPageRejectsInvalidAndChurn(t *testing.T) {
	client := New("http://example.invalid", time.Second)
	for _, pair := range [][2]string{{"unknown", "x"}, {"user", ""}, {"account", "x\n"}} {
		if _, err := client.ConnectionIdentityPage(context.Background(), 0, "node", pair[0], pair[1], 0, 50); err != ErrConnectionQuery {
			t.Fatalf("accepted %#v: %v", pair, err)
		}
	}
	var call int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		cid := 7
		if call == 2 {
			cid = 8
		}
		fmt.Fprintf(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":1,"total":1,"offset":0,"limit":200,"connections":[{"cid":%d}]}`, cid)
	}))
	defer server.Close()
	if _, err := New(server.URL, time.Second).ConnectionIdentityPage(context.Background(), 0, "node", "mqtt_client", "device", 0, 50); err != ErrConnectionUnavailable {
		t.Fatalf("accepted churn: %v", err)
	}
}

func TestConnectionNamePageBoundsFullNodeAndNeverProjectsNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":3,"total":3,"offset":0,"limit":200,"connections":[{"cid":1,"name":"same"},{"cid":2,"name":"other"},{"cid":3,"name":"same","in_msgs":7}]}`)
	}))
	defer server.Close()
	page, err := New(server.URL, time.Second).ConnectionIdentityPage(context.Background(), 0, "node", "name", "same", 1, 50)
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].CID != 3 || page.Items[0].InMessages == nil || *page.Items[0].InMessages != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	encoded, _ := json.Marshal(page)
	if strings.Contains(string(encoded), "same") || strings.Contains(string(encoded), "other") || strings.Contains(string(encoded), `"name"`) {
		t.Fatalf("name leaked: %s", encoded)
	}
}

func TestConnectionNamePageRejectsNodesAboveBoundBeforeSecondPage(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":0,"total":1001,"offset":0,"limit":200,"connections":[]}`)
	}))
	defer server.Close()
	if _, err := New(server.URL, time.Second).ConnectionIdentityPage(context.Background(), 0, "node", "name", "same", 0, 50); err != ErrConnectionSearchLimit || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}
